package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/collectors"
)

func main() {
	fmt.Println(`
	 █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
	██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
	███████║██████╔╝██║  ███╗██║   ██║███████╗
	██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
	██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
	╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝
	`)
	fmt.Println("Argus: AI-powered log analysis and root cause identification")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	llm, err := agents.CreateAgent()
	if err != nil {
		fmt.Printf("Failed to init Gemini: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Gemini connected")

	// ── 2. init docker collector ─────────────────────────────────────────
	collector, err := collectors.NewDockerCollector()
	if err != nil {
		fmt.Printf("Docker not available: %v\n", err)
		os.Exit(1)
	}

	if err := collector.Validate(ctx); err != nil {
		fmt.Printf("Docker daemon not running: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Docker connected")

	// ── 3. list containers and pick first one (TUI picker comes later) ───
	targets, err := collector.ListContainers(ctx)
	if err != nil {
		fmt.Printf("Failed to list containers: %v\n", err)
		os.Exit(1)
	}

	if len(targets) == 0 {
		fmt.Println("No running containers found. Start one first.")
		os.Exit(1)
	}

	for _, t := range targets {
		fmt.Printf("  %s (%s) - %s\n", t.Name, t.Image, t.Status)
	}

	// enter a number to pick a container, user input
	var choice int
	fmt.Print("Enter the number of the container to stream logs from: ")
	_, err = fmt.Scanf("%d", &choice)
	if err != nil {
		fmt.Printf("Invalid input: %v\n", err)
		os.Exit(1)
	}

	if choice < 1 || choice > len(targets) {
		fmt.Println("Invalid container number.")
		os.Exit(1)
	}

	// for now just pick the first container
	// TUI selector replaces this later
	selected := targets[choice-1]
	fmt.Printf("✓ Streaming logs from: %s (%s)\n", selected.Name, selected.Image)

	// ── 4. always fetch history first (works on running AND stopped containers) ──
	fmt.Printf("Fetching last 200 lines from %s...\n", selected.Name)

	logs, err := collector.FetchLogs(ctx, selected, collectors.FetchOptions{
		TailLines: "200",
		Since:     "", // empty = no time filter
	})
	if err != nil {
		fmt.Printf("Failed to fetch logs: %v\n", err)
		os.Exit(1)
	}

	// print what we got so you can see it working
	for _, entry := range logs {
		fmt.Printf("  [%s] %s\n", entry.Level, entry.Message)
	}

	fmt.Printf("✓ Collected %d log entries\n\n", len(logs))

	if len(logs) == 0 {
		fmt.Println("No logs found.")
		os.Exit(1)
	}

	// ── 5. if container is still running, stream new lines in background ──
	// new lines get appended to logs slice as they arrive
	// for now just print them — TUI will render them later
	if selected.Status == "running" {
		fmt.Println("Container is live — streaming new lines in background...")
		go func() {
			liveCh, errCh := collector.Stream(ctx, selected)
			for {
				select {
				case entry, ok := <-liveCh:
					if !ok {
						return
					}
					// for now just print — TUI will handle this later
					fmt.Printf("  [LIVE][%s] %s\n", entry.Level, entry.Message)
					// TODO: append to logs and re-analyze on demand

				case err := <-errCh:
					fmt.Printf("  Stream ended: %v\n", err)
					return

				case <-ctx.Done():
					return
				}
			}
		}()
	}

	fmt.Printf("✓ Collected %d log entries\n\n", len(logs))

	if len(logs) == 0 {
		fmt.Println("No logs collected. Is the container producing output?")
		os.Exit(1)
	}

	// ── 3. init orchestrator ─────────────────────────────────────────────
	orch := agents.NewOrchestrator(llm)

	// ── 4. drain events channel in background goroutine ──────────────────
	// without this, o.Events <- ... in orchestrator will block forever
	// because nobody is reading from the channel
	// think of it like starting a background thread that just prints events
	go func() {
		for event := range orch.Events {
			// print each agent event as it happens
			fmt.Printf("  [%s] %s: %s\n", event.Type, event.Tool, event.Message)
		}
	}()

	query := "Are there any errors, repeated failures, or anomalies in these logs? If so, summarize them and suggest a root cause."
	fmt.Printf("\n> Query: %s\n\n", query)
	result, err := orch.Run(ctx, query, logs)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n── Result ──────────────────────────────\n%s\n", result)

}

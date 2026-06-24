package main

import (
	"context"
	"fmt"
	"github.com/vishnuprasad2004/argus/agents"
	"os"
	"time"
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

	// ── 2. hardcoded test logs (replace with docker collector later) ─────
	// pretend these came from a real container
	logs := []agents.LogEntry{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Level:     "INFO",
			Source:    "nginx",
			Message:   "server started on port 80",
		},
		{
			Timestamp: time.Now().Add(-3 * time.Minute),
			Level:     "ERROR",
			Source:    "nginx",
			Message:   "connect() failed (111: Connection refused) while connecting to upstream",
		},
		{
			Timestamp: time.Now().Add(-3 * time.Minute),
			Level:     "ERROR",
			Source:    "nginx",
			Message:   "connect() failed (111: Connection refused) while connecting to upstream",
		},
		{
			Timestamp: time.Now().Add(-2 * time.Minute),
			Level:     "WARN",
			Source:    "nginx",
			Message:   "upstream response timeout after 30s",
		},
		{
			Timestamp: time.Now().Add(-1 * time.Minute),
			Level:     "ERROR",
			Source:    "nginx",
			Message:   "no live upstreams while connecting to upstream, client: 127.0.0.1",
		},
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

	query := "why is nginx failing to connect to upstream?"
	fmt.Printf("\n> Query: %s\n\n", query)
	result, err := orch.Run(ctx, query, logs)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n── Result ──────────────────────────────\n%s\n", result)

}

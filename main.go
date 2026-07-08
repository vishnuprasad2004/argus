package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/tui"
)

// // func main() {
// // 	fmt.Println(`
// // 	 █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
// // 	██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
// // 	███████║██████╔╝██║  ███╗██║   ██║███████╗
// // 	██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
// // 	██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
// // 	╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝
// // 	`)
// // 	fmt.Println("Argus: AI-powered log analysis and root cause identification")
// // 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
// // 	defer cancel()
// // 	llm, err := agents.CreateAgent()
// // 	if err != nil {
// // 		fmt.Printf("Failed to init Gemini: %v\n", err)
// // 		os.Exit(1)
// // 	}
// // 	fmt.Println("✓ Gemini connected")

// // 	// // ── 2. init docker collector ─────────────────────────────────────────
// // 	// collector, err := docker.NewDockerCollector()
// // 	// if err != nil {
// // 	// 	fmt.Printf("Docker not available: %v\n", err)
// // 	// 	os.Exit(1)
// // 	// }

// // 	// if err := collector.Validate(ctx); err != nil {
// // 	// 	fmt.Printf("Docker daemon not running: %v\n", err)
// // 	// 	os.Exit(1)
// // 	// }
// // 	// fmt.Println("✓ Docker connected")

// // 	// // ── 3. list containers and pick first one (TUI picker comes later) ───
// // 	// targets, err := collector.ListContainers(ctx)
// // 	// if err != nil {
// // 	// 	fmt.Printf("Failed to list containers: %v\n", err)
// // 	// 	os.Exit(1)
// // 	// }

// // 	// if len(targets) == 0 {
// // 	// 	fmt.Println("No running containers found. Start one first.")
// // 	// 	os.Exit(1)
// // 	// }

// // 	// for _, t := range targets {
// // 	// 	fmt.Printf("  %s (%s) - %s\n", t.Name, t.Image, t.Status)
// // 	// }

// // 	// // enter a number to pick a container, user input
// // 	// var choice int
// // 	// fmt.Print("Enter the number of the container to stream logs from: ")
// // 	// _, err = fmt.Scanf("%d", &choice)
// // 	// if err != nil {
// // 	// 	fmt.Printf("Invalid input: %v\n", err)
// // 	// 	os.Exit(1)
// // 	// }

// // 	// if choice < 1 || choice > len(targets) {
// // 	// 	fmt.Println("Invalid container number.")
// // 	// 	os.Exit(1)
// // 	// }

// // 	// // for now just pick the first container
// // 	// // TUI selector replaces this later
// // 	// selected := targets[choice-1]
// // 	// fmt.Printf("✓ Streaming logs from: %s (%s)\n", selected.Name, selected.Image)

// // 	// // ── 4. always fetch history first (works on running AND stopped containers) ──
// // 	// fmt.Printf("Fetching last 200 lines from %s...\n", selected.Name)

// // 	// logs, err := collector.FetchLogs(ctx, selected, docker.FetchOptions{
// // 	// 	TailLines: "200",
// // 	// 	Since:     "", // empty = no time filter
// // 	// })
// // 	// if err != nil {
// // 	// 	fmt.Printf("Failed to fetch logs: %v\n", err)
// // 	// 	os.Exit(1)
// // 	// }

// // 	// // print what we got so you can see it working
// // 	// for _, entry := range logs {
// // 	// 	fmt.Printf("  [%s] %s\n", entry.Level, entry.Message)
// // 	// }

// // 	// fmt.Printf("✓ Collected %d log entries\n\n", len(logs))

// // 	// if len(logs) == 0 {
// // 	// 	fmt.Println("No logs found.")
// // 	// 	os.Exit(1)
// // 	// }

// // 	// // ── 5. if container is still running, stream new lines in background ──
// // 	// // new lines get appended to logs slice as they arrive
// // 	// // for now just print them — TUI will render them later
// // 	// if selected.Status == "running" {
// // 	// 	fmt.Println("Container is live — streaming new lines in background...")
// // 	// 	go func() {
// // 	// 		liveCh, errCh := collector.Stream(ctx, selected)
// // 	// 		for {
// // 	// 			select {
// // 	// 			case entry, ok := <-liveCh:
// // 	// 				if !ok {
// // 	// 					return
// // 	// 				}
// // 	// 				// for now just print — TUI will handle this later
// // 	// 				fmt.Printf("  [LIVE][%s] %s\n", entry.Level, entry.Message)
// // 	// 				// TODO: append to logs and re-analyze on demand

// // 	// 			case err := <-errCh:
// // 	// 				fmt.Printf("  Stream ended: %v\n", err)
// // 	// 				return

// // 	// 			case <-ctx.Done():
// // 	// 				return
// // 	// 			}
// // 	// 		}
// // 	// 	}()
// // 	// }

// // 	// fmt.Printf("✓ Collected %d log entries\n\n", len(logs))

// // 	// if len(logs) == 0 {
// // 	// 	fmt.Println("No logs collected. Is the container producing output?")
// // 	// 	os.Exit(1)
// // 	// }

// // 	// ── 3. init orchestrator ─────────────────────────────────────────────
// // 	orch := agents.NewOrchestrator(llm)

// // 	// ── 4. drain events channel in background goroutine ──────────────────
// // 	// without this, o.Events <- ... in orchestrator will block forever
// // 	// because nobody is reading from the channel
// // 	// think of it like starting a background thread that just prints events
// // 	go func() {
// // 		for event := range orch.Events {
// // 			// print each agent event as it happens
// // 			fmt.Printf("  [%s] %s: %s\n", event.Type, event.Tool, event.Message)
// // 		}
// // 	}()

// // 	query := "Are there any errors, repeated failures, or anomalies in these logs? If so, summarize them and suggest a root cause."
// // 	fmt.Printf("\n> Query: %s\n\n", query)
// // 	result, err := orch.Run(ctx, query, logs)
// // 	if err != nil {
// // 		fmt.Printf("Error: %v\n", err)
// // 		os.Exit(1)
// // 	}

// // 	fmt.Printf("\n── Result ──────────────────────────────\n%s\n", result)

// // }

// func main() {
//     fmt.Println(`
//  █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
// ██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
// ███████║██████╔╝██║  ███╗██║   ██║███████╗
// ██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
// ██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
// ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝
//     `)
//     fmt.Println("Argus — AI-powered log analysis")
//     fmt.Println("─────────────────────────────────")

//     // init Gemini once — shared across all sources
//     llm, err := agents.CreateAgent()
//     if err != nil {
//         fmt.Printf("Failed to init Gemini: %v\n", err)
//         os.Exit(1)
//     }
//     fmt.Println("✓ Gemini connected")

//     // show source menu
//     fmt.Println("Select log source:")
//     fmt.Println("  1. Docker container")
//     fmt.Println("  2. Run a process (npm run dev, python main.py etc)")
//     fmt.Println("  3. Exit")
//     fmt.Print("\nChoice: ")

//     var choice string
//     fmt.Scanln(&choice)

//     switch strings.TrimSpace(choice) {
//     case "1":
//         runDockerMode(llm)
//     case "2":
//         runProcessMode(llm)
//     case "3":
//         fmt.Println("bye!")
//         os.Exit(0)
//     default:
//         fmt.Println("invalid choice")
//         os.Exit(1)
//     }
// }

// func runDockerMode(llm *googleai.GoogleAI) {
//     ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//     defer cancel()

//     collector, err := docker.NewDockerCollector()
//     if err != nil {
//         fmt.Printf("Docker not available: %v\n", err)
//         return
//     }
//     if err := collector.Validate(ctx); err != nil {
//         fmt.Printf("Docker daemon not running: %v\n", err)
//         return
//     }
//     fmt.Println("✓ Docker connected")

//     // list containers
//     targets, err := collector.ListContainers(ctx)
//     if err != nil {
//         fmt.Printf("Failed to list containers: %v\n", err)
//         return
//     }
//     if len(targets) == 0 {
//         fmt.Println("No containers found (running or stopped).")
//         return
//     }

//     fmt.Println("\nContainers:")
//     for i, t := range targets {
//         status := "●" // running
//         if t.Status != "running" {
//             status = "○" // stopped
//         }
//         fmt.Printf("  %d. %s %s (%s) — %s\n", i+1, status, t.Name, t.Image, t.Status)
//     }

//     fmt.Print("\nPick a container: ")
//     var choice int
//     fmt.Scanf("%d", &choice)
//     if choice < 1 || choice > len(targets) {
//         fmt.Println("invalid choice")
//         return
//     }
//     selected := targets[choice-1]

//     // fetch history
//     fmt.Printf("\nFetching last 200 lines from %s...\n", selected.Name)
//     logs, err := collector.FetchLogs(ctx, selected, docker.FetchOptions{
//         TailLines: "200",
//     })
//     if err != nil {
//         fmt.Printf("Failed to fetch logs: %v\n", err)
//         return
//     }
//     fmt.Printf("✓ Got %d log entries\n\n", len(logs))

//     // print fetched logs
//     for _, e := range logs {
//         fmt.Printf("  [%s] %s\n", e.Level, e.Message)
//     }

//     // start live stream in background if running
//     // mu protects logs slice — two goroutines touch it (stream + query)
//     var mu sync.Mutex
//     if selected.Status == "running" {
//         fmt.Println("\n[streaming live logs in background...]")
//         go func() {
//             liveCh, errCh := collector.Stream(ctx, selected)
//             for {
//                 select {
//                 case entry, ok := <-liveCh:
//                     if !ok {
//                         return
//                     }
//                     fmt.Printf("  [LIVE][%s] %s\n", entry.Level, entry.Message)
//                     mu.Lock()
//                     logs = append(logs, entry)
//                     mu.Unlock()
//                 case err := <-errCh:
//                     fmt.Printf("\n⚠ Stream ended: %v\n", err)
//                     return
//                 case <-ctx.Done():
//                     return
//                 }
//             }
//         }()
//     }

//     // hand off to interactive query loop
//     runQueryLoop(llm, &logs, &mu, ctx)
// }

// func runProcessMode(llm *googleai.GoogleAI) {
//     fmt.Print("Enter command to run (e.g. npm run dev): ")

//     // use bufio here not fmt.Scanln — Scanln stops at spaces
//     // bufio.NewReader reads the full line including spaces
//     reader := bufio.NewReader(os.Stdin)
//     command, _ := reader.ReadString('\n')
//     command = strings.TrimSpace(command)

//     if command == "" {
//         fmt.Println("no command entered")
//         return
//     }

//     ctx, cancel := context.WithCancel(context.Background())
//     defer cancel()

//     proc, err := process.NewProcessCollector()
//     if err != nil {
//         fmt.Printf("Error: %v\n", err)
//         return
//     }

//     logCh, resultCh, err := proc.Start(ctx, command)
//     if err != nil {
//         fmt.Printf("Failed to start process: %v\n", err)
//         return
//     }

//     // logs slice — shared between stream goroutine and query goroutine
//     var logs []agents.LogEntry
//     var mu sync.Mutex  // mutex = lock, prevents two goroutines writing at same time

//     // goroutine 1: collect logs as they stream in
//     go func() {
//         for entry := range logCh {  // range over channel — like forEach in JS
//             fmt.Printf("  [%s] %s\n", entry.Level, entry.Message)
//             mu.Lock()       // lock before writing — like synchronized in Java
//             logs = append(logs, entry)
//             mu.Unlock()     // unlock after writing
//         }
//     }()

//     // goroutine 2: watch for process exit
//     go func() {
//         result := <-resultCh  // blocks until process exits
//         fmt.Printf("\n⚠  %s\n", result.Message)
//         fmt.Printf("Session saved to: %s\n\n", proc.LogFilePath())

//         // show stats automatically when process exits
//         orch := agents.NewOrchestrator(llm)
//         mu.Lock()
//         snapshot := make([]agents.LogEntry, len(logs))
//         copy(snapshot, logs)
//         mu.Unlock()

//         statsOut, _ := orch.RunStats(ctx, snapshot)
//         fmt.Printf("\n── Stats ──────────────────\n%s\n\n", statsOut.Result)
//         fmt.Println("Process has stopped. You can still query the logs above.")
//     }()

//     // main goroutine: query loop — runs while process is also running
//     fmt.Println("\n[Process started. Type queries anytime, or /quit to exit]")
//     runQueryLoop(llm, &logs, &mu, ctx)

//     // cleanup — kill process if user exits query loop
//     proc.Kill()
// }

// // runQueryLoop is the interactive prompt
// // works while logs are still being collected concurrently
// func runQueryLoop(llm *googleai.GoogleAI, logs *[]agents.LogEntry, mu *sync.Mutex, ctx context.Context) {
//     orch := agents.NewOrchestrator(llm)

//     // drain events in background so orchestrator doesn't block
//     go func() {
//         for event := range orch.Events {
//             fmt.Printf("  ⚙ [%s] %s\n", event.Tool, event.Message)
//         }
//     }()

//     reader := bufio.NewReader(os.Stdin)
//     fmt.Println("─────────────────────────────────────────────")
//     fmt.Println("Commands: /stats  /quit  or type any question")
//     fmt.Println("─────────────────────────────────────────────")

//     for {
//         fmt.Print("\n> ")
//         input, _ := reader.ReadString('\n')
//         input = strings.TrimSpace(input)

//         if input == "" {
//             continue
//         }

//         switch input {
//         case "/quit":
//             fmt.Println("bye!")
//             return

//         case "/stats":
//             // stats agent — no LLM, instant
//             mu.Lock()
//             snapshot := make([]agents.LogEntry, len(*logs))
//             copy(snapshot, *logs)
//             mu.Unlock()

//             statsOut, err := orch.RunStats(ctx, snapshot)
//             if err != nil {
//                 fmt.Printf("stats error: %v\n", err)
//                 continue
//             }
//             fmt.Printf("\n── Stats ──────────────────\n%s\n", statsOut.Result)

//         default:
//             // natural language query → orchestrator → LLM
//             mu.Lock()
//             snapshot := make([]agents.LogEntry, len(*logs))
//             copy(snapshot, *logs)
//             mu.Unlock()

//             if len(snapshot) == 0 {
//                 fmt.Println("No logs yet — wait a moment and try again")
//                 continue
//             }

//             fmt.Println("thinking...")
//             result, err := orch.Run(ctx, input, snapshot)
//             if err != nil {
//                 fmt.Printf("error: %v\n", err)
//                 continue
//             }
//             fmt.Printf("\n── Answer ─────────────────\n%s\n", result)
//         }
//     }
// }

func main() {
    llm, err := agents.CreateAgent()
    if err != nil {
        fmt.Printf("Failed to init Gemini: %v\n", err)
        os.Exit(1)
    }

    p := tea.NewProgram(
        tui.NewRootModel(llm),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}
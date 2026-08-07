# Argus

```
 █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
███████║██████╔╝██║  ███╗██║   ██║███████╗
██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝
```

> AI-powered log analysis for SREs and developers — in your terminal.

<!-- TODO: add demo GIF here once recorded with vhs or asciinema -->

---

## What is Argus?

Argus is a TUI (Terminal User Interface) application built in Go that brings AI-powered log analysis directly into your terminal. Instead of tab-switching between Lens, Grafana, kubectl, and your log viewer, Argus gives you a single keyboard-driven interface to tail logs, detect anomalies, and ask natural language questions about what's happening in your services — all without leaving the terminal.

Named after the hundred-eyed giant of Greek mythology who never sleeps, Argus watches your services so you don't have to.

---

## The Problem

Every developer running containers or services locally hits the same wall when something breaks:

- Open Docker Desktop to find the container
- Open a separate terminal to tail logs
- Grep for errors manually
- Google the error message
- Context-switch back and forth trying to piece together what happened

On a laptop already running Docker, Minikube, and an editor, this workflow is slow, fragmented, and expensive on RAM. There is no single lightweight tool that does log tailing, anomaly detection, and AI-assisted root cause analysis in one place, from the terminal.

Argus solves this.

---

## Features

### Currently working
- **Docker log analysis** — connect to any running or stopped container, fetch historical logs and stream live output simultaneously
- **Process log capture** — run any command (`npm run dev`, `python main.py`, `go run .`) through Argus and capture all stdout/stderr with level detection
- **AI root cause analysis** — ask natural language questions about your logs, get direct answers with evidence
- **Conversational memory** — Argus remembers your conversation, so follow-up questions work naturally
- **Smart routing** — casual replies ("ok thanks") are handled conversationally, log questions trigger the analysis agents, stats queries use pure computation with zero LLM cost
- **Live + historical logs** — fetches the last 200 lines on connect, then streams new lines in the background
- **Preset commands** — `/stats`, `/clear`, `/quit` run instantly without an LLM call
- **Dual scrollable panels** — log viewer and answer panel are independently scrollable, switchable with `tab`
- **Markdown rendering** — LLM responses render with bold, italics, and code blocks in the terminal

### Coming soon
- Kubernetes pod log analysis (namespace → pod selector)
- Log file ingestion (.log file drag and drop)
- `~/.argus/config.yaml` first-run setup

---

## Why Go?

The entire cloud-native toolchain — `kubectl`, `helm`, `k9s`, `lazygit`, `Prometheus` — is written in Go. There are good reasons:

| Concern | Go advantage |
|---|---|
| Distribution | Single binary, no runtime, no JVM, no Python env |
| Lightweight & Less Memory Usage | ~15MB binary, <30MB RAM at runtime |
| Concurrency | Goroutines and channels make log streaming natural |
| TUI ecosystem | Bubble Tea + Lipgloss is the best TUI library available in any language |

When I made the same tool in Java or Python, it lacked being a lightweight tool, in Spring Boot would be a 200MB JAR requiring a JVM. In Node.js it would lack the TUI polish. Go was the right choice.

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Language | Go 1.22+ | Single binary, low RAM, CNCF-native |
| TUI framework | Bubble Tea + Lipgloss | Same as k9s, lazygit — Elm-arch, clean |
| CLI framework | Cobra + Viper | Same as kubectl, Helm — industry standard |
| AI | Google Gen AI Go SDK + Gemini API | Free tier, fast, good reasoning |
| LLM model | gemini-1.5-flash | Fast responses, free tier, good for log analysis |
| Markdown | Glamour | Same library used by GitHub's Glow CLI |
| Docker | docker/docker SDK | Official Go client |
| Config | Viper | YAML + env var + flag merging |

---

### Agent system

Argus uses an **agents-as-tools** pattern. The orchestrator is the conversational brain — it decides whether to answer directly or call a specialist agent:

```
User query
    │
    ▼
Orchestrator (Gemini LLM + conversation history)
    │
    ├── answer directly        (casual replies, follow-ups)
    ├── call log_analysis      (extract errors, patterns, anomalies)
    ├── call rca_agent         (root cause from log analysis output)
    └── call stats_agent       (counts, error rate — zero LLM cost)
```

Each agent is a `Tool` from the orchestrator's perspective. The orchestrator decides which tool to call based on the user's intent, then synthesizes the tool output into a natural conversational response.

### Log pipeline

```
Source (Docker / Process / k8s)
    │
    ▼
Collector (normalized LogEntry stream)
    │
    ▼
Pipeline (filter noise, scrub secrets) // TODO: pipeline optimization through context engineering is pending.
    │
    ▼
Orchestrator → Agents → Answer
```

All sources produce the same `LogEntry` struct — agents never know whether logs came from Docker, a process, or Kubernetes.

### TUI screen flow

```
Welcome
    │
    ▼
Source Select  (Docker / Process / k8s)
    │
    ├── Docker  → Container Select → Chat Screen
    └── Process → Process Setup   → Process Chat Screen
```

---

## Project Structure

```
argus/
├── main.go                          ← binary entrypoint (5 lines)
├── cmd/
│   └── root.go                      ← Cobra root command, config init, TUI launch
├── internal/
│   ├── config/
│   │   └── config.go                ← Viper config loader, ~/.argus/config.yaml
│   ├── collectors/
│   │   ├── docker/
│   │   │   └── docker.go            ← Docker SDK, FetchLogs + Stream
│   │   └── process/
│   │       └── process.go           ← exec runner, stdout/stderr capture
│   └── tui/
│       ├── app.go                   ← Root Bubble Tea model, screen routing
│       ├── screens/
│       │   ├── welcome.go
│       │   ├── source_select.go
│       │   ├── container_select.go
│       │   ├── chat.go              ← Docker log view + query bar
│       │   ├── process_setup.go     ← command input screen
│       │   ├── process_chat.go      ← process log view + query bar
│       │   └── messages.go          ← screen transition message types
│       ├── components/
│       │   ├── log_viewer.go        ← scrollable color-coded log panel
│       │   ├── query_bar.go         ← bottom input with /command detection
│       │   └── thinking.go          ← agent event display with fun verbs
│       └── styles/
│           └── theme.go             ← ALL colors, fonts, styles — edit here only
└── agents/
    ├── types.go                     ← LogEntry, AgentInput, AgentOutput, interfaces
    ├── gemini_agent.go              ← Gemini LLM client setup
    ├── orchestrator.go              ← SRE brain, conversation history, tool routing
    ├── log_analysis_agent.go        ← extracts errors and patterns from logs
    ├── rca_agent.go                 ← root cause analysis from log analysis output
    └── stats_agent.go               ← pure Go metrics, zero LLM cost
```

---

## Getting Started

### Prerequisites

- Go 1.22+
- Docker (for container log analysis)
- A Gemini API key — free at [aistudio.google.com](https://aistudio.google.com)

### Install

```bash
git clone https://github.com/vishnuprasad2004/argus
cd argus
go build -o bin/argus .
```

### Configure

On first run, Argus creates `~/.argus/config.yaml`:

```yaml
# ~/.argus/config.yaml
gemini_api_key: "your_key_here"
model: "gemini-1.5-flash"
log_tail_lines: "200"
```

Or set via environment variable:

```bash
export GEMINI_API_KEY=your_key_here
```

### Run

```bash
./bin/argus
```

Or during development:

```bash
make run
```

---

## Usage

### Docker container analysis

1. Select **Docker Container** from the source menu
2. Pick a running or stopped container
3. Argus fetches the last 200 lines and starts streaming live
4. Type any question: `why is nginx returning 502?`
5. Use `tab` to switch between log panel and answer panel
6. Use `/stats` for instant error counts, `/clear` to reset answers

### Process log capture

1. Navigate to your project directory: `cd ~/projects/my-api`
2. Run `argus`
3. Select **Process** from the source menu
4. Enter your start command: `npm run dev`
5. Argus runs the command and captures all output
6. Query anytime while the process runs — or after it crashes

### Preset commands

| Command | What it does | LLM cost |
|---|---|---|
| `/stats` | Error counts, warn counts, error rate | Free |
| `/clear` | Clear conversation history | Free |
| `/quit` | Exit Argus | Free |

---

## Keybindings

| Key | Action |
|---|---|
| `↑` / `↓` | Scroll focused panel |
| `tab` | Switch between log panel and answer panel |
| `esc` | Go back to previous screen |
| `enter` | Submit query / confirm selection |
| `ctrl+c` | Quit anywhere |

---

## Design Decisions

**Why not stream the LLM response token by token?**
Planned for v2. The current approach batches the full response then renders it — simpler to implement correctly with Bubble Tea's message loop.

**Why Gemini instead of Claude or GPT-4?**
Free tier. Argus is a portfolio and developer tool — asking users to pay for API calls on first run creates friction. Gemini 1.5 Flash is fast, free, and good enough for log analysis. Claude/OpenAI support is planned via config.

**Why manual ReAct loop instead of LangChainGo's agent executor?**
LangChainGo's executor was unstable at time of development. The manual routing approach (orchestrator asks LLM "which tool?", dispatches, feeds result back) is more predictable and easier to debug.

**Why separate log and answer viewports?**
Logs stream continuously — you want them auto-scrolling. Answers are reference material — you want to scroll back through them. Same panel for both meant one always fighting the other.

---

<!-- ## Contributing

<!-- TODO: add contribution guide -->

Pull requests welcome. For major changes open an issue first. -->

---

## Future Enhancements

- [ ] Kubernetes pod log analysis
- [ ] Log file ingestion
- [ ] Token-by-token streaming responses
- [ ] Multi-model support (Claude, GPT-4 via config)
- [ ] GitHub Actions release pipeline
- [ ] `argus watch` — daemon mode with anomaly alerts

---

<!-- 
NOTES FOR LATER EDITS:
- Add demo GIF after recording with vhs (https://github.com/charmbracelet/vhs)
- Add benchmark numbers (binary size, RAM usage) after profiling
- Add screenshots of each screen
- Update roadmap as features ship
- Add comparison table vs alternatives (k9s, stern, lnav) when ready
-->

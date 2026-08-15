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

<!-- TODO: replace with actual demo GIF recorded with vhs -->
<!-- ![Argus demo](demo.gif) -->

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Gemini](https://img.shields.io/badge/Powered%20by-Gemini-4285F4?style=flat&logo=google)](https://aistudio.google.com)

---

## What is Argus?

Argus is a terminal UI application built in Go that brings AI-powered log analysis directly into your terminal. Instead of tab-switching between Docker Desktop, Grafana, kubectl, and a log viewer, Argus gives you one keyboard-driven interface to tail logs, detect anomalies, and ask natural language questions about what is happening in your services — without leaving the terminal.

Named after the hundred-eyed giant of Greek mythology who never sleeps, Argus watches your services so you don't have to.

---

## The Problem

Every developer running containers or services locally hits the same wall when something breaks:

- Open Docker Desktop to find the container
- Open a separate terminal to tail logs
- Grep for errors manually
- Google the stack trace
- Context-switch between five tabs trying to piece together what happened

On a laptop already running Docker, Minikube, and an editor, this workflow is slow, fragmented, and expensive on RAM. There is no single lightweight tool that does log tailing, anomaly detection, and AI-assisted root cause analysis in one place, from the terminal.

Argus solves this.

---

## Features

### Log sources

| Source | What it does |
|---|---|
| **Docker** | Connect to any running or stopped container. Fetches last N lines of history and streams live output simultaneously |
| **Process** | Run any command (`npm run dev`, `python main.py`, `go run .`) through Argus. Captures stdout and stderr separately with auto level detection. Shows stats automatically on crash |
| **Log file** | Analyze any `.log`, `.txt`, or newline-delimited JSON file. Choose tail mode or full file |
| **Kubernetes** | Coming in v0.2.0 — namespace → pod selector, live streaming |

### AI capabilities

- **Natural language querying** — ask anything about your logs in plain English
- **Root cause analysis** — get direct answers with evidence from the logs
- **Conversational memory** — Argus remembers the full conversation, so follow-up questions work naturally
- **Smart routing** — casual replies are handled conversationally, log questions trigger analysis agents, stats queries run free with zero LLM cost
- **Token-by-token streaming** — responses stream as Gemini generates them, same feel as Claude Code or ChatGPT
- **Markdown rendering** — bold, italics, code blocks, and lists rendered natively in the terminal

### Terminal UX

- **Dual scrollable panels** — log viewer and answer panel independently scrollable, switch with `tab`
- **Live + historical logs** — history on connect, live stream running in background
- **Color-coded log levels** — ERROR in red, WARN in yellow, DEBUG in grey
- **Fun thinking indicators** — Noodling, Cogitating, Discombobulating while agents work
- **Preset commands** — `/stats`, `/clear`, `/quit` run instantly, no LLM call

### Security

- **Secret scrubbing** — API keys, passwords, tokens, JWT, AWS keys, emails, IPs, credit card numbers stripped before any LLM call
- **Local-only stats** — `/stats` never sends data to Gemini
- **Config with safe permissions** — `~/.argus/config.yaml` stored with `600` permissions

---

## Getting Started

### Prerequisites

- Go 1.22+ (not required for binary installation)
- Docker Desktop (for container log analysis)
- A Gemini API key — free at [aistudio.google.com](https://aistudio.google.com)

### Install from source

```bash
git clone https://github.com/vishnuprasad2004/argus
cd argus
go build -o bin/argus ./cmd/
./bin/argus
```

### Binary Installation (recommended)

Download the binary for your platform from [Releases](https://github.com/vishnuprasad2004/argus/releases):

<!-- # macOS (Apple Silicon)
chmod +x argus-darwin-arm64 && ./argus-darwin-arm64

# macOS (Intel)
chmod +x argus-darwin-amd64 && ./argus-darwin-amd64
 -->
```bash
# Linux
chmod +x argus-linux-amd64 && ./argus-linux-amd64
# Windows
argus-windows-amd64.exe
```

### Or install the binaries using the following commands: (curl is required)

```bash
# Linux
curl -L -o argus https://github.com/vishnuprasad2004/argus/releases/download/v0.1.0/argus-linux-amd64
# Windows
curl -L -o argus curl -L -o argus.exe https://github.com/vishnuprasad2004/argus/releases/download/v0.1.0/argus-windows-amd64.exe
```


### First run

On first launch, Argus runs a setup wizard that walks you through:

1. Entering your Gemini API key
2. Choosing a model (Flash Lite / Flash / Pro)

Your config is saved to `~/.argus/config.yaml` with owner-only permissions.

You can also skip the wizard by setting an environment variable:

```bash
export GEMINI_API_KEY=your_key_here
./bin/argus
```

Or edit the config directly:

```yaml
# ~/.argus/config.yaml
gemini_api_key: "AIza..."
model: "gemini-2.5-flash-lite"   # fastest, free tier — recommended
log_tail_lines: "200"
```

---

## Usage

### Docker container analysis

```
1. Launch argus
2. Select Docker Container
3. Pick any running or stopped container
4. Argus fetches history and starts live streaming
5. Ask anything: "why is nginx returning 502?"
6. Tab switches between log panel and answer panel
```

### Process log capture

```
1. cd into your project directory
2. Launch argus
3. Select Process
4. Enter your start command: npm run dev
5. Argus runs it and captures all output
6. Query anytime — or after it crashes (stats shown automatically)
```

### Log file analysis

```
1. Launch argus
2. Select Log File
3. Enter the file path: /var/log/nginx/error.log
4. Choose tail lines or full file
5. Query the logs
```

### Preset commands

| Command | What it does | LLM cost |
|---|---|---|
| `/stats` | Error counts, warn counts, error rate | Free — no LLM |
| `/clear` | Clear conversation history | Free |
| `/quit` | Exit Argus | Free |

---

## Keybindings

| Key | Action |
|---|---|
| `↑` / `↓` | Scroll focused panel |
| `tab` | Switch between log panel and answer panel |
| `enter` | Submit query or confirm selection |
| `esc` | Go back to previous screen |
| `ctrl+c` | Quit from anywhere |

---

## Why Go?

The entire cloud-native toolchain — `kubectl`, `helm`, `k9s`, `lazygit`, `Prometheus` — is written in Go. There are good reasons:

| Concern | Go advantage |
|---|---|
| Distribution | Single binary, no runtime, no JVM, no Python env needed |
| Memory | ~15MB binary, under 30MB RAM at runtime |
| Concurrency | Goroutines and channels make concurrent log streaming natural |
| TUI ecosystem | Bubble Tea + Lipgloss is the best TUI stack in any language |
| Ecosystem fit | CNCF-native, same toolchain as the infra it monitors |

The same tool in Spring Boot would ship as a 200MB JAR requiring a JVM. In Node.js it would lack the TUI polish. Go was the right choice.

---

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Language | Go 1.22+ | Single binary, low RAM, CNCF-native |
| TUI framework | Bubble Tea + Lipgloss | Same as k9s, lazygit — clean Elm architecture |
| CLI framework | Cobra + Viper | Same as kubectl, Helm — industry standard |
| AI SDK | Google GenAI Go SDK | Official SDK, supports all current Gemini models |
| LLM | Gemini 2.5 Flash Lite | Fast, free tier, good reasoning for log analysis |
| Markdown | Glamour | Same library as GitHub's Glow CLI |
| Docker | docker/docker SDK | Official Go client |
| Config | Viper | YAML + env var + flag merging with precedence |

---

## Architecture

### Agent system

Argus uses an **agents-as-tools** pattern. The orchestrator is the conversational brain — it decides whether to answer directly from context or invoke a specialist agent:

```
User query
    │
    ▼
Orchestrator (Gemini LLM + full conversation history)
    │
    ├── answer directly      → casual replies, follow-up questions
    ├── log_analysis agent   → extract errors, patterns, anomalies
    ├── rca agent            → root cause from log analysis output
    └── stats agent          → counts and error rate (zero LLM cost)
```

Sub-agents are tools from the orchestrator's perspective. The orchestrator routes based on intent, dispatches the tool, then synthesizes the result into a natural conversational response — streamed token by token.

### Log pipeline

```
Source (Docker / Process / File / k8s)
    │
    ▼
Collector → normalized LogEntry{}
    │
    ▼
Scrubber → strips secrets, PII, tokens
    │
    ▼
Summarizer → compresses large windows (~92% token reduction)
    │
    ▼
Orchestrator → Agents → streamed answer
```

All sources produce the same `LogEntry` struct from `internals/types`. Agents never know whether logs came from Docker, a process, a file, or Kubernetes.

### TUI screen flow

```
First run → Setup Wizard (API key + model choice)
    │
    ▼
Welcome
    │
    ▼
Source Select
    │
    ├── Docker  → Container Select → Chat
    ├── Process → Command Input   → Process Chat
    └── File    → File Setup      → File Chat
```

---

## Project Structure

```
argus/
├── main.go                              ← binary entrypoint (5 lines)
├── cmd/
│   └── root.go                          ← Cobra root command, TUI launch
├── internal/
│   ├── config/
│   │   └── config.go                    ← Viper loader, Save(), IsFirstRun()
│   ├── pipeline/
│   │   ├── scrubber.go                  ← strips secrets and PII before LLM
│   │   └── summarizer.go                ← compresses large log windows
│   ├── collectors/
│   │   ├── docker/collector.go          ← Docker SDK, FetchLogs + Stream
│   │   ├── process/process.go           ← exec runner, stdout/stderr capture
│   │   └── file/file.go                 ← .log .txt JSON file reader
│   └── tui/
│       ├── app.go                       ← root Bubble Tea model, screen routing
│       ├── screens/
│       │   ├── setup_wizard.go          ← first-run API key + model wizard
│       │   ├── welcome.go
│       │   ├── source_select.go
│       │   ├── container_select.go
│       │   ├── chat.go                  ← Docker: log panel + query bar
│       │   ├── process_setup.go         ← command input screen
│       │   ├── process_chat.go          ← Process: log panel + query bar
│       │   ├── file_setup.go            ← path input + lines choice
│       │   ├── file_chat.go             ← File: log panel + query bar
│       │   └── messages.go              ← screen transition message types
│       ├── components/
│       │   ├── log_viewer.go            ← scrollable color-coded log panel
│       │   ├── query_bar.go             ← input with /command detection
│       │   └── thinking.go              ← streaming agent events + fun verbs
│       └── styles/
│           └── theme.go                 ← all colors and styles — edit here only
└── agents/
    ├── types.go                         ← shared types, interfaces, event types
    ├── gemini_agent.go                  ← Google GenAI SDK client, stream support
    ├── orchestrator.go                  ← SRE brain, history, routing, tool dispatch
    ├── log_analysis_agent.go            ← extracts errors and patterns
    ├── rca_agent.go                     ← root cause from analysis output
    └── stats_agent.go                   ← pure Go metrics, zero LLM cost
```

---

## Design Decisions

**Why the official Google GenAI SDK instead of LangChainGo?**
LangChainGo's agent executor was unstable and used deprecated API endpoints. The official SDK is maintained by Google, always supports the latest models, and gives direct control over streaming and conversation history.

**Why a manual routing loop instead of a framework?**
The orchestrator asks Gemini "which tool is needed?" then dispatches and feeds results back for synthesis. This is more predictable, easier to debug, and cheaper than framework overhead — two LLM calls for a tool query vs one for a direct answer.

**Why separate log and answer viewports?**
Logs stream continuously and should auto-scroll. Answers are reference material you scroll back through. One panel for both meant they constantly fought each other.

**Why token-based streaming for the final answer?**
Same reason Claude Code and ChatGPT stream — it feels responsive immediately. The user sees tokens appearing rather than waiting 10 seconds for a full response.

**Why Gemini instead of Claude or GPT-4?**
Free tier with no credit card required. Argus is a developer tool — adding a billing requirement on first run kills adoption. Multi-model support (Claude, Ollama for local) is planned via config.

---

## Roadmap

- [x] Docker container log analysis
- [x] Process log capture
- [x] Log file ingestion (.log, .txt, JSON)
- [x] Conversational AI with memory
- [x] Token streaming responses
- [x] Secret scrubbing pipeline
- [x] First-run setup wizard
- [ ] Kubernetes pod log analysis (v0.2.0)
- [ ] Local LLM via Ollama (no data leaves machine)
- [ ] Multi-model support (Claude, GPT-4 via config)
- [ ] `brew install argus` via Homebrew tap
- [ ] GitHub Actions release pipeline
- [ ] `/export` — save conversation to file
- [ ] `argus watch` — daemon mode with anomaly alerts

---

## Contributing

Pull requests are welcome. For major changes, open an issue first to discuss what you'd like to change.

```bash
git clone https://github.com/vishnuprasad2004/argus
cd argus
go mod tidy
make run
```

---

## License

MIT — see [LICENSE](LICENSE)

---

<!--
NOTES FOR LATER EDITS:
- Record demo GIF with vhs (https://github.com/charmbracelet/vhs) and add at top
- Add binary size + RAM benchmark numbers after profiling
- Add screenshots of wizard, source select, and chat screens
- Add comparison table vs alternatives (stern, lnav, k9s) when ready
- Add CONTRIBUTING.md with PR guidelines
- Add brew tap instructions when GoReleaser is set up
-->
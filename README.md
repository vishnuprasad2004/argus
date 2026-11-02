# Argus

```
 █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
███████║██████╔╝██║  ███╗██║   ██║███████╗
██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝
```

> A lightweight, terminal-native observability workspace with an AI copilot.
> Logs and live metrics first. Ask questions in plain English when you need to.

<!-- TODO: replace with demo GIF recorded with vhs -->
<!-- ![Argus demo](demo.gif) -->

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Gemini](https://img.shields.io/badge/Powered%20by-Gemini-4285F4?style=flat&logo=google&logoColor=white)](https://aistudio.google.com)

---

## What is Argus?

Argus is a TUI (terminal user interface) application written in Go. It connects to a Docker container, a running process, or a log file, streams the logs into a split-pane workspace, shows live container metrics, and lets you ask an AI assistant questions about what is happening.

Logs and operational state are the main workspace. The AI is a contextual copilot on the side, not the centerpiece. It ships as a single binary with no runtime to install.

Named after the hundred-eyed giant of Greek mythology who never sleeps.

---

## The Problem

When something breaks locally, the workflow is usually:

- Open Docker Desktop to find the container
- Open another terminal to tail logs
- Grep for errors by hand
- Google the stack trace
- Switch between five tabs to piece together what happened

On a laptop already running Docker and an editor, that is slow and heavy. Argus puts log tailing, basic metrics, and AI-assisted analysis in one keyboard-driven terminal window.

---

## Features

### Log sources

| Source | What it does |
|---|---|
| **Docker** | Pick any running or stopped container. Loads recent history, then streams live output. |
| **Process** | Run any command (`npm run dev`, `python main.py`, `go run .`) through Argus. Captures stdout and stderr, detects log levels, and shows stats when the process exits. |
| **Log file** | Analyze a `.log`, `.txt`, or newline-delimited JSON file. Choose the last N lines or the whole file. |
| **Kubernetes** | Planned. |

### Workspace

- **Responsive split layout.** On wide terminals (120+ columns) logs take the left ~65% and the Insights pane takes the right ~35%. On narrow terminals it switches to a single pane with a tab bar.
- **Follow mode.** Live logs auto-scroll. Scroll up to read and Argus stops following. A `↓ N new logs` indicator shows what you missed. `ctrl+f` or `end` resumes following.
- **Independent scrolling.** The log pane and the Insights pane scroll separately. `tab` switches between them.
- **Color-coded log levels** with live and historical logs in one view.
- **Persistent query bar.** It is always ready for typing, whichever pane has scroll focus.

### Live metrics (Docker)

For running containers, the Insights pane shows CPU %, memory (used / limit), network in and out, block I/O, and PIDs. These come straight from the Docker stats API and refresh every 2 seconds. No Prometheus or exporter is needed.

### AI assistant

- **Natural-language questions** about the logs you are looking at
- **Root cause analysis** with evidence from the logs
- **Conversation memory** within a session, so follow-ups work
- **Tool routing.** The orchestrator decides whether to answer from context or call a specialist agent (`log_analysis`, `rca`, `stats`)
- **Token streaming.** Answers appear as they are generated
- **Markdown rendering** in the terminal: code blocks, lists, emphasis
- **Free local stats.** `/stats` runs in pure Go with no LLM call
- **Experimental: agent memory.** Argus reads `~/.argus/agent.md` (your stack, services, preferences) and matching files in `~/.argus/runbooks/` and includes them as context

### Security

- **Secret scrubbing before the LLM.** API keys, passwords, tokens, JWTs, AWS keys, emails, IP addresses, and card-like numbers are redacted from logs before anything is sent to Gemini.
- **Local-only commands.** `/stats` never leaves your machine.
- **Config permissions.** `~/.argus/config.yaml` is written with owner-only (`600`) permissions.

> Scrubbing is regex-based and best-effort, not a guarantee. The log pane in your terminal shows raw, unscrubbed logs; only the data sent to the model is scrubbed. Review your own data-handling requirements before pointing Argus at sensitive production logs.

---

## Layout

```
Wide terminal (120+ columns)
┌──────────────────────────────────────────────────────────────────────┐
│ argus  ● payments-api  node:18                                       │
├──────────────────────────────────────┬───────────────────────────────┤
│ LOGS                                 │ INSIGHTS                      │
│ 14:32:01 ERROR payments dial tcp ... │ CPU ████████░░  23%           │
│ 14:32:02 WARN  payments retry 3/5    │ MEM ██████████  487/512MB     │
│ 14:32:03 INFO  payments GET /health  │ NET ↑ 1.2MB  ↓ 0.3MB          │
│ ↓ following                          │ ───────────────────────────── │
│                                      │ ◆ Argus  (answers stream here)│
├──────────────────────────────────────┴───────────────────────────────┤
│ ❯ why did the error rate spike?                                      │
├──────────────────────────────────────────────────────────────────────┤
│ ↑↓ scroll · ctrl+f follow · tab switch panel · esc back · /stats     │
└──────────────────────────────────────────────────────────────────────┘

Narrow terminal: tab bar (Logs · Insights) with one pane visible at a time.
```

---

## Getting Started

### Prerequisites

- A Gemini API key (free at [aistudio.google.com](https://aistudio.google.com))
- Docker Desktop, only if you want Docker container analysis
- Go (version in `go.mod`), only if building from source
- On Windows, use [Windows Terminal](https://aka.ms/terminal) for the best rendering

### Install a prebuilt binary

Download from [Releases](https://github.com/vishnuprasad2004/argus/releases), or use the commands below.

**Linux**

```bash
curl -L -o argus https://github.com/vishnuprasad2004/argus/releases/latest/download/argus-linux-amd64
chmod +x argus
./argus
```

**Windows (PowerShell)**

```powershell
curl.exe -L -o argus.exe https://github.com/vishnuprasad2004/argus/releases/latest/download/argus-windows-amd64.exe
.\argus.exe
```

If Windows blocks the file, run `Unblock-File .\argus.exe` first.

**macOS:** prebuilt binaries are not published yet. Build from source below.

### Build from source

```bash
git clone https://github.com/vishnuprasad2004/argus
cd argus
go mod tidy
go build -o bin/argus ./cmd/
./bin/argus
```

### First run

On first launch, a setup wizard asks for:

1. Your Gemini API key
2. A model (Flash Lite, Flash, or Pro)

It saves your choices to `~/.argus/config.yaml`.

You can skip the wizard by setting the key in your environment:

```bash
export GEMINI_API_KEY=your_key_here
```

Or edit the config directly:

```yaml
# ~/.argus/config.yaml
gemini_api_key: "AIza..."
model: "gemini-2.5-flash-lite"   # fastest, free tier (recommended)
log_tail_lines: "200"
```

Environment variables win over the config file.

---

## Usage

### Docker containers

1. Launch `argus` and choose **Docker Container**
2. Pick a container (running or stopped)
3. Argus loads recent history and, for running containers, starts streaming live logs and metrics
4. Ask a question, for example `why is nginx returning 502?`

### Processes

1. `cd` into your project directory
2. Launch `argus` and choose **Process**
3. Enter the command to run, for example `npm run dev`
4. Argus runs it, captures output, and writes a session log to `.argus-session.log` in that directory
5. Ask questions while it runs, or after it exits (stats are shown automatically)

### Log files

1. Launch `argus` and choose **Log File**
2. Enter the path, for example `/var/log/nginx/error.log`
3. Enter a line count, `all`, or press `f` for the full file
4. Ask questions about the loaded logs

### Commands

| Command | What it does | LLM call? |
|---|---|---|
| `/stats` | Error, warning, and info counts and error rate | No |
| `/clear` | Clear the Insights conversation | No |
| `/quit` | Exit Argus | No |

Anything else you type is sent to the assistant as a question.

### Keybindings

| Key | Action |
|---|---|
| `↑` `↓` `PgUp` `PgDn` | Scroll the focused pane |
| `ctrl+u` / `ctrl+d` | Half-page scroll |
| `tab` | Switch focus between logs and Insights |
| `ctrl+f` | Toggle follow mode |
| `end` | Jump to latest logs and resume following |
| `enter` | Submit a query or confirm a selection |
| `esc` | Go back |
| `ctrl+c` | Quit from anywhere |

Inside the workspace, single letters are never bound to actions, so everything you type goes to the query bar. (The log file setup screen does accept `f` as a shortcut for "full file".)

---

## Architecture

### Agents as tools

The orchestrator is the conversational brain. It reads the question, the conversation so far, and any memory context, then either answers directly or asks for a tool:

```
User query
    │
    ▼
Orchestrator (Gemini + conversation history + memory context)
    │
    ├── answer directly      casual replies, follow-ups
    ├── log_analysis         extract errors, repeated failures, anomalies
    ├── rca                  root cause from the analysis output
    └── stats                counts and error rate (no LLM)
    │
    ▼
Tool result is fed back to the orchestrator, which writes the final answer (streamed)
```

### Log pipeline

```
Source (Docker / Process / File)
    │
    ▼
Collector  → normalized LogEntry
    │
    ▼
Scrubber   → redacts secrets and PII (before any LLM call)
    │
    ▼
Orchestrator → agents → streamed answer
```

Every source produces the same `LogEntry` (in `internal/types`), so agents never know whether logs came from Docker, a process, or a file.

### TUI structure

Each source screen (Docker, Process, File) embeds a shared `workspace.Core`. The core owns the log viewer, query bar, Insights pane, layout, and orchestrator wiring. Each screen only adds what is specific to its source, such as loading and streaming. A feature built once in the core works for all three sources.

```
First run → Setup Wizard
    │
    ▼
Welcome → Source Select
    │
    ├── Docker  → Container Select → Docker workspace
    ├── Process → Command Input    → Process workspace
    └── File    → File Setup       → File workspace
```

---

## Project Structure

```
argus/
├── main.go                       entrypoint
├── cmd/
│   └── root.go                   Cobra root command, TUI launch
├── agents/
│   ├── types.go                  shared types, interfaces, event types
│   ├── gemini_agent.go           Google GenAI SDK client (Generate, Chat, ChatStream)
│   ├── orchestrator_agent.go     routing, history, tool dispatch, streaming
│   ├── log_analysis_agent.go
│   ├── rca_agent.go
│   └── stats_agent.go            pure Go, no LLM
├── pipeline/
│   └── scrubber.go               secret and PII redaction
└── internal/
    ├── types/types.go            LogEntry (shared, avoids import cycles)
    ├── config/config.go          Viper loader, Save(), IsFirstRun()
    ├── memory/                   agent.md and runbook context (experimental)
    ├── collectors/
    │   ├── docker/
    │   │   ├── collector.go      list, fetch history, stream
    │   │   └── metrics.go        CPU, memory, network, I/O, PIDs
    │   ├── process/process.go    exec runner, stdout/stderr capture
    │   └── file/file.go          .log / .txt / JSON reader
    └── tui/
        ├── app.go                root model, screen routing
        ├── layout/               responsive layout math and chrome
        ├── workspace/            shared core: state, query handling, view
        ├── screens/              welcome, source select, setup, per-source chat
        ├── components/           log viewer, query bar, thinking, metrics panel
        └── styles/theme.go       all colors and styles in one place
```

---

## Design Decisions

**Official Google GenAI SDK instead of LangChainGo.** LangChainGo's agent executor was unstable and targeted deprecated endpoints. The official SDK tracks new models and gives direct control over streaming and history.

**A hand-rolled orchestrator instead of a framework.** The model asks for a tool in a fixed format, Argus runs it, and the result goes back for synthesis. It is easy to debug and adds no framework overhead.

**Struct embedding for the shared workspace.** Docker, Process, and File screens had about 85% duplicated code. Embedding `workspace.Core` removed the duplication without changing how the root model routes screens.

**Separate log and answer panes.** Logs stream continuously and should auto-scroll. Answers are reference material you scroll back through. One pane for both made them fight each other.

**No single-letter shortcuts.** Early versions bound `f`, `k`, `j`, and `g`, which made those letters impossible to type in the query bar. Shortcuts now use keys that can't be typed as text.

**Gemini by default.** It has a free tier, so there's no billing step on first run. Support for other providers and local models is on the roadmap.

---

## Known Limitations

- **High log volume.** The log viewer re-renders its full buffer on every new line, so very chatty sources can make the UI sluggish or unresponsive. A windowed (virtualized) log view is the planned fix.
- **Metrics are Docker-only.** Process and file sources have no metrics yet.
- **No Kubernetes support yet.**
- **Prebuilt binaries cover Linux and Windows.** macOS is build-from-source.
- **Scrubbing is best-effort.** See the note under Security.
- **Early software (v0.x).** Expect rough edges.

---

## Roadmap

**Done**
- [x] Docker, Process, and Log file sources
- [x] Conversational AI with tool routing and token streaming
- [x] Secret scrubbing before LLM calls
- [x] First-run setup wizard
- [x] Responsive split-pane workspace with follow mode
- [x] Live Docker metrics (CPU, memory, network, I/O, PIDs)
- [x] Shared workspace core across all sources

**Next**
- [ ] Windowed log view for high-volume streams
- [ ] Local insights in the Insights pane (error rate, top repeated errors, no LLM)
- [ ] Aligned log columns (timestamp, level, source, message)
- [ ] Log search and filtering
- [ ] `?` shortcut overlay and command palette

**Later**
- [ ] Kubernetes pod logs (namespace and pod selectors)
- [ ] Prometheus and OpenTelemetry detection for richer metrics
- [ ] Log template parsing to cluster repeated lines and cut token usage
- [ ] Episodic memory: remember past incidents and retrieve similar ones
- [ ] Local LLM support via Ollama, and multi-model support
- [ ] `/export` conversations and logs
- [ ] Homebrew tap and a release pipeline with GitHub Actions
- [ ] `argus watch` daemon mode with anomaly alerts

---

## Research and Inspiration

The planned log-compression work draws on [LogParser-LLM](https://arxiv.org/abs/2408.13727), which combines a prefix parse tree with selective LLM calls to extract log templates cheaply. The UX takes cues from tools like k9s and lazygit.

---

## Contributing

Pull requests are welcome. For larger changes, please open an issue first.

```bash
git clone https://github.com/vishnuprasad2004/argus
cd argus
go mod tidy
go run ./cmd/
```

---

## License

MIT. See [LICENSE](LICENSE).

<!--
NOTES FOR LATER EDITS:
- Record a demo GIF with vhs (https://github.com/charmbracelet/vhs) and add it at the top
- Add screenshots: setup wizard, source select, split workspace
- Add binary size and RAM numbers after profiling
- Remove "High log volume" from Known Limitations once the windowed log view ships
- Add a comparison table vs stern, lnav, k9s
- Add CONTRIBUTING.md
-->
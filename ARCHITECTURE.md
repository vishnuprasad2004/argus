# Argus Architecture

## Scope and status

This document describes the code currently executed by the repository, based
on the implementation and README. It deliberately distinguishes implemented
paths from incomplete or disconnected code. Argus is a single-process Go TUI
that collects one log source at a time and optionally sends scrubbed logs to
the Gemini API for conversational analysis.

## Runtime composition

```text
main.go
  -> cmd.Execute (Cobra)
      -> config.Load
      -> agents.CreateAgentWithConfig
      -> Bubble Tea Program / tui.RootModel
          -> setup or source-selection screens
          -> source-specific chat screen
              -> collector -> []LogEntry -> LogViewer
              -> Orchestrator -> Gemini / local StatsAgent -> answer viewport
```

The root command creates one Gemini client for a configured launch and passes
it to screen models. Each chat screen creates its own `Orchestrator`, so
conversation history and event channels are scoped to that screen session.

## Startup and configuration

`main.go` only calls `cmd.Execute()`. The Cobra root command loads configuration
from `~/.argus/config.yaml`, accepting `GEMINI_API_KEY` as an environment
override. If no config file exists, it launches the setup wizard with no LLM.
Otherwise it builds a Gemini client and starts Bubble Tea in the alternate
screen buffer with mouse motion enabled.

`internal/config` persists an API key, selected Gemini model, and default log
tail line count. Writes use mode `0600`; the `.argus` directory is created with
mode `0755`. `LogTailLines` is configured but source chat screens presently
use their own hard-coded defaults (Docker uses 200 history lines).

## User-interface state machine

`internal/tui/app.go` owns screen selection and routes Bubble Tea messages.

```text
first-run: SetupWizard -> Welcome -> SourceSelect
otherwise: Welcome -> SourceSelect

SourceSelect
  Docker  -> ContainerSelect -> Docker Chat
  Process -> ProcessSetup    -> Process Chat
  Log File-> FileSetup       -> File Chat
  Kubernetes -> unavailable selector item
```

The three chat screens independently implement nearly the same layout:

- a colorized, scrollable `LogViewer`;
- an answer `viewport`;
- `QueryBar` input and preset commands;
- a `ThinkingIndicator` driven by agent events;
- `tab` to change scroll focus and `esc` to return to source selection.

They use `tea.Cmd` functions for blocking work. Docker and process screens
receive streaming logs; file chat loads a snapshot once. Presentation styling
and Markdown rendering are centralized in `internal/tui/styles/theme.go`.

## Log model and collectors

All collectors produce `internal/types.LogEntry`:

```go
type LogEntry struct {
    Timestamp time.Time
    Level     string
    Source    string
    Message   string
    Metadata  map[string]string
}
```

| Source | Package | Current behaviour |
| --- | --- | --- |
| Docker | `internal/collectors/docker` | Lists all containers, fetches historical logs, and streams running-container output through Docker's SDK. It demultiplexes stdout/stderr, parses an RFC3339 timestamp when present, and heuristically assigns a log level. |
| Process | `internal/collectors/process` | Runs a command in Argus's current working directory, separately consumes stdout and stderr, writes a fresh `.argus-session.log`, and reports process exit. Stdout is level-detected; stderr defaults to `ERROR`. |
| File | `internal/collectors/file` | Validates and reads a file, then returns either all lines or a tail. `.json` is interpreted as newline-delimited `LogEntry` JSON; other extensions use heuristic timestamp and level detection. |

The Docker and process collectors expose channels used by their chat screens to
append logs as they arrive. The file collector has no watcher or tail-follow
mode. Process command execution uses `strings.Fields`, rather than a shell, so
quoted arguments and shell syntax are unsupported.

## Analysis and event flow

```text
QueryBar
  -> /stats: StatsAgent directly (local only)
  -> natural-language query
       -> Orchestrator.Run
            -> pipeline.Scrub(copy of LogEntry messages)
            -> Gemini routing request with conversation history
            -> direct answer OR tool dispatch
                 log_analysis -> Gemini prompt over all scrubbed log lines
                 rca          -> log_analysis, then Gemini RCA prompt
                 stats        -> local counts/error rate
            -> streamed final Gemini response
       -> AgentEvent channel -> ThinkingIndicator / streamed answer viewport
```

`Orchestrator` stores ordered user/assistant turns in memory for the duration
of the chat screen. It asks Gemini to decide whether a tool is needed. A
tool result is then supplied to Gemini for synthesis into the final response.
Direct answers are also generated a second time through the streaming method,
after the initial routing request.

`StatsAgent` counts `ERROR`, `WARN`, `INFO`, and `DEBUG`, and calculates error
rate as `ERROR / total logs`. It does not use Gemini. If an LLM call fails,
the orchestrator attempts to return local stats and emits a warning event.

The orchestrator emits `AgentEvent` values on a buffered channel. The relevant
chat screen starts a one-event-at-a-time listener and reschedules it to render
tool status and streaming chunks. Consumers are therefore required while an
orchestrator run is in progress.

## Data handling and security boundary

`pipeline.Scrub` returns a copied log slice, replacing matching API keys,
passwords, tokens, secrets, common PII, authorization headers, and AWS access
keys in the `Message` field. The scrubber is called immediately inside
`Orchestrator.Run`, before log-analysis or root-cause prompts are assembled.

It does not scrub `Source` or `Metadata`, and it does not protect data sent by
any future direct Gemini call that bypasses `Orchestrator.Run`. New AI-facing
features should use the same scrubbed log boundary.

## Persistent user context

`internal/memory` manages these external per-user files:

```text
~/.argus/
  config.yaml
  agent.md
  runbooks/*.md
  memory/episodes.json  (path reserved; no episode persistence implemented)
```

`Store.Build` reads non-placeholder `agent.md` content and runbooks selected by
simple name matching against the query and source names; it injects this into
the orchestration system prompt. `NewOrchestrator` creates this store on each
chat session.

The package includes helpers to append remembered facts and runbook steps.
However, the orchestration `Run` method does not call its `parseRememberCommand`
or `handleRemember` helpers, so “remember …” does not currently persist facts
through the normal chat path. The wizard also currently transitions from model
selection directly to `saveConfig`, which makes its stack, services, and
preferences steps—and `saveAll`—unreachable.

## Boundaries and dependencies

```text
cmd                 -> agents, config, tui
tui/screens         -> agents, collectors, components, styles
agents              -> config, memory, pipeline, types, Google GenAI SDK
collectors          -> types (+ Docker SDK where applicable)
pipeline            -> types
memory/config       -> filesystem
```

Primary external libraries are Bubble Tea/Bubbles/Lipgloss/Glamour for the
terminal UI, Cobra/Viper for startup/configuration, Docker's Go SDK for
container logs, and `google.golang.org/genai` for Gemini. `langchaingo` remains
in dependencies because a legacy type is still present in `screens/messages.go`,
but current agent execution uses the official Google GenAI SDK.

## Known gaps and maintenance implications

- Kubernetes is deliberately unavailable in the source selector; no collector
  or chat flow exists.
- The README's broad product direction should not be read as proof of a wired
  feature. In particular, conversational memory has retrieval code but the
  remember command path is disconnected.
- There are no repository tests at present. `go test ./...` succeeds by
  compiling all packages.
- `main.go` retains extensive commented-out prototype code. The active entry
  point is the final, minimal `main` function only.
- `file_chat.go` contains diagnostic writes to `/tmp/argus-debug.log`, an
  implementation detail that is not part of the normal persistence model.

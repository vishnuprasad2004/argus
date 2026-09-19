# Agent Guide for Argus

## Project at a glance

Argus is a Go terminal application for inspecting Docker, child-process, and
file logs, then asking Gemini for an analysis. It is a Bubble Tea application
launched through Cobra. The module is `github.com/vishnuprasad2004/argus` and
the declared Go version is 1.26.0.

The active entry path is:

`main.go` -> `cmd.Execute()` -> `cmd/root.go` -> Bubble Tea root model.

Do not treat large commented-out sections in `main.go` as active behaviour.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `cmd/` | Cobra root command, configuration and Gemini client startup. |
| `agents/` | Gemini wrapper, orchestration, LLM specialist agents, and local stats. |
| `internal/collectors/` | Docker, process, and file acquisition normalized to `types.LogEntry`. |
| `internal/config/` | `~/.argus/config.yaml` loading and saving. |
| `internal/memory/` | Per-user `~/.argus/agent.md` and runbook storage/retrieval. |
| `internal/tui/` | Bubble Tea root model, screens, reusable components, and styles. |
| `pipeline/` | Pre-LLM secret and PII scrubbing. |

## Core invariants

- Use `internal/types.LogEntry` as the common log interchange type. Collectors
  must populate it rather than exposing source-specific records beyond their
  package boundary.
- Scrub log messages with `pipeline.Scrub` before any LLM-bound prompt. Keep
  new data paths within this boundary.
- The `StatsAgent` is local-only: do not introduce an LLM call for `/stats`.
- Chat screens own their cancellation context and must cancel it when leaving a
  live source. Long-running I/O belongs in `tea.Cmd` functions, not directly in
  a Bubble Tea `Update` method.
- `agents.Orchestrator.Events` must have an active consumer when `Run` is used;
  sending an event can otherwise block the orchestration goroutine.
- Configuration and user memory live outside the repository, under
  `~/.argus/`. Never add a real API key, generated session log, or user memory
  to version control.

## How to work in this repository

1. Find the active source-specific chat screen before changing an interaction:
   `chat.go` for Docker, `process_chat.go` for child processes, and
   `file_chat.go` for files. They intentionally share a similar shape but are
   not yet a common abstraction.
2. Keep TUI state updates value-safe. Bubble Tea models are returned by value;
   mutate pointer-owned components such as `LogViewer` deliberately and return
   the updated enclosing model.
3. Prefer new files/packages only when a concern cannot naturally live with its
   source, agent, or UI layer. Avoid import cycles across `agents`, `internal`,
   and `pipeline`.
4. Document user-visible behaviour in `README.md` only after it is actually
   wired into the active UI and command path.

## Local commands

```powershell
go test ./...
go build -o bin/argus ./cmd/
go run ./cmd/
```

`make build`, `make run`, and `make test` offer the same workflows in a
Unix-like shell. The Makefile's `clean` target uses `rm -rf`, so use it only
when intentionally removing `bin/` artifacts.

## Testing expectations

There are currently no `_test.go` files. Add focused tests alongside changed
logic, especially for parsers, scrubbing patterns, configuration, memory
selection, and agent routing. Run `go test ./...` after Go changes; run
`gofmt` on modified Go files.

## Current implementation caveats

- Kubernetes is displayed as unavailable; there is no Kubernetes collector.
- File ingestion is snapshot-only, not a live tail.
- A process command is split with `strings.Fields`; shell quoting, pipes,
  redirects, and environment assignments are not interpreted.
- The orchestrator has helper functions for `remember` commands, but `Run`
  does not currently invoke them. The wizard currently saves after model
  selection through `saveConfig`, so its later stack/services/preferences
  stages and `saveAll` are unreachable.
- `file_chat.go` writes diagnostics to `/tmp/argus-debug.log`; retain or remove
  that explicitly rather than assuming it is standard application logging.

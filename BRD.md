# Argus — Big Functional Document (BFD)
> Lightweight Terminal-Native SRE AI Agent · Go CLI + Bubble Tea TUI · Kubernetes + Prometheus + LLM RCA

**Version:** 0.1 (MVP)
**Author:** Vishnu
**Last updated:** 2026-05-26
**Status:** Pre-build · Design locked

---

## 1. The Problem

Every SRE and backend developer working locally runs the same painful stack:

| Tool | RAM usage | Purpose |
|---|---|---|
| Docker Desktop | ~500MB | Container runtime |
| Minikube | ~800MB | Local K8s cluster |
| Lens IDE | ~400MB | K8s dashboard |
| VS Code / IntelliJ | ~1–2GB | Editor |
| Grafana + Prometheus | ~300MB | Metrics dashboard |
| Browser (kubectl dashboards) | ~300MB | Everything else |

On a laptop with 8–16GB RAM, this stack eats **3–5GB before you write a single line of code.** When something breaks in the cluster, you're tab-switching between Lens, a Grafana dashboard, `kubectl` in a terminal, and your log viewer — while your fan screams and your laptop throttles.

The real problem: **there is no single lightweight tool that does log tailing + metric anomaly detection + AI-assisted root cause analysis in one place, from the terminal, with near-zero RAM overhead.**

---

## 2. The Solution — Argus

**Argus** (named after the hundred-eyed giant of Greek mythology who never sleeps) is a single Go binary CLI tool that replaces the Lens + Grafana dashboard + kubectl tab-switching workflow for day-to-day SRE tasks on a developer laptop.

It connects to a running Kubernetes cluster via kubeconfig, pulls logs and Prometheus metrics concurrently, feeds a structured context window to an LLM (Claude 3.5 Sonnet via OpenRouter), and streams back a root cause analysis — all inside a keyboard-driven Bubble Tea TUI in the terminal.

**One binary. ~15MB. <30MB RAM at runtime. No JVM. No Python env. No browser.**

### Core value proposition

```
$ argus analyze --namespace production

┌─ Argus SRE Agent ─────────────────────────────────────────────┐
│ Namespace: production        Cluster: minikube                 │
│ Pods: 6 running, 1 degraded  Prometheus: connected            │
├──────────────────────────────────────────────────────────────── │
│ ⚠  INCIDENT DETECTED — payments-svc                           │
│                                                                 │
│ Summary: OOMKilled 3x in 10 min, cascading 5xx to api-gateway │
│ Root cause: Unclosed DB connection handles in pool (high conf) │
│ Affected: payments-svc, api-gateway                            │
│                                                                 │
│ Suggested fixes:                                               │
│  1. Increase memory limit: resources.limits.memory: 512Mi      │
│  2. Patch HikariCP maxPoolSize or add connection timeout       │
│  3. Check DB_MAX_CONNECTIONS env var — currently unset         │
│                                                                 │
│ Confidence: HIGH    [r] re-analyze  [l] view logs  [q] quit   │
└────────────────────────────────────────────────────────────────┘
```

---

## 3. Target Users

| Persona | Pain Argus solves |
|---|---|
| Fresher/mid SRE on a dev laptop | Replaces Lens + Grafana on low-RAM machines |
| Backend dev running local K8s | Quick incident triage without leaving terminal |
| Platform engineer at a startup | Faster RCA than reading raw logs manually |
| Senior SRE (v2 target) | Team-shared runbook RAG + alert integrations |

> **MVP focus:** solo developer on Minikube. Cloud cluster support (EKS, GKE) is v2.

---

## 4. MVP Feature Scope

### 4.1 Commands

#### `argus logs`
Tail and filter pod logs in a live Bubble Tea panel.

```
argus logs --pod payments-svc --namespace production --tail 100
argus logs --namespace production          # shows pod picker TUI
argus logs --pod payments-svc --since 15m  # last 15 minutes
argus logs --pod payments-svc --grep ERROR # filter lines
```

Behaviour:
- Streams logs line-by-line using `client-go` `PodLogs` API with `Follow: true`
- Lines are piped through a goroutine into the Bubble Tea message loop
- TUI shows scrollable log panel with timestamp, pod name, and line content
- Color-coded: ERROR lines red, WARN lines yellow, INFO lines default
- `[/]` to enter filter mode, `[q]` to quit, `[s]` to save to file

---

#### `argus analyze`
Core command. Fetches logs + metrics + events, builds context, calls LLM, streams RCA.

```
argus analyze --namespace production
argus analyze --pod payments-svc
argus analyze --since 15m --namespace staging
```

Behaviour:
- Concurrently (goroutines) fetches:
  - Pod logs (last 200 lines per pod in namespace, or single pod)
  - Prometheus anomalies (CPU/mem spikes, error rate spikes, last N minutes)
  - K8s warning events (OOMKilled, CrashLoopBackOff, BackOff, FailedScheduling)
- Truncates log context intelligently to stay within LLM token limit (~6000 tokens)
- Builds structured prompt (see Section 7)
- Streams response token-by-token into TUI RCA panel
- Outputs structured JSON: summary, root\_cause, confidence, affected\_services, suggested\_fixes, runbook\_hints
- Renders each JSON field as a separate styled TUI section

---

#### `argus top`
Lightweight Prometheus metrics viewer — replaces Grafana for quick checks.

```
argus top --namespace production
argus top --pod payments-svc --watch  # refresh every 5s
```

Behaviour:
- Queries Prometheus HTTP API for key metrics per pod:
  - CPU usage (millicores) vs limit
  - Memory usage (MB) vs limit
  - HTTP error rate (5xx / total, last 5 min)
  - Restart count
- Renders as a live-updating table in Bubble Tea TUI
- Color thresholds: green < 70%, yellow 70–90%, red > 90%
- `[a]` to jump to `analyze` for a selected pod

---

#### `argus events`
Kubernetes warning events viewer with AI triage.

```
argus events --namespace production
argus events --namespace production --ai   # annotate with LLM severity
```

Behaviour:
- Lists all Warning-type K8s events for the namespace (last 1 hour by default)
- Columns: time, pod, reason, message, count
- `--ai` flag: sends event list to LLM, gets back severity (P1/P2/P3) and one-line triage note per event
- Rendered as a table in Bubble Tea, sortable by time or severity

---

### 4.2 TUI Design

Built with **Bubble Tea** (model/update/view) + **Lipgloss** (styles).

**Global TUI layout:**

```
┌─ Argus ──────────────────── minikube · production ─── 14:32:01 ─┐
│  [1] Logs  [2] Analyze  [3] Top  [4] Events  [?] Help  [q] Quit │
├─────────────────────────────────────────────────────────────────── │
│                                                                    │
│                   [ main content panel ]                          │
│                                                                    │
├─────────────────────────────────────────────────────────────────── │
│ Status: Connected · Pods: 6/6 healthy · Prometheus: OK            │
└────────────────────────────────────────────────────────────────────┘
```

- Number keys switch between the 4 views (tabs)
- Each view is a separate Bubble Tea model composed into the root model
- `[r]` refreshes current view
- `[c]` opens cluster/namespace switcher
- `[q]` or `Ctrl+C` quits cleanly

**TUI models:**
- `RootModel` — holds active tab, cluster info, status bar
- `LogsModel` — viewport + streaming log lines + filter state
- `AnalyzeModel` — spinner → streaming RCA text → structured result panels
- `TopModel` — table with auto-refresh ticker
- `EventsModel` — table + optional AI annotation overlay

---

### 4.3 Configuration

Config file: `~/.argus/config.yaml`

```yaml
# ~/.argus/config.yaml

cluster:
  kubeconfig: ""          # empty = default ~/.kube/config
  context: ""             # empty = current context
  default_namespace: "default"

ai:
  provider: "openrouter"
  api_key: ""             # or set ARGUS_AI_KEY env var
  model: "anthropic/claude-3.5-sonnet"
  max_tokens: 1000
  timeout_seconds: 30

prometheus:
  url: "http://localhost:9090"   # port-forward or in-cluster
  timeout_seconds: 10

logs:
  default_tail_lines: 200
  default_since: "15m"
  max_context_lines: 500        # cap for LLM context building

tui:
  refresh_interval_seconds: 5   # for argus top --watch
  theme: "dark"                 # dark | light
```

**Config precedence:** CLI flags > env vars > config file > defaults.

Key env vars:
- `ARGUS_AI_KEY` — OpenRouter API key
- `ARGUS_PROMETHEUS_URL` — Prometheus endpoint
- `ARGUS_NAMESPACE` — default namespace
- `KUBECONFIG` — standard K8s env var, respected automatically

---

## 5. Project Structure

```
argus/
├── cmd/
│   ├── main.go            ← binary entrypoint
│   ├── root.go            ← Cobra root command, global flags, config init
│   ├── logs.go            ← `argus logs` command
│   ├── analyze.go         ← `argus analyze` command
│   ├── top.go             ← `argus top` command
│   └── events.go          ← `argus events` command
│
├── internal/
│   ├── k8s/
│   │   ├── client.go      ← kubeconfig loader, clientset init
│   │   ├── logs.go        ← pod log streaming (Follow + tail)
│   │   ├── events.go      ← warning events lister
│   │   └── pods.go        ← pod lister, status checker
│   │
│   ├── metrics/
│   │   ├── prometheus.go  ← Prometheus HTTP API client (PromQL queries)
│   │   └── anomaly.go     ← spike detection, threshold checks
│   │
│   ├── ai/
│   │   ├── client.go      ← OpenRouter HTTP client, streaming support
│   │   ├── rca.go         ← context builder, RCA response parser
│   │   └── prompts.go     ← system prompt + user prompt templates
│   │
│   ├── tui/
│   │   ├── root.go        ← RootModel (tabs, status bar, key routing)
│   │   ├── logs.go        ← LogsModel (viewport, streaming, filter)
│   │   ├── analyze.go     ← AnalyzeModel (spinner, streaming RCA, result)
│   │   ├── top.go         ← TopModel (table, auto-refresh)
│   │   ├── events.go      ← EventsModel (table, AI annotation)
│   │   └── styles.go      ← Lipgloss style definitions (colors, borders)
│   │
│   └── config/
│       └── config.go      ← Viper config loader, Config struct
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 6. Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Language | Go 1.22+ | Single binary, low RAM, CNCF-native, fast compile |
| CLI framework | Cobra + Viper | Same as kubectl, Helm — industry standard |
| TUI framework | Bubble Tea + Lipgloss | Same as k9s, lazygit — Elm-arch, clean |
| Kubernetes client | client-go | Official K8s Go client |
| Metrics | prometheus/client_golang | Prometheus HTTP API v1 querier |
| AI | OpenRouter HTTP API (stdlib net/http) | No Go SDK needed, full control, free tier |
| LLM model | claude-3.5-sonnet via OpenRouter | Best reasoning for log analysis |
| Config | Viper | YAML config + env var + flag merging |
| Build | Makefile + GoReleaser (v2) | Cross-platform binary release |

**No database. No server. No Docker image needed to run it.**

---

## 7. LLM RCA Prompt Design

### System prompt
```
You are an expert Site Reliability Engineer performing root cause analysis.
You will receive structured incident context from a Kubernetes cluster:
pod logs, Prometheus metric anomalies, and Kubernetes warning events.

Analyze the data and respond ONLY with a valid JSON object. No markdown, no preamble.
Use this exact schema:
{
  "summary": "one sentence — what happened",
  "root_cause": "technical explanation of the likely root cause",
  "confidence": "high | medium | low",
  "affected_services": ["list of service names"],
  "suggested_fixes": ["concrete fix 1", "concrete fix 2"],
  "runbook_hints": ["what to check next", "related symptoms to watch"]
}
```

### User prompt (built at runtime)

```
=== INCIDENT CONTEXT ===
Namespace: production
Time window: last 15 minutes
Analysis triggered: 2026-05-26T14:32:01Z

=== POD LOGS ===
[payments-svc-7d9f8b-xkp2q] (last 200 lines, truncated to 2000 tokens)
2026-05-26T14:28:11Z ERROR HikariPool-1 - Connection is not available, request timed out after 30000ms
2026-05-26T14:28:12Z ERROR Failed to acquire connection from pool
...

=== PROMETHEUS ANOMALIES ===
- container_memory_usage_bytes{pod="payments-svc"}: 487Mi — 3.1x above p95 baseline (156Mi)
- http_requests_total{status="5xx", service="payments-svc"}: 47 errors in last 5 min (baseline: 0.2/min)
- container_cpu_usage_seconds_total: normal

=== KUBERNETES EVENTS ===
- 14:29:03  OOMKilled     payments-svc-7d9f8b-xkp2q    count: 3
- 14:29:45  BackOff       payments-svc-7d9f8b-xkp2q    Back-off restarting failed container
- 14:30:12  Pulling       payments-svc-7d9f8b-xkp2q    Pulling image for restart
```

### Context window budget

| Section | Token budget |
|---|---|
| System prompt | ~200 tokens |
| Pod logs | ~2500 tokens (hard cap, truncate oldest first) |
| Prometheus anomalies | ~300 tokens |
| K8s events | ~300 tokens |
| Response (max_tokens) | 1000 tokens |
| **Total** | **~4300 tokens** — well within Claude 3.5 Sonnet's context |

---

## 8. Key Go Concepts Used (learning map)

| Concept | Where used in Argus | Difficulty |
|---|---|---|
| Structs + interfaces | Every internal package — `K8sClient`, `MetricsClient`, `AIClient` all implement interfaces | Easy |
| Error handling `(val, err)` | Everywhere — get used to `if err != nil` | Easy |
| Goroutines | Log streaming: one goroutine reads, sends lines to channel | Medium |
| Channels | `chan LogLine` pipes from k8s streamer to TUI message loop | Medium |
| `context.Context` | Cancellation for log streaming, HTTP timeouts | Medium |
| `net/http` | OpenRouter API calls, Prometheus HTTP API | Easy |
| Bubble Tea `tea.Model` | All TUI views implement `Init()`, `Update()`, `View()` | Medium |
| `encoding/json` | Parsing LLM structured output, Prometheus API response | Easy |
| `sync.WaitGroup` | Concurrent fetch of logs + metrics + events in `analyze` | Medium |

---

## 9. V2 Roadmap (post-MVP)

| Feature | Description |
|---|---|
| Cloud cluster support | Auto-detect EKS/GKE/AKS from kubeconfig, no code change needed |
| RAG on runbooks | Index your `runbooks/` markdown folder into a local vector store (bbolt + embeddings), retrieve relevant docs during RCA |
| GitHub integration | Auto-create GitHub issue on P1 incident with RCA pre-filled |
| Gmail alerts | Send incident summary digest to email via Gmail API |
| Honeycomb trace correlation | Pull distributed traces for a failing service alongside logs |
| Loki log source | Alternative to direct K8s logs — query Loki for richer log search |
| `brew install argus` | GoReleaser + Homebrew tap for one-line install |
| OpenTelemetry spans | Correlate OTel trace IDs found in logs with span data |
| `argus watch` | Daemon mode — continuously monitors namespace, alerts on anomalies |

---

## 10. Build Plan — 4 Weeks

### Week 1 — Go fundamentals + K8s layer
- Go tour: Basics, Methods/Interfaces, Concurrency sections only
- Project scaffold: `go mod init github.com/yourusername/argus`
- Cobra root command + `logs` subcommand skeleton
- `internal/config`: Viper config loader
- `internal/k8s/client.go`: kubeconfig loader, clientset init
- `internal/k8s/logs.go`: pod log streaming with `Follow: true`
- `internal/k8s/pods.go`: list pods by namespace
- **Milestone:** `argus logs --pod X --namespace Y` streams live logs to stdout

### Week 2 — Prometheus + AI engine
- `internal/metrics/prometheus.go`: HTTP API client, PromQL queries for CPU/mem/errors
- `internal/metrics/anomaly.go`: simple spike detection (value vs rolling average)
- `internal/k8s/events.go`: warning events lister
- `internal/ai/client.go`: OpenRouter HTTP client with streaming (SSE)
- `internal/ai/prompts.go`: system prompt + runtime context builder
- `internal/ai/rca.go`: JSON response parser, token budget enforcer
- **Milestone:** `argus analyze --namespace X` prints full RCA to stdout

### Week 3 — Bubble Tea TUI
- `internal/tui/styles.go`: Lipgloss color theme, borders, badges
- `internal/tui/root.go`: RootModel with tab switching, status bar
- `internal/tui/logs.go`: LogsModel — viewport, streaming, `/` filter
- `internal/tui/analyze.go`: AnalyzeModel — spinner → streaming text → structured result
- `internal/tui/top.go`: TopModel — table with auto-refresh ticker
- `internal/tui/events.go`: EventsModel — events table
- Wire TUI into all 4 commands
- **Milestone:** Full TUI working — all 4 views navigable with keyboard

### Week 4 — Polish + ship
- `argus top --watch` auto-refresh
- `argus events --ai` annotation mode
- `~/.argus/config.yaml` auto-creation on first run with defaults
- Graceful shutdown (context cancellation on `Ctrl+C`)
- `Makefile`: `make build`, `make run`, `make release`
- README with demo GIF (use `vhs` or `asciinema`)
- GitHub Actions: build + release binary for Linux/macOS/Windows
- **Milestone:** Public GitHub repo, downloadable binary, demo GIF in README

---

## 11. Resume Talking Points

**One-liner:**
> "Built Argus — an open-source Go CLI SRE agent that replaced Lens, Grafana, and kubectl tab-switching with a single 15MB binary. Connects to Kubernetes via client-go, queries Prometheus concurrently using goroutines, and streams AI-powered root cause analysis via Claude 3.5 Sonnet — all inside a Bubble Tea TUI."

**Technical depth answers:**

- *Why Go?* — "The entire CNCF toolchain — kubectl, Helm, k9s, Prometheus — is Go-native. A single binary with no runtime dependencies is the right distribution model for a developer tool."
- *Why not your Spring Boot version?* — "Spring Boot was great for validating the agent architecture and prompt design. Go gives me a 15MB binary vs a 200MB JAR + JVM. The right tool for the right job."
- *Hardest part?* — "Integrating Bubble Tea's Elm-style message loop with live goroutine-based log streaming. A `chan tea.Msg` bridges the K8s stream and the TUI event loop cleanly."
- *Prometheus integration?* — "Querying the HTTP API v1 `/query_range` endpoint with PromQL, then applying a simple z-score baseline to flag anomalies — no heavyweight SDK needed."

**Companies this resonates with:**
Datadog, Grafana Labs, Honeycomb, HashiCorp, any cloud team at Microsoft/Google/AWS, AutoRABIT (DevOps tooling), any startup with a Platform/SRE team.

---

## 12. Non-Goals (MVP)

- No multi-user / auth — single developer tool
- No persistent storage — stateless, no DB
- No cloud cluster support in v1 (v2)
- No Helm chart or in-cluster deployment
- No Windows-native TUI (works via WSL2)
- No PagerDuty / Slack alerts in MVP (v2)
- No RAG on runbooks in MVP (v2)
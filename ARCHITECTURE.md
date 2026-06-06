# Argus — Complete System Architecture
> AI-Powered Log Intelligence Platform · Go · Terminal-Native · Local-First

**Classification:** Staff/Principal Engineer Design Doc  
**Status:** Greenfield Architecture v1.0  
**Scope:** MVP (6–8 weeks) → Production path  

---

## 0. First Principles & Key Decisions

Before prescribing architecture, challenge the assumptions:

**Why not just use Grafana + Loki + Prometheus?**  
Because that stack requires 4+ processes, a browser, and 2–3GB RAM. Argus's entire value is replacing it with one binary in a terminal. The architecture must be RAM-first, not feature-first.

**Why not use an existing log agent (Fluentd, Vector, OpenTelemetry Collector)?**  
We could embed Vector's concepts but not its binary — it's Rust, not Go, and adds a hard dependency. We implement a lightweight subset of its pipeline model natively in Go. This keeps the binary single and small.

**Should AI touch every log line?**  
No. This is the most important architectural decision. Sending raw logs to an LLM is expensive, slow, and noisy. AI should only touch *signals* — anomalies, incidents, summarized windows. The pipeline must do heavy pre-processing before the AI layer ever sees data.

**Single binary or daemon + client?**  
Both. `argus daemon` runs as a background collector process (optional, for persistent watching). `argus` TUI connects to it via Unix socket. If no daemon is running, the TUI runs embedded collectors directly. This gives you flexibility without forcing a daemon.

---

## 1. System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ARGUS SYSTEM                                │
│                                                                     │
│  ┌──────────────┐    ┌─────────────────────────────────────────┐   │
│  │   SOURCES    │    │            ARGUS DAEMON                 │   │
│  │              │───▶│  ┌──────────┐  ┌──────────┐            │   │
│  │  • processes │    │  │Collectors│─▶│ Pipeline │            │   │
│  │  • docker    │    │  └──────────┘  └────┬─────┘            │   │
│  │  • k8s pods  │    │                     │                   │   │
│  │  • log files │    │  ┌──────────────────▼──────────────┐   │   │
│  │  • journald  │    │  │         Ring Buffer              │   │   │
│  │  • syslog    │    │  │    (in-memory, configurable)     │   │   │
│  │  • cloud     │    │  └─────────┬──────────┬────────────┘   │   │
│  └──────────────┘    │            │          │                 │   │
│                      │  ┌─────────▼─┐  ┌────▼──────────┐     │   │
│                      │  │ SQLite    │  │  AI Engine    │     │   │
│                      │  │ + FTS5    │  │  (selective)  │     │   │
│                      │  └─────────┬─┘  └────┬──────────┘     │   │
│                      │            │          │                 │   │
│                      │  ┌─────────▼──────────▼──────────┐    │   │
│                      │  │       Unix Socket API          │    │   │
│                      │  └────────────────┬───────────────┘    │   │
│                      └───────────────────┼─────────────────────┘   │
│                                          │                         │
│  ┌───────────────────────────────────────▼─────────────────────┐   │
│  │                    ARGUS TUI (Bubble Tea)                    │   │
│  │   Logs │ Analyze │ Top │ Events │ Query │ Incidents │ Chat   │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Log Ingestion — Heterogeneous Sources

### 2.1 The Collector Interface

Every source implements one interface. This is the extensibility contract:

```go
// internal/collector/collector.go

type LogLine struct {
    ID        string            // ulid — sortable, unique
    Timestamp time.Time
    Source    SourceMeta
    Raw       string            // original unmodified line
    Level     Level             // parsed: DEBUG/INFO/WARN/ERROR/FATAL/UNKNOWN
    Message   string            // extracted message body
    Fields    map[string]string // structured fields if JSON log
    TraceID   string            // if present in log line
    SpanID    string            // if present
}

type SourceMeta struct {
    Kind      SourceKind   // process | docker | k8s | file | journal | syslog | cloud
    Name      string       // "payments-svc", "nginx", "/var/log/app.log"
    Namespace string       // k8s namespace or ""
    Container string       // docker/k8s container name or ""
    Labels    map[string]string
}

type Collector interface {
    Name()    string
    Collect(ctx context.Context, out chan<- LogLine) error
    Healthy() bool
}
```

Every collector is just a goroutine pumping `LogLine` into a channel. The pipeline doesn't care where it came from.

### 2.2 Source Implementations

| Source | Implementation strategy | Go package |
|---|---|---|
| **Process** (`npm run dev`, `go run .`) | `exec.Cmd` with `StdoutPipe` + `StderrPipe`, line scanner | `os/exec` |
| **Docker containers** | Docker Engine API `/containers/{id}/logs?follow=true` via HTTP streaming | `docker/docker/client` |
| **Kubernetes pods** | `client-go` `PodLogs` with `Follow: true`, concurrent per-pod goroutines | `k8s.io/client-go` |
| **Log files** | `fsnotify` for inotify-based tail (not polling), seek to EOF on start | `fsnotify/fsnotify` |
| **Journald** | `sd_journal_get_fd()` via CGo OR parse `journalctl -f -o json` subprocess (CGo-free) | subprocess |
| **Syslog** | Embedded UDP+TCP syslog server on configurable port, RFC5424 parser | `net` stdlib |
| **Docker Compose** | Iterate all containers in compose project, fan-in with `docker/docker/client` | above |
| **Cloud (AWS CloudWatch)** | AWS SDK `FilterLogEvents` with polling (no push available) | `aws-sdk-go-v2` |
| **Cloud (GCP Logging)** | `logging.googleapis.com` streaming read API | `cloud.google.com/go/logging` |
| **stdin pipe** | `bufio.Scanner(os.Stdin)` — `argus pipe` command, e.g. `npm run dev \| argus pipe` | stdlib |

### 2.3 Process Watcher (most novel collector)

The process collector is Argus's most distinctive feature — wrapping any local command:

```
argus watch --cmd "npm run dev" --name frontend
argus watch --cmd "java -jar app.jar" --name payments-svc
```

This forks the process as a child, pipes stdout/stderr, and streams lines with the process name as source. When the process dies, Argus detects it and can optionally restart it (if `--restart` flag set). This is a lightweight Supervisor + log collector in one.

**Risk:** Process becomes orphan if Argus crashes. Mitigation: write PID to `~/.argus/pids/` and clean up on startup.

### 2.4 Auto-discovery

When run with `argus watch --auto`, Argus auto-discovers:
1. All running Docker containers (via Docker socket)
2. All pods in current kubeconfig context + namespace
3. Common log files: `/var/log/*.log`, `~/.pm2/logs/`, `./logs/*.log`

Auto-discovery runs once on startup and on a configurable interval (default 30s) to catch new containers/pods.

---

## 3. The Ingestion Pipeline

This is the heart of Argus. Inspired by the Unix pipeline philosophy and Vector's topology model.

```
Collectors (N goroutines)
     │
     ▼  chan LogLine (buffered, 10k)
  Fan-in multiplexer
     │
     ▼
  ┌──────────────────────────────────┐
  │         PIPELINE STAGES          │
  │                                  │
  │  1. Parse & Normalize            │
  │  2. Enrich                       │
  │  3. Filter / Sampling            │
  │  4. Anomaly Pre-scorer           │
  │  5. Windowed Aggregator          │
  │  6. Incident Detector            │
  └──────────────┬───────────────────┘
                 │
         ┌───────┴────────┐
         ▼                ▼
    Ring Buffer       SQLite + FTS5
    (hot path)        (cold path)
         │
         ▼
    AI Trigger
    (selective)
```

### 3.1 Stage 1 — Parse & Normalize

Auto-detects log format per source, applies the right parser:

```go
type Parser interface {
    Detect(line string) bool   // returns true if this parser matches
    Parse(line string) (LogLine, error)
}
```

Parsers registered in priority order:
1. **JSON parser** — detects `{` prefix, unmarshals, maps common field names (`msg`/`message`, `level`/`severity`, `ts`/`time`/`timestamp`, `trace_id`)
2. **Logfmt parser** — `key=value key="value"` format (Go standard, 12-factor apps)
3. **Logrus/Zap/Zerolog** — specific JSON schema detection
4. **Java log4j/logback** — `YYYY-MM-DD HH:mm:ss.SSS [thread] LEVEL logger - message`
5. **Apache/Nginx access log** — combined log format regex
6. **RFC5424 syslog** — structured syslog
7. **Python logging** — `LEVEL:logger:message`
8. **Fallback** — raw line, timestamp = now, level heuristic (scan for ERROR/WARN keywords)

Level heuristic for unstructured logs scans for keywords: `fatal|panic` → FATAL, `error|err|exception|traceback` → ERROR, `warn|warning` → WARN, `debug|trace` → DEBUG, else INFO.

### 3.2 Stage 2 — Enrich

Adds derived fields cheaply (no I/O):
- **Fingerprint**: `xxhash(source.Name + normalized_message_template)` — strips variable parts (UUIDs, IPs, numbers) to group repeated errors. This is how Sentry does error grouping.
- **Message template extraction**: replace `[a-f0-9-]{36}` → `<uuid>`, `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}` → `<ip>`, numbers → `<n>`. Template becomes the grouping key.
- **Stack trace detection**: multi-line join — if a line starts with whitespace/`at ` after an ERROR, it belongs to the previous error's stack trace.

### 3.3 Stage 3 — Filter & Sampling

Configurable in `~/.argus/config.yaml`:

```yaml
pipeline:
  filters:
    - drop_level: DEBUG        # drop DEBUG lines from storage (keep in ring buffer)
    - drop_source: "healthz"   # drop health check noise
  sampling:
    rate: 1.0                  # 1.0 = keep all, 0.1 = keep 10% of INFO
    always_keep: [ERROR, FATAL, WARN]
```

Sampling is critical for high-volume sources (nginx access logs can be 10k lines/sec). Drop INFO/DEBUG at high rates, always keep errors.

### 3.4 Stage 4 — Anomaly Pre-scorer

**This runs before AI and is the key to not sending everything to an LLM.**

Per-source sliding window counters (in memory, 1-min and 5-min windows):

```go
type WindowStats struct {
    ErrorRate     float64   // errors per minute
    ErrorRateDiff float64   // delta vs previous window
    UniqueErrors  int       // distinct fingerprints
    TotalLines    int64
    P99Latency    float64   // if latency found in logs
}
```

Scoring rules (configurable thresholds):
- Error rate > 5x baseline → score += 0.4
- New error fingerprint never seen before → score += 0.3
- OOMKilled / panic / segfault / SIGSEGV in line → score += 0.5 (immediate)
- Error rate spike (>3x in 60s) → score += 0.3
- Consecutive same error > 10x → score += 0.2

Score ≥ 0.6 → trigger incident detection. Score ≥ 0.4 → flag for AI analysis queue. Score < 0.4 → store only, no AI.

**This means AI only fires on genuinely anomalous signal, not routine logging.**

### 3.5 Stage 5 — Windowed Aggregator

Groups log lines into 1-minute tumbling windows per source. Used for:
- Summary generation (AI summarizes the window, not every line)
- Storage compaction (store summary + raw count instead of every INFO line)
- Time-series metrics export (to embedded Prometheus-compatible endpoint)

### 3.6 Stage 6 — Incident Detector

An incident is declared when:
1. Anomaly pre-scorer crosses threshold (0.6), OR
2. A known incident pattern matches (configurable regex/keyword rules), OR
3. AI RCA identifies a root cause (feedback loop)

```go
type Incident struct {
    ID           string       // ulid
    StartedAt    time.Time
    ResolvedAt   *time.Time
    Sources      []string
    Severity     Severity     // P1/P2/P3/P4
    Title        string       // AI-generated one-liner
    RootCause    string       // AI RCA
    AffectedLines []string    // log line IDs
    Fingerprints  []string    // error fingerprints involved
    Status       Status       // open | investigating | resolved
    Timeline     []TimelineEvent
}
```

---

## 4. Storage Design

### 4.1 Why SQLite (not Postgres, not ClickHouse, not DuckDB)

**The tradeoffs:**

| Option | Pros | Cons | Decision |
|---|---|---|---|
| SQLite + FTS5 | Zero deps, embedded, full-text search, WAL mode fast | Not distributed, single writer | ✅ MVP + production for local tool |
| DuckDB | Columnar, fast analytics, Go bindings exist | CGo required, heavier, less mature Go SDK | v2 option for analytics queries |
| ClickHouse | Excellent for log analytics | Requires a server, defeats local-first | No |
| Loki | Purpose-built for logs | Requires running Loki server | No |
| Plain files | Dead simple | No query, no full-text search | No |

SQLite in WAL mode with FTS5 can handle **~50k inserts/sec** on an i5 — more than enough. FTS5 full-text search over millions of log lines is fast (milliseconds for most queries).

### 4.2 Schema

```sql
-- core log storage
CREATE TABLE log_lines (
    id          TEXT PRIMARY KEY,  -- ulid
    ts          INTEGER NOT NULL,  -- unix nano, indexed
    source_kind TEXT NOT NULL,
    source_name TEXT NOT NULL,
    namespace   TEXT,
    container   TEXT,
    level       INTEGER NOT NULL,  -- 0=UNKNOWN 1=DEBUG 2=INFO 3=WARN 4=ERROR 5=FATAL
    message     TEXT NOT NULL,
    raw         TEXT NOT NULL,
    fields      TEXT,              -- JSON blob
    fingerprint TEXT,              -- error group hash
    trace_id    TEXT,
    incident_id TEXT               -- FK to incidents, nullable
);

CREATE INDEX idx_log_ts ON log_lines(ts DESC);
CREATE INDEX idx_log_source ON log_lines(source_name, ts DESC);
CREATE INDEX idx_log_level ON log_lines(level, ts DESC);
CREATE INDEX idx_log_fingerprint ON log_lines(fingerprint);

-- FTS5 full-text search over message + raw
CREATE VIRTUAL TABLE log_fts USING fts5(
    message, raw, source_name,
    content='log_lines', content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 1'
);

-- Incidents
CREATE TABLE incidents (
    id          TEXT PRIMARY KEY,
    started_at  INTEGER NOT NULL,
    resolved_at INTEGER,
    severity    INTEGER NOT NULL,
    title       TEXT,
    root_cause  TEXT,
    sources     TEXT,  -- JSON array
    status      TEXT DEFAULT 'open'
);

-- AI-generated summaries per time window
CREATE TABLE summaries (
    id          TEXT PRIMARY KEY,
    source_name TEXT NOT NULL,
    window_start INTEGER NOT NULL,
    window_end   INTEGER NOT NULL,
    summary     TEXT NOT NULL,
    line_count  INTEGER,
    error_count INTEGER
);

-- Error fingerprints + history (for "have I seen this before?")
CREATE TABLE fingerprints (
    hash        TEXT PRIMARY KEY,
    template    TEXT NOT NULL,
    first_seen  INTEGER NOT NULL,
    last_seen   INTEGER NOT NULL,
    count       INTEGER DEFAULT 1,
    source_name TEXT,
    resolved    BOOLEAN DEFAULT FALSE,
    notes       TEXT   -- AI or human notes on this error pattern
);

-- NL query history + cached results
CREATE TABLE query_history (
    id          TEXT PRIMARY KEY,
    query       TEXT NOT NULL,
    result      TEXT,
    ts          INTEGER NOT NULL
);
```

### 4.3 Ring Buffer (hot path)

In-memory circular buffer, separate from SQLite. Holds last N lines (default 10k, configurable) per source. This is what the TUI reads for live view — zero disk I/O for the streaming log panel. Written to SQLite asynchronously in batches (every 500ms or 500 lines, whichever comes first).

```go
type RingBuffer struct {
    lines   []LogLine
    head    int
    size    int
    mu      sync.RWMutex
    notify  chan struct{}  // signals TUI that new lines arrived
}
```

### 4.4 Data Retention

Configurable TTL-based cleanup:
```yaml
storage:
  db_path: "~/.argus/argus.db"
  retention:
    raw_logs_days: 7      # delete raw log lines after 7 days
    summaries_days: 30    # keep summaries longer
    incidents_days: 90    # keep incidents for 3 months
    fingerprints: forever # never delete error fingerprint history
  max_db_size_gb: 2       # auto-compact when exceeded
```

A background goroutine runs cleanup every 6 hours.

---

## 5. AI Engine

### 5.1 Architecture — What the AI sees and when

```
Pipeline
  │
  ├── Anomaly score ≥ 0.4 ──▶ AI Analysis Queue (buffered chan, size 100)
  │                                    │
  │                          ┌─────────▼──────────┐
  │                          │   AI Dispatcher    │
  │                          │  (rate limited,    │
  │                          │   deduplicated)    │
  │                          └────────┬───────────┘
  │                                   │
  │                    ┌──────────────┼──────────────┐
  │                    ▼              ▼               ▼
  │             Summarizer      RCA Agent       NL Query Agent
  │             (windowed)      (incidents)     (on demand)
  │
  └── User types query ──▶ NL Query Agent
```

### 5.2 AI Provider Abstraction

Supports Ollama (local) and OpenRouter/Anthropic (cloud), switchable per config:

```go
type AIProvider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan string, error)
    Name() string
}

// implementations:
// internal/ai/ollama.go    — http://localhost:11434/api/chat
// internal/ai/openrouter.go — https://openrouter.ai/api/v1/chat/completions
// internal/ai/anthropic.go  — direct Anthropic API
```

Config:
```yaml
ai:
  provider: openrouter       # openrouter | ollama | anthropic
  api_key: "${ARGUS_AI_KEY}" # env var interpolation
  model: "anthropic/claude-3.5-sonnet"
  ollama_url: "http://localhost:11434"
  ollama_model: "llama3.2"   # used when provider=ollama
  max_tokens: 1500
  timeout_s: 45
  rate_limit:
    requests_per_minute: 10  # stay within free tier limits
```

### 5.3 The Three AI Agents

#### Agent 1 — Summarizer
Triggered every 5-minute window per source if activity detected.  
Input: up to 200 log lines from the window (truncated by token budget).  
Output: 2–3 sentence summary + error count + key events.  
Stored in `summaries` table. Used by TUI "Summary" view and by RCA Agent as context.

System prompt:
```
You are an SRE analyzing application logs. Summarize the following log window in 2-3 sentences.
Include: what the service was doing, any errors (with counts), any notable events.
Be technical and concise. Respond in plain text, no markdown.
```

#### Agent 2 — RCA Agent
Triggered when incident detector fires.  
Input: structured context (NOT raw logs — pre-processed):

```
=== INCIDENT CONTEXT ===
Service: payments-svc | Window: last 15 min | Anomaly score: 0.84

=== ERROR FINGERPRINTS (new/spiking) ===
[NEW] "HikariPool connection timeout after <n>ms" — 47 occurrences
[SPIKE 8x] "Failed to acquire DB connection" — 23 occurrences

=== RECENT SUMMARY (5-min window) ===
payments-svc experienced a connection pool exhaustion starting at 14:28.
47 timeout errors logged, 3 OOMKill events detected.

=== K8S EVENTS (if applicable) ===
OOMKilled x3, BackOff x2

=== PROMETHEUS ANOMALIES (if applicable) ===
memory_usage: 3.1x above p95 baseline
```

Output: structured JSON (summary, root_cause, confidence, affected_services, suggested_fixes, runbook_hints).

**Key insight:** The RCA agent never sees raw log lines. It sees pre-aggregated signals. This keeps token usage low and accuracy high.

#### Agent 3 — NL Query Agent
Triggered on demand when user types a query in the TUI.  
Uses a two-step approach:

**Step 1 — Query planner** (fast, cheap prompt):
```
User query: "why was payments-svc slow yesterday afternoon?"
→ Plan: {time_range: "yesterday 12:00-18:00", sources: ["payments-svc"], 
          levels: ["ERROR","WARN"], intent: "performance_rca"}
```

**Step 2 — Execute plan** against SQLite (FTS5 + time range), fetch top-K relevant lines + summaries, feed to LLM for final answer.

This avoids sending the entire log history to the LLM. Only the query-relevant slice is sent.

### 5.4 Historical Incident Learning (RAG-lite)

When a new incident fires, before calling the full RCA agent, Argus checks:

1. Is this fingerprint in the `fingerprints` table with `resolved=true`? → Show previous resolution immediately, skip LLM call.
2. Are there similar past incidents? → Retrieve them from `incidents` table, prepend to RCA prompt as "similar past incidents."

This is RAG without a vector DB — just SQLite FTS5 similarity on incident titles + fingerprint matching. Cheap, fast, no embedding model needed. Good enough for MVP.

For v2: add local embeddings (via Ollama's embedding endpoint or a tiny bundled model) and do proper semantic similarity over `fingerprints.template`.

---

## 6. Daemon vs Embedded Mode

```
argus                    # starts TUI, embedded mode (collectors run in-process)
argus daemon start       # starts background daemon, writes to ~/.argus/argus.sock
argus daemon stop
argus daemon status
argus                    # TUI auto-detects daemon, connects via socket instead
```

**Embedded mode:** All collectors, pipeline, storage, and AI run in the same process as the TUI. Simple, no IPC. Fine for casual use.

**Daemon mode:** Collectors run persistently in the background. TUI is a thin client connecting via Unix domain socket. Logs are collected even when TUI is closed. Better for "always-on" monitoring.

**IPC protocol:** Simple length-prefixed JSON frames over Unix socket. Not gRPC — too heavy for MVP. Not HTTP — adds latency. Raw socket with a small frame protocol is 50 lines of Go.

```go
type Frame struct {
    Type    string          // "subscribe_logs" | "query" | "incident_update" | "ping"
    Payload json.RawMessage
}
```

TUI subscribes to a stream of `LogLine` events filtered by source/level. Daemon sends matching frames. Backpressure via bounded channel — if TUI is slow, daemon drops frames (log tailing is lossy by design, storage is not).

---

## 7. TUI Design (Bubble Tea)

### 7.1 View Architecture

```
RootModel
├── Header (cluster, daemon status, active sources count, time)
├── TabBar  [1]Logs [2]Analyze [3]Top [4]Events [5]Query [6]Incidents [7]Chat
├── ActiveView (one of below)
│   ├── LogsView      — streaming log viewport, filter, source picker
│   ├── AnalyzeView   — RCA result panels, confidence, fixes
│   ├── TopView       — live metrics table per source
│   ├── EventsView    — K8s events or process events table
│   ├── QueryView     — NL query input + AI answer + source log refs
│   ├── IncidentsView — incident list, timeline, drill-down
│   └── ChatView      — free-form chat with AI about your logs
└── StatusBar (keys hint, last AI action, error count)
```

### 7.2 LogsView — the most important view

```
┌─ Argus ──────────────────── minikube·prod ── 3 sources ── 14:32:01 ─┐
│  [1]Logs [2]Analyze [3]Top [4]Events [5]Query [6]Incidents [7]Chat  │
├─────────────────────────────────────────────────────────────────────── │
│ Source: [ALL ▼]  Level: [ALL ▼]  [/] Filter  [f] Follow  [p] Pause  │
├─────────────────────────────────────────────────────────────────────── │
│ 14:32:01 payments-svc  INFO  Server started on :8080                 │
│ 14:32:02 api-gateway   INFO  Upstream healthy                        │
│ 14:32:03 payments-svc  ERROR HikariPool timeout after 30000ms  ← ─┐ │
│ 14:32:03 payments-svc  ERROR Failed to acquire DB connection      │ │
│ 14:32:04 payments-svc  ERROR HikariPool timeout after 30000ms     │ │
│                                                          grouped(3)┘ │
│ ▶ 14:32:05 [INCIDENT DETECTED] payments-svc — score 0.84            │
│ 14:32:06 payments-svc  WARN  Retry attempt 1/3                      │
├─────────────────────────────────────────────────────────────────────── │
│ 847 lines · 3 errors · [a] analyze · [i] incident · [s] save        │
└─────────────────────────────────────────────────────────────────────────┘
```

Key UX decisions:
- **Error grouping:** consecutive identical fingerprints collapse to `error message (3x)` — same as k9s event grouping
- **Follow mode:** auto-scrolls to bottom. `[f]` to toggle. Pauses on manual scroll.
- **Source picker:** dropdown over source list. `[space]` to toggle sources.
- **`[a]` hotkey:** triggers `analyze` on selected time range (or last 15 min if nothing selected)
- **Incident markers:** inline markers in log stream when incident is declared

### 7.3 QueryView

```
┌─ Query ─────────────────────────────────────────────────────────────┐
│ Ask anything about your logs...                                     │
│ ┌─────────────────────────────────────────────────────────────────┐ │
│ │ why was payments-svc failing yesterday afternoon?_              │ │
│ └─────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│ ● Argus                                                             │
│ Between 14:28 and 14:45 yesterday, payments-svc experienced         │
│ connection pool exhaustion. 47 HikariCP timeout errors occurred,    │
│ causing 3 OOMKill events. The likely cause is unclosed database     │
│ connections in the order processing loop.                           │
│                                                                     │
│ Referenced log lines: [14:28:11] [14:28:12] [14:29:03] ← clickable │
│ Related incident: INC-0042 (resolved) ← click to open              │
│                                                                     │
│ ▶ _                                                                 │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 8. Plugin System

Plugins are Go shared libraries (`.so`) loaded at startup via Go's `plugin` package, OR — better for portability — external processes communicating via stdin/stdout JSON (like LSP, like Terraform providers).

**MVP: no plugin system.** Hard-code the 8 collectors. Plugin system is v2.

**v2 plugin contract:**
```go
// Every plugin is a subprocess that reads JSON from stdin, writes JSON to stdout
type PluginManifest struct {
    Name        string
    Version     string
    SourceKind  string       // "splunk" | "datadog" | "loki" | etc.
    Config      []ConfigField
}
```

This avoids CGo and dynamic linking headaches. Plugin as subprocess is battle-tested (Terraform, Neovim, Helix all do this).

---

## 9. Complete Project Structure

```
argus/
├── cmd/
│   ├── main.go              ← binary entrypoint
│   ├── root.go              ← Cobra root, global flags, config bootstrap
│   ├── watch.go             ← argus watch --cmd / --docker / --k8s / --file
│   ├── logs.go              ← argus logs (TUI logs view standalone)
│   ├── analyze.go           ← argus analyze (one-shot RCA, no TUI)
│   ├── query.go             ← argus query "why did X fail" (one-shot NL)
│   ├── daemon.go            ← argus daemon start|stop|status
│   └── tui.go               ← argus (no subcommand) → full TUI
│
├── internal/
│   ├── collector/
│   │   ├── collector.go     ← Collector interface, LogLine, SourceMeta types
│   │   ├── fanin.go         ← fan-in multiplexer goroutine
│   │   ├── process.go       ← exec.Cmd stdout/stderr pipe
│   │   ├── docker.go        ← Docker Engine API log streaming
│   │   ├── k8s.go           ← client-go PodLogs
│   │   ├── file.go          ← fsnotify tail
│   │   ├── journal.go       ← journalctl subprocess
│   │   ├── syslog.go        ← embedded syslog server
│   │   ├── stdin.go         ← os.Stdin pipe mode
│   │   ├── cloudwatch.go    ← AWS CloudWatch polling
│   │   └── autodiscover.go  ← Docker + K8s auto-discovery
│   │
│   ├── pipeline/
│   │   ├── pipeline.go      ← stage orchestrator
│   │   ├── parser.go        ← Parser interface + registry
│   │   ├── parsers/
│   │   │   ├── json.go
│   │   │   ├── logfmt.go
│   │   │   ├── java.go
│   │   │   ├── nginx.go
│   │   │   ├── syslog.go
│   │   │   └── fallback.go
│   │   ├── enrich.go        ← fingerprint, template extraction, stack trace join
│   │   ├── filter.go        ← configurable drop/sample rules
│   │   ├── anomaly.go       ← sliding window scorer
│   │   ├── aggregator.go    ← tumbling window aggregator
│   │   └── incident.go      ← incident declaration logic
│   │
│   ├── storage/
│   │   ├── storage.go       ← Storage interface
│   │   ├── sqlite.go        ← SQLite + FTS5 implementation
│   │   ├── ringbuffer.go    ← in-memory ring buffer
│   │   ├── schema.go        ← CREATE TABLE statements, migrations
│   │   └── retention.go     ← TTL cleanup background job
│   │
│   ├── ai/
│   │   ├── provider.go      ← AIProvider interface
│   │   ├── openrouter.go
│   │   ├── anthropic.go
│   │   ├── ollama.go
│   │   ├── dispatcher.go    ← rate-limited AI queue
│   │   ├── summarizer.go    ← windowed summary agent
│   │   ├── rca.go           ← RCA agent, context builder
│   │   ├── nlquery.go       ← NL query planner + executor
│   │   ├── prompts.go       ← all prompt templates
│   │   └── history.go       ← fingerprint + incident similarity lookup
│   │
│   ├── daemon/
│   │   ├── daemon.go        ← daemon lifecycle, PID file
│   │   ├── server.go        ← Unix socket server, frame protocol
│   │   └── client.go        ← TUI-side socket client
│   │
│   ├── tui/
│   │   ├── root.go          ← RootModel, tab routing
│   │   ├── logs.go          ← LogsView
│   │   ├── analyze.go       ← AnalyzeView
│   │   ├── top.go           ← TopView
│   │   ├── events.go        ← EventsView
│   │   ├── query.go         ← QueryView
│   │   ├── incidents.go     ← IncidentsView
│   │   ├── chat.go          ← ChatView
│   │   ├── components/
│   │   │   ├── table.go     ← reusable sortable table
│   │   │   ├── viewport.go  ← log viewport with follow mode
│   │   │   ├── spinner.go   ← loading indicator
│   │   │   ├── badge.go     ← severity badge (P1/P2/P3)
│   │   │   └── input.go     ← text input with history
│   │   └── styles.go        ← Lipgloss theme
│   │
│   └── config/
│       ├── config.go        ← Config struct, Viper loader
│       └── defaults.go      ← sensible defaults
│
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml         ← cross-platform release
└── README.md
```

---

## 10. Go Dependencies (go.mod)

```
github.com/spf13/cobra                 ← CLI
github.com/spf13/viper                 ← config
github.com/charmbracelet/bubbletea     ← TUI
github.com/charmbracelet/lipgloss      ← TUI styles
github.com/charmbracelet/bubbles       ← TUI components (viewport, textinput, table)
github.com/fsnotify/fsnotify           ← file watching
github.com/docker/docker/client        ← Docker API
k8s.io/client-go                       ← Kubernetes
github.com/mattn/go-sqlite3            ← SQLite (CGo — unavoidable for FTS5)
github.com/oklog/ulid/v2               ← sortable unique IDs
github.com/cespare/xxhash/v2           ← fast fingerprint hashing
github.com/prometheus/client_golang    ← Prometheus HTTP API
github.com/aws/aws-sdk-go-v2           ← CloudWatch (optional, build tag)
```

**CGo note:** `go-sqlite3` requires CGo. This means cross-compilation needs a C cross-compiler. For release: use `zig cc` as the C compiler in GoReleaser — it makes cross-compilation trivial. Alternative: `modernc.org/sqlite` is a pure Go SQLite port (no CGo) — loses ~20% performance but gains zero-CGo build. Use `modernc.org/sqlite` for MVP simplicity.

---

## 11. Fault Tolerance & Reliability

| Failure | Handling |
|---|---|
| Collector goroutine panics | `recover()` in each goroutine, restart with backoff, log to stderr |
| SQLite write failure | Ring buffer continues; retry writes; alert in status bar |
| LLM API timeout/error | AI dispatcher retries 3x with exponential backoff; skip if still failing; mark incident as "AI unavailable" |
| Docker socket unavailable | Docker collector logs warning, skips — other collectors unaffected |
| K8s cluster unreachable | K8s collector retries with backoff; pipeline continues without it |
| Unix socket disconnect | TUI falls back to embedded mode automatically |
| Process collector child dies | Emit synthetic "FATAL: process exited with code N" log line; optionally restart |
| Ring buffer full | Drop oldest lines (not newest) — explicit tradeoff: recent > historical for live view |
| SQLite DB corrupted | Rename to `.bak`, create fresh DB, log warning — don't crash the tool |

---

## 12. Performance Targets (i5, 8GB RAM)

| Metric | Target |
|---|---|
| Argus binary RAM (idle) | < 50MB |
| Argus RAM at 1k lines/sec | < 150MB |
| Log ingestion throughput | ≥ 50k lines/sec (pipeline, pre-storage) |
| SQLite write throughput | ≥ 5k lines/sec (batched, WAL mode) |
| TUI re-render latency | < 16ms (60fps) |
| FTS5 query latency | < 100ms for 1M rows |
| AI RCA latency | 3–15s (network-bound for cloud, 5–30s for Ollama) |
| Binary size | < 25MB (< 15MB without cloud collectors) |
| Startup time | < 200ms to first TUI frame |

---

## 13. Security & Privacy

- **API keys:** stored in `~/.argus/config.yaml` with `0600` permissions. Never logged. Env var override always available.
- **Log data:** stays in `~/.argus/argus.db`. User explicitly controls what goes to cloud LLM via provider config.
- **Sensitive log scrubbing (v2):** configurable regex rules to redact patterns before sending to AI: `scrub: ["password=\S+", "Bearer \S+", "\d{16}"]`
- **Daemon socket:** `~/.argus/argus.sock` with `0600` permissions — owner-only access.
- **No telemetry:** Argus never phones home. No analytics, no crash reporting unless user opts in (v2).

---

## 14. MVP Build Phases (6–8 weeks)

### Phase 1 — Week 1–2: Foundation
- [ ] Project scaffold, `go.mod`, Makefile
- [ ] Config system (Viper + `~/.argus/config.yaml`)
- [ ] `Collector` interface + `LogLine` type
- [ ] Process collector (`argus watch --cmd`)
- [ ] File collector (`argus watch --file`)
- [ ] Docker collector
- [ ] Fan-in multiplexer
- [ ] Basic parser stage (JSON + fallback)
- [ ] Ring buffer
- [ ] `argus logs` command — streaming to stdout (no TUI yet)

**End of Phase 1:** `argus watch --cmd "npm run dev"` streams colored logs to terminal.

### Phase 2 — Week 3: Storage + Pipeline
- [ ] SQLite schema + migrations
- [ ] FTS5 virtual table
- [ ] Enrichment stage (fingerprinting, template extraction)
- [ ] Filter + sampling stage
- [ ] Anomaly pre-scorer (sliding window)
- [ ] Batch writer to SQLite
- [ ] Retention cleanup job

**End of Phase 2:** Logs persisted, queryable with raw SQL.

### Phase 3 — Week 4: AI Engine
- [ ] AIProvider interface + OpenRouter implementation
- [ ] Ollama implementation
- [ ] AI dispatcher (rate-limited queue)
- [ ] RCA agent + prompt design
- [ ] Summarizer agent
- [ ] NL query agent (query planner + executor)
- [ ] Fingerprint history lookup
- [ ] `argus analyze` one-shot command (no TUI)
- [ ] `argus query "..."` one-shot command

**End of Phase 3:** `argus analyze --source payments-svc` prints RCA to stdout.

### Phase 4 — Week 5–6: Kubernetes + TUI
- [ ] K8s collector (client-go)
- [ ] Incident detector
- [ ] Bubble Tea RootModel + styles
- [ ] LogsView (viewport, follow, filter, source picker)
- [ ] AnalyzeView (streaming RCA)
- [ ] TopView (metrics table)
- [ ] IncidentsView
- [ ] QueryView (NL query input + AI answer)
- [ ] Wire everything together

**End of Phase 4:** Full TUI working, all views navigable.

### Phase 5 — Week 7–8: Polish + Ship
- [ ] Daemon mode (Unix socket, server + client)
- [ ] Journald collector
- [ ] Prometheus integration (metrics queries in TopView)
- [ ] Auto-discovery (Docker + K8s)
- [ ] Error handling hardening
- [ ] README + demo GIF (vhs or asciinema)
- [ ] GitHub Actions CI + GoReleaser
- [ ] `argus init` first-run wizard

**End of Phase 5:** Public release, `go install`, downloadable binary.

---

## 15. What NOT to Build in MVP

Explicitly out of scope to stay on timeline:

- Plugin system
- Cloud log sources (CloudWatch, GCP) — add post-MVP
- RAG with embeddings — SQLite FTS5 is enough
- Metrics collection (only reading from Prometheus, not collecting)
- Alert routing (PagerDuty, Slack, Gmail)
- Multi-user / team features
- Web UI
- Windows native (WSL2 works)
- Helm chart / Docker image for Argus itself

---

## 16. This vs Existing Tools

| Tool | Gap Argus fills |
|---|---|
| k9s | K8s-only, no AI, no log analysis, no process/file/docker |
| Stern | K8s log tail only, no analysis, no TUI panels |
| Loki + Grafana | Requires running servers, browser, 1GB+ RAM |
| Datadog/Honeycomb | Cloud SaaS, expensive, data leaves machine |
| Vector | Excellent pipeline, but no TUI, no AI, no incident detection |
| OpenTelemetry Collector | Infrastructure piece only, no user-facing intelligence |

Argus's unique position: **the only single-binary, terminal-native, AI-powered log intelligence tool that works across local processes, Docker, and Kubernetes simultaneously.**
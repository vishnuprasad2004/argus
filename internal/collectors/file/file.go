package file

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/vishnuprasad2004/argus/agents"
)

type FileCollector struct {
    path string
}

type FetchOptions struct {
    TailLines int  // 0 = full file
    FullFile  bool
}

func NewFileCollector(path string) (*FileCollector, error) {
    // expand ~ to home dir
    if strings.HasPrefix(path, "~/") {
        home, err := os.UserHomeDir()
        if err != nil {
            return nil, err
        }
        path = filepath.Join(home, path[2:])
    }

    // check file exists
    info, err := os.Stat(path)
    if err != nil {
        return nil, fmt.Errorf("file not found: %s", path)
    }
    if info.IsDir() {
        return nil, fmt.Errorf("path is a directory, not a file: %s", path)
    }

    // warn if over 10MB
    if info.Size() > 10*1024*1024 {
        fmt.Printf("⚠ File is %.1fMB — consider using tail mode\n",
            float64(info.Size())/1024/1024)
    }

    return &FileCollector{path: path}, nil
}

func (f *FileCollector) Name() string { return filepath.Base(f.path) }
func (f *FileCollector) Path() string { return f.path }






func (f *FileCollector) FetchLogs(opts FetchOptions) ([]agents.LogEntry, error) {
    file, err := os.Open(f.path)
    if err != nil {
        return nil, fmt.Errorf("cannot open file: %w", err)
    }
    defer file.Close()

    ext := strings.ToLower(filepath.Ext(f.path))

    var lines []string
    scanner := bufio.NewScanner(file)

    // increase scanner buffer for long lines (default 64KB is too small for JSON logs)
    buf := make([]byte, 1024*1024) // 1MB buffer
    scanner.Buffer(buf, 1024*1024)

    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("read error: %w", err)
    }

    // apply tail if not full file
    if !opts.FullFile && opts.TailLines > 0 && len(lines) > opts.TailLines {
        lines = lines[len(lines)-opts.TailLines:]
    }

    // parse based on extension
    switch ext {
    case ".json":
        return parseJSONLines(lines, f.Name())
    default:
        // .log, .txt, anything else — plain text parser
        return parsePlainLines(lines, f.Name())
    }
}








// parsePlainLines handles .log and .txt files
// tries common log formats, falls back to raw line
func parsePlainLines(lines []string, source string) ([]agents.LogEntry, error) {
    var entries []agents.LogEntry

    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue // skip blank lines
        }
        entries = append(entries, parsePlainLine(line, source))
    }
    return entries, nil
}

func parsePlainLine(raw, source string) agents.LogEntry {
    entry := agents.LogEntry{
        Source:    source,
        Timestamp: time.Now(),
        Level:     "INFO",
        Message:   raw,
        Metadata:  map[string]string{"type": "file"},
    }

    upper := strings.ToUpper(raw)

    // ── try common timestamp formats ──────────────────────────────────
    formats := []string{
        "2006-01-02T15:04:05.999999999Z07:00", // RFC3339Nano
        "2006-01-02T15:04:05Z07:00",            // RFC3339
        "2006-01-02 15:04:05",                  // common plain format
        "2006/01/02 15:04:05",                  // nginx style
        "02/Jan/2006:15:04:05 -0700",           // Apache combined log
    }

    for _, format := range formats {
        // most log lines start with timestamp — try first 35 chars
        sample := raw
        if len(raw) > 35 {
            sample = raw[:35]
        }
        // try to parse first word as timestamp
        parts := strings.SplitN(raw, " ", 2)
        if len(parts) == 2 {
            if t, err := time.Parse(format, strings.TrimSpace(parts[0])); err == nil {
                entry.Timestamp = t
                entry.Message = parts[1]
                break
            }
        }
        _ = sample
    }

    // ── detect level ──────────────────────────────────────────────────
    switch {
    case strings.Contains(upper, "FATAL") ||
         strings.Contains(upper, "PANIC") ||
         strings.Contains(upper, "CRITICAL"):
        entry.Level = "ERROR"
    case strings.Contains(upper, "ERROR") ||
         strings.Contains(upper, "ERR ") ||
         strings.Contains(upper, "[ERROR]"):
        entry.Level = "ERROR"
    case strings.Contains(upper, "WARN") ||
         strings.Contains(upper, "[WARN]") ||
         strings.Contains(upper, "WARNING"):
        entry.Level = "WARN"
    case strings.Contains(upper, "DEBUG") ||
         strings.Contains(upper, "[DEBUG]"):
        entry.Level = "DEBUG"
    }

    // ── stack trace lines stay as ERROR ───────────────────────────────
    if strings.HasPrefix(strings.TrimSpace(raw), "at ") ||
       strings.HasPrefix(strings.TrimSpace(raw), "\tat ") {
        entry.Level = "ERROR"
        entry.Metadata["is_stack_trace"] = "true"
    }

    return entry
}




// parseJSONLines handles newline-delimited JSON (one object per line)
// common format for structured logging (Winston, Pino, Zap, Logrus JSON mode)
func parseJSONLines(lines []string, source string) ([]agents.LogEntry, error) {
    var entries []agents.LogEntry

    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        var raw map[string]any
        if err := json.Unmarshal([]byte(line), &raw); err != nil {
            // not valid JSON — parse as plain text
            entries = append(entries, parsePlainLine(line, source))
            continue
        }

        entry := agents.LogEntry{
            Source:    source,
            Timestamp: time.Now(),
            Level:     "INFO",
            Metadata:  map[string]string{"type": "json"},
        }

        // ── extract message — try common field names ──────────────────
        // different loggers use different field names
        for _, key := range []string{"message", "msg", "text", "body", "log"} {
            if v, ok := raw[key].(string); ok {
                entry.Message = v
                break
            }
        }
        if entry.Message == "" {
            entry.Message = line // fallback to raw
        }

        // ── extract level ─────────────────────────────────────────────
        for _, key := range []string{"level", "severity", "lvl", "log_level"} {
            if v, ok := raw[key].(string); ok {
                entry.Level = normalizeLevel(v)
                break
            }
        }

        // ── extract timestamp ─────────────────────────────────────────
        for _, key := range []string{"time", "timestamp", "ts", "@timestamp", "datetime"} {
            if v, ok := raw[key].(string); ok {
                formats := []string{
                    time.RFC3339Nano,
                    time.RFC3339,
                    "2006-01-02 15:04:05",
                }
                for _, f := range formats {
                    if t, err := time.Parse(f, v); err == nil {
                        entry.Timestamp = t
                        break
                    }
                }
                break
            }
            // unix timestamp as float
            if v, ok := raw[key].(float64); ok {
                entry.Timestamp = time.Unix(int64(v), 0)
                break
            }
        }

        entries = append(entries, entry)
    }

    return entries, nil
}

// normalizeLevel maps various level strings to ERROR/WARN/INFO/DEBUG
func normalizeLevel(raw string) string {
    switch strings.ToUpper(strings.TrimSpace(raw)) {
    case "ERROR", "ERR", "FATAL", "CRITICAL", "PANIC", "EMERG", "ALERT":
        return "ERROR"
    case "WARN", "WARNING":
        return "WARN"
    case "DEBUG", "TRACE", "VERBOSE":
        return "DEBUG"
    default:
        return "INFO"
    }
}
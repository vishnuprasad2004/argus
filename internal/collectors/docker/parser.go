package docker

import (
	"github.com/vishnuprasad2004/argus/agents"
	"strings"
	"time"
)

// parseDockerLine turns one raw docker log line into a clean LogEntry
// raw format from docker: "2024-01-15T10:23:45.123456789Z some message here"
func parseDockerLine(raw string, target ContainerTarget) agents.LogEntry {
	entry := agents.LogEntry{
		Source:    target.Name,
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   raw, // default: use whole line as message
		Metadata: map[string]string{
			"container_id": target.ID,
			"image":        target.Image,
		},
	}

	// Strategy 1: Docker timestamp prefix
	// format: "2024-01-15T10:23:45.123Z rest of message"
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) == 2 {
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err == nil {
			entry.Timestamp = t
			entry.Message = parts[1]
			raw = parts[1] // update raw so level detection below uses clean message
		}
	}

	// Strategy 2: detect level from common unstructured patterns
	upper := strings.ToUpper(raw)
	switch {
	case strings.Contains(upper, "FATAL") ||
		strings.Contains(upper, "PANIC") ||
		strings.Contains(upper, "NPM ERROR") || // npm error lines
		strings.Contains(upper, "UNHANDLEDREJECTION"):
		entry.Level = "ERROR"

	case strings.Contains(upper, "ERROR") ||
		strings.Contains(upper, "ERR!") || // node style
		strings.Contains(upper, "EXCEPTION") ||
		strings.Contains(upper, "SIGNAL SIGTERM"): // process killed
		entry.Level = "ERROR"

	case strings.Contains(upper, "WARN") ||
		strings.Contains(upper, "WARNING") ||
		strings.Contains(upper, "DEPRECATED"):
		entry.Level = "WARN"

	case strings.Contains(upper, "DEBUG") ||
		strings.Contains(upper, "VERBOSE"):
		entry.Level = "DEBUG"
	}

	// Strategy 3: stack trace lines — keep as ERROR, group them
	// lines starting with "at " are JS/Java stack frames
	if strings.HasPrefix(strings.TrimSpace(raw), "at ") {
		entry.Level = "ERROR" // stack trace = belongs to an error
		entry.Metadata["is_stack_trace"] = "true"
	}

	return entry
}
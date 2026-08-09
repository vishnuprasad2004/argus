package types

import "time"

type LogEntry struct {
	Timestamp time.Time
	Level     string // ERROR, WARN, INFO, DEBUG
	Source    string // pod name / container id / process name
	Message   string
	Metadata  map[string]string // namespace, image, pid etc
}
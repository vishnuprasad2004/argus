package pipeline

import (
	"regexp"
	"github.com/vishnuprasad2004/argus/internal/types"
)

var scrubPatterns = []struct {
	name    string
	pattern *regexp.Regexp
	replace string
}{
	// secrets
	{"api_key", regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*\S+`), "api_key=[REDACTED]"},
	{"password", regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`), "password=[REDACTED]"},
	{"token", regexp.MustCompile(`(?i)(token|bearer|jwt)\s*[:=]?\s*[A-Za-z0-9\-._]+`), "token=[REDACTED]"},
	{"secret", regexp.MustCompile(`(?i)(secret|private[_-]?key)\s*[:=]\s*\S+`), "secret=[REDACTED]"},

	// PII
	{"email", regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), "[EMAIL]"},
	{"phone", regexp.MustCompile(`\b(\+\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`), "[PHONE]"},
	{"credit_card", regexp.MustCompile(`\b\d{4}[\s\-]?\d{4}[\s\-]?\d{4}[\s\-]?\d{4}\b`), "[CARD]"},
	{"ssn", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), "[SSN]"},
	{"ip", regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), "[IP]"},

	// auth headers
	{"auth_header", regexp.MustCompile(`(?i)Authorization:\s*\S+\s+\S+`), "Authorization: [REDACTED]"},
	{"aws_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[AWS_KEY]"},
}

func Scrub(logs []types.LogEntry) []types.LogEntry {
	scrubbed := make([]types.LogEntry, len(logs))
	for i, entry := range logs {
		scrubbed[i] = entry
		for _, p := range scrubPatterns {
			scrubbed[i].Message = p.pattern.ReplaceAllString(scrubbed[i].Message, p.replace)
		}
	}
	return scrubbed
}
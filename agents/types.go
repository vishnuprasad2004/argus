package agents

import (
	"context"
	"time"
	"github.com/tmc/langchaingo/llms/googleai"
)

type LogEntry struct {
	Timestamp time.Time
	Level     string // ERROR, WARN, INFO, DEBUG
	Source    string // pod name / container id / process name
	Message   string
	Metadata  map[string]string // namespace, image, pid etc
}

type AgentInput struct {
	Query   string
	Logs    []LogEntry
	Context string // rolling summary from previous window
}

type AgentOutput struct {
	Result     string
	Structured map[string]any // metrics data for TUI charts
	TokensUsed int
}

type Agent interface {
	Name() string
	Description() string // orchestrator reads this to pick which agent to call
	Run(ctx context.Context, input AgentInput) (AgentOutput, error)
}

type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

type AgentTool struct {
  agent Agent
	client *googleai.GoogleAI
}

type EventType string

const (
    EventToolCall EventType = "tool_call"
    EventAnswer   EventType = "answer"
    EventWarning  EventType = "warning"
    EventError    EventType = "error"
)

type AgentEvent struct {
  Type    EventType // "thinking" | "tool_call" | "tool_result" | "answer"
  Tool    string
  Message string
}

// TUI subscribes and renders:
// ◆ Analyzing logs...
// ⚙ Called: log_analysis_agent
// ⚙ Called: rca_agent  
// ✓ Root cause identified


func (t AgentTool) Name() string        { return t.agent.Name() }
func (t AgentTool) Description() string { return t.agent.Description() }
func (t AgentTool) Call(ctx context.Context, input string) (string, error) {
	out, err := t.agent.Run(ctx, AgentInput{Query: input})
	if err != nil {
			return "", err
	}
	return out.Result, nil
}
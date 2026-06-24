package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

type Orchestrator struct {
	client     *googleai.GoogleAI
	logAgent   *LogAnalysisAgent
	rcaAgent   *RCAAgent
	statsAgent *StatsAgent
	Events     chan AgentEvent // TUI listens to this
}



func NewOrchestrator(client *googleai.GoogleAI) *Orchestrator {
	return &Orchestrator{
		client:     client,
		logAgent:   &LogAnalysisAgent{AgentTool{client: client}},
		rcaAgent:   &RCAAgent{client: client},
		statsAgent: &StatsAgent{},   // no LLM needed
		Events:     make(chan AgentEvent, 10),
  }
}

func (o *Orchestrator) Run(ctx context.Context, query string, logs []LogEntry) (string, error) {
	o.Events <- AgentEvent{Type: EventToolCall, Tool: "log_analysis", Message: "⚙ Analyzing logs..."}
	logResult, err := o.logAgent.Run(ctx, AgentInput{Logs: logs, Query: query})
	if err != nil {
    return "", fmt.Errorf("orchestrator: log analysis failed: %w", err)
  }

	routingPrompt := fmt.Sprintf(`
		You are a router. Reply with ONE word only.

		User asked: "%s"
		Log analysis: %s

		Reply RCA if the user wants diagnosis/root cause/why something failed.
		Reply STATS if the user wants counts/metrics/numbers/summary.
		ONE WORD ONLY.`, query, logResult.Result)

	decision, err := llms.GenerateFromSinglePrompt(ctx, o.client, routingPrompt)
    if err != nil {
        return "", fmt.Errorf("orchestrator: routing failed: %w", err)
    }
  decision = strings.TrimSpace(strings.ToUpper(decision))

	var finalResult string

	switch decision {

		// RCA is for root cause analysis, diagnosis, and suggested fix
		case "RCA":
			o.Events <- AgentEvent{Type: EventToolCall, Tool: "rca_agent", Message: "⚙ Performing root cause analysis..."}
			rcaResult, err := o.rcaAgent.Run(ctx, AgentInput{Query: query, Context: logResult.Result})
			if err != nil {
				return "", fmt.Errorf("orchestrator: rca_agent failed: %w", err)
			}
			finalResult = rcaResult.Result


		// STATS is for metrics summary, counts, error rate, etc.
		case "STATS":
			o.Events <- AgentEvent{Type: EventToolCall, Tool: "stats_agent", Message: "⚙ Computing metrics summary..."}
			statsResult, err := o.statsAgent.Run(ctx, AgentInput{Logs: logs})
			if err != nil {
				return "", fmt.Errorf("orchestrator: stats_agent failed: %w", err)
			}
			finalResult = statsResult.Result


		default:
			o.Events <- AgentEvent{Type: EventWarning, Tool: "orchestrator", Message: fmt.Sprintf("unexpected decision '%s', defaulting to RCA", decision)}
			rcaResult, err := o.rcaAgent.Run(ctx, AgentInput{
					Query:   query,
					Context: logResult.Result,
			})
			if err != nil {
					return "", fmt.Errorf("orchestrator: fallback rca failed: %w", err)
			}
			finalResult = rcaResult.Result
	}

	o.Events <- AgentEvent{Type: EventAnswer, Tool: "orchestrator", Message: finalResult}
	return finalResult, nil
}
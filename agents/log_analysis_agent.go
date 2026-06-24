package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

type LogAnalysisAgent struct {
	AgentTool
}

var log_analysis_prompt string = `
You are an SRE log analysis expert. Analyze these logs.
		Extract:
		1. All ERROR/FATAL entries with timestamps
		2. Repeated patterns (same error 3+ times)
		3. Any anomalies or warning spikes

		Be concise. No markdown. Plain text only.

		LOGS:
`
func (agent *LogAnalysisAgent) Name() string { return "log_analysis" }

func (agent *LogAnalysisAgent) Description() string {
	return "Analyzes log entries to extract errors, stack traces, repeated failures, and anomalies. Input: log window as text."
}

func (agent *LogAnalysisAgent) Run(ctx context.Context, in AgentInput) (AgentOutput, error) {

	var logLines strings.Builder 

	for _, l := range in.Logs {
    fmt.Fprintf(&logLines, "[%s] %s %s\n",
		l.Level, l.Timestamp.Format("15:04:05"), l.Message)
  }
	
	prompt := fmt.Sprintf(log_analysis_prompt + "\n", logLines.String())
	result, err := llms.GenerateFromSinglePrompt(ctx, agent.client, prompt)

	if err != nil {
		return AgentOutput{}, err
	}
	return AgentOutput{Result: result}, nil

}
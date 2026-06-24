package agents

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

type RCAAgent struct {
    client *googleai.GoogleAI
}

func (a *RCAAgent) Name() string { return "rca_agent" }

func (a *RCAAgent) Description() string {
    return "Given log analysis output, identifies root cause and suggests a fix."
}

func (a *RCAAgent) Run(ctx context.Context, in AgentInput) (AgentOutput, error) {
    // in.Context = output from LogAnalysisAgent
    // in.Query   = original user question
    prompt := fmt.Sprintf(`
			You are a senior SRE performing root cause analysis.

			The user asked: "%s"

			Log analysis findings:
			%s

			Based on the above, provide:
			1. Root cause (one sentence, be specific)
			2. Evidence from the logs that supports it
			3. One concrete fix (command or config change)

			No markdown. Plain text. Be direct.
		`, in.Query, in.Context)

    result, err := llms.GenerateFromSinglePrompt(ctx, a.client, prompt)
    if err != nil {
        return AgentOutput{}, fmt.Errorf("rca_agent: %w", err)
    }
    return AgentOutput{Result: result}, nil
}
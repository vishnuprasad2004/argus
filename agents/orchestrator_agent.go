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
	history    []ConversationTurn
}

type ConversationTurn struct {
	Role    string // "user" or "assistant"
	Content string
}

func NewOrchestrator(client *googleai.GoogleAI) *Orchestrator {
	return &Orchestrator{
		client:     client,
		logAgent:   &LogAnalysisAgent{AgentTool{client: client}},
		rcaAgent:   &RCAAgent{client: client},
		statsAgent: &StatsAgent{}, // no LLM needed
		Events:     make(chan AgentEvent, 10),
	}
}

func (o *Orchestrator) Run(ctx context.Context, query string, logs []LogEntry) (string, error) {

	// add user message to history
	o.history = append(o.history, ConversationTurn{
		Role:    "user",
		Content: query,
	})

	// build history string
	var historyStr strings.Builder
	for _, turn := range o.history {
		fmt.Fprintf(&historyStr, "%s: %s\n", turn.Role, turn.Content)
	}

	// build log summary — only send if logs exist
	// don't resend full logs every turn, too many tokens
	logContext := ""
	if len(logs) > 0 {
		logContext = fmt.Sprintf("\n\nAvailable log data: %d entries, last entry at %s",
			len(logs), logs[len(logs)-1].Timestamp.Format("15:04:05"))
	}

	// THE key prompt — orch is the agent, tools are just capabilities
	systemPrompt := fmt.Sprintf(`
You are Argus, an expert SRE AI assistant embedded in a terminal tool.
You are having a conversation with a developer about their running service.

You have access to these tools (use them ONLY when the user's question requires it):
- log_analysis: reads and extracts patterns/errors from logs. Use when user asks about errors, failures, anomalies.
- stats: counts log levels, computes error rate. Use when user asks for numbers/metrics/counts.
- rca: identifies root cause from log analysis. Use when user asks WHY something failed.

Rules:
- If the user is just chatting ("ok thanks", "got it", "cool") — reply conversationally, NO tools.
- If the user asks a follow-up on your previous answer — answer from context, NO tools unless new info needed.
- Only call a tool if the answer genuinely requires reading the logs.
- Be concise. No markdown. You are in a terminal.
- You remember the full conversation below.

Conversation history:
%s
%s

When you need a tool, reply EXACTLY in this format and nothing else:
TOOL: tool_name
REASON: why you need it

Otherwise just reply normally.
`, historyStr.String(), logContext)

	// single LLM call — orch decides everything
	response, err := llms.GenerateFromSinglePrompt(ctx, o.client,
		systemPrompt+"\n\nuser: "+query)
	if err != nil {
		return "", fmt.Errorf("orchestrator: %w", err)
	}

	// check if orch wants to call a tool
	if strings.HasPrefix(strings.TrimSpace(response), "TOOL:") {
		return o.handleToolCall(ctx, response, query, logs, historyStr.String())
	}

	// no tool needed — orch answered directly
	o.history = append(o.history, ConversationTurn{Role: "assistant", Content: response})
	o.Events <- AgentEvent{Type: EventAnswer, Tool: "orchestrator", Message: response}
	return response, nil
}




func (o *Orchestrator) handleToolCall(ctx context.Context, toolResponse, originalQuery string, logs []LogEntry, historyStr string) (string, error) {

	// parse which tool orch asked for
	// response looks like: "TOOL: log_analysis\nREASON: user asking about errors"
	lines := strings.Split(strings.TrimSpace(toolResponse), "\n")
	toolName := strings.TrimPrefix(lines[0], "TOOL:")
	toolName = strings.TrimSpace(toolName)

	o.Events <- AgentEvent{Type: EventToolCall, Tool: toolName, Message: "running..."}

	// run the tool
	var toolResult string
	var err error

	switch toolName {
	case "log_analysis":
		out, e := o.logAgent.Run(ctx, AgentInput{Logs: logs, Query: originalQuery})
		toolResult, err = out.Result, e

	case "stats":
		out, e := o.statsAgent.Run(ctx, AgentInput{Logs: logs})
		toolResult, err = out.Result, e

	case "rca":
		// rca needs log analysis first
		logOut, e := o.logAgent.Run(ctx, AgentInput{Logs: logs, Query: originalQuery})
		if e != nil {
			return "", e
		}
		out, e := o.rcaAgent.Run(ctx, AgentInput{Query: originalQuery, Context: logOut.Result})
		toolResult, err = out.Result, e

	default:
		toolResult = "unknown tool"
	}

	if err != nil {
		return "", fmt.Errorf("tool %s: %w", toolName, err)
	}

	o.Events <- AgentEvent{Type: EventToolCall, Tool: toolName, Message: "done"}

	// feed tool result BACK to orchestrator so IT writes the final answer
	// this is key — tool result is not shown raw, orch synthesizes it
	finalPrompt := fmt.Sprintf(`
		You are Argus, an SRE AI assistant. 

		Conversation so far:
		%s

		You just ran the %s tool and got this result:
		%s

		Now answer the user's question: "%s"

		Be conversational and concise. No markdown. No preamble like "Based on the tool results".
		Just answer naturally as if you already knew this.
		`, historyStr, toolName, toolResult, originalQuery)

	finalAnswer, err := llms.GenerateFromSinglePrompt(ctx, o.client, finalPrompt)
	if err != nil {
		return "", err
	}

	o.history = append(o.history, ConversationTurn{Role: "assistant", Content: finalAnswer})
	o.Events <- AgentEvent{Type: EventAnswer, Tool: "orchestrator", Message: finalAnswer}
	return finalAnswer, nil
}

// RunStats runs just the stats agent — no LLM, instant response
// exported so main.go and TUI can call it directly for /stats preset
func (o *Orchestrator) RunStats(ctx context.Context, logs []LogEntry) (AgentOutput, error) {
	return o.statsAgent.Run(ctx, AgentInput{Logs: logs})
}

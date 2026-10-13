package workspace

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/tui/components"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
	"github.com/vishnuprasad2004/argus/internal/types"
)

// ── shared message types ──────────────────────────────────────────────────
// Decision: shared messages live here, not in screens package.
// Source-specific messages (procExitedMsg etc) stay in their screen files.

type QueryResultMsg struct {
	Result string
	Err    error
}

type AppendAnswerMsg struct {
	Content string
}

type ClearAnswersMsg struct{}

// ── query handling ────────────────────────────────────────────────────────

// HandleQuery routes /commands to presets, natural language to orchestrator.
// Each screen calls this — source-specific preset overrides handled via OnPreset.
// Decision: OnPreset callback lets screens inject source-specific commands
// (like /quit killing a process) without subclassing.
func (c *Core) HandleQuery(
	msg components.QuerySubmitMsg,
	logs []types.LogEntry,
	onPreset func(string) (tea.Cmd, bool), // returns cmd + handled bool
) tea.Cmd {
	wrapWidth := c.Width - 4
	if c.Dims.IsWide() {
		wrapWidth = c.Dims.InsightsPaneWidth - 4
	}

	if wrapWidth <= 0 {
		wrapWidth = 60 // safe fallback if dims not computed yet
	}

	userLine := styles.Brand.Render("❯ you") + "\n" +
		styles.WrapText(msg.Input, wrapWidth)

	var cmds []tea.Cmd
	cmds = append(cmds, func() tea.Msg {
		return AppendAnswerMsg{Content: userLine}
	})

	if msg.IsPreset {
		// let the screen handle source-specific commands first
		if onPreset != nil {
			if cmd, handled := onPreset(msg.Input); handled {
				cmds = append(cmds, cmd)
				return tea.Batch(cmds...)
			}
		}

		// shared presets
		switch msg.Input {
		case "/stats":
			cmds = append(cmds, func() tea.Msg {
				out, err := c.Orch.RunStats(context.Background(), logs)
				if err != nil {
					return QueryResultMsg{Err: err}
				}
				return QueryResultMsg{Result: out.Result}
			})
		case "/clear":
			cmds = append(cmds, func() tea.Msg {
				return ClearAnswersMsg{}
			})
		default:
			cmds = append(cmds, func() tea.Msg {
				return QueryResultMsg{
					Result: "unknown command. available: /stats /clear /quit",
				}
			})
		}
		return tea.Batch(cmds...)
	}

	// natural language — disable input, run orchestrator
	c.QueryBar.Disable()
	cmds = append(cmds,
		c.WatchOrchestratorEvents(),
		c.RunOrchestrator(msg.Input, logs),
	)
	return tea.Batch(cmds...)
}

func (c *Core) RunOrchestrator(query string, logs []types.LogEntry, sourceNames ...string) tea.Cmd {
	return func() tea.Msg {
		if c.Orch == nil {
			return QueryResultMsg{
				Err: fmt.Errorf("AI not configured — add GEMINI_API_KEY to ~/.argus/config.yaml"),
			}
		}
		_, err := c.Orch.Run(c.Ctx, query, logs, sourceNames...)
		if err != nil {
			return QueryResultMsg{Err: err}
		}
		return nil // success handled by EventChunk → EventAnswer
	}
}

func (c *Core) WatchOrchestratorEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-c.Orch.Events:
			return components.AgentEventMsg(event)
		case <-c.Ctx.Done():
			return nil
		}
	}
}

// ── answer management ─────────────────────────────────────────────────────

func (c *Core) RefreshAnswerVP() {
	// wrap each answer to the correct width before joining
	wrapWidth := c.Width - 4
	if c.Dims.IsWide() {
		wrapWidth = c.Dims.InsightsPaneWidth - 4
	}
	if wrapWidth <= 0 {
		wrapWidth = 60
	}

	var wrapped []string
	for _, answer := range c.AnswerContent {
		// lipgloss Width forces hard wrap at column boundary
		wrapped = append(wrapped, lipgloss.NewStyle().
			Width(wrapWidth).
			Render(answer))
	}

	content := strings.Join(wrapped, "\n\n")
	c.AnswerVP.SetContent(content)
	c.AnswerVP.GotoBottom()
}

func (c *Core) AppendChunk(chunk string) {
	if c.StreamingAnswer == "" {
		c.AnswerContent = append(c.AnswerContent,
			styles.AgentAnswer.Render("◆ Argus")+"\n")
	}
	c.StreamingAnswer += chunk
	last := len(c.AnswerContent) - 1
	c.AnswerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" +
		c.StreamingAnswer
	c.RefreshAnswerVP()
}

func (c *Core) FinalizeStreamingAnswer() {
	if c.StreamingAnswer == "" {
		return
	}
	last := len(c.AnswerContent) - 1
	rendered := styles.RenderMarkdown(c.StreamingAnswer, c.Width-4)
	c.AnswerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" + rendered
	c.StreamingAnswer = ""
	c.RefreshAnswerVP()
}

// HandleAgentEvent processes streaming events from the orchestrator.
// Returns a cmd to watch for the next event.
// Decision: this lives in Core so all screens get streaming for free.
func (c *Core) HandleAgentEvent(msg components.AgentEventMsg) tea.Cmd {
	switch agents.EventType(msg.Type) {
	case agents.EventChunk:
		c.AppendChunk(msg.Message)
		thinkCmd := c.Thinking.Update(msg)
		return tea.Batch(thinkCmd, c.WatchOrchestratorEvents())

	case agents.EventAnswer:
		c.FinalizeStreamingAnswer()
		c.QueryBar.Enable()
		c.Thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
		return nil

	default:
		thinkCmd := c.Thinking.Update(msg)
		return tea.Batch(thinkCmd, c.WatchOrchestratorEvents())
	}
}

// HandleQueryResult processes orchestrator results.
// Only handles errors — success is handled by EventChunk/EventAnswer.
func (c *Core) HandleQueryResult(msg QueryResultMsg) {
	c.QueryBar.Enable()
	if msg.Err != nil {
		errLine := styles.LogError.Render("✗ Error: " + msg.Err.Error())
		c.AnswerContent = append(c.AnswerContent, errLine)
		c.RefreshAnswerVP()
		c.Thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
		return
	}
	// success path — render the result
	if msg.Result != "" {
		rendered := styles.RenderMarkdown(msg.Result, c.Width-4)
		answer := styles.AgentAnswer.Render("◆ Argus") + "\n" + rendered
		c.AnswerContent = append(c.AnswerContent, answer)
		c.RefreshAnswerVP()
	}
	c.Thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
}

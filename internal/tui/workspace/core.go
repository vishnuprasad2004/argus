package workspace

import (
	"context"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/tui/components"
	"github.com/vishnuprasad2004/argus/internal/tui/layout"
)

// Core holds all state and logic shared across every workspace screen.
// Embed this into ChatModel, ProcessChatModel, FileChatModel.
// Decision: embedding over interface — screens keep their own types,
// app.go doesn't change, no type assertions needed anywhere.
type Core struct {
	Width       int
	Height      int
	Initialized bool

	// components
	LogViewer *components.LogViewer
	QueryBar  components.QueryBar
	Thinking  components.ThinkingIndicator

	// answer panel
	AnswerVP      viewport.Model
	AnswerContent []string
	AnswerReady   bool

	// layout
	FocusedPanel int // 0 = logs, 1 = insights
	Dims         layout.Dims

	// AI
	Orch *agents.Orchestrator

	// lifecycle
	Ctx    context.Context
	Cancel context.CancelFunc

	// streaming state
	StreamingAnswer string
}

// NewCore constructs the shared core — call from each screen's constructor.
// llm can be nil on first run (wizard not completed yet).
func NewCore(ctx context.Context, cancel context.CancelFunc, llm *agents.GeminiClient) Core {
	return Core{
		QueryBar: components.NewQueryBar(),
		Thinking: components.NewThinkingIndicator(),
		Orch:     agents.NewOrchestrator(llm),
		Ctx:      ctx,
		Cancel:   cancel,
	}
}

// Init starts the spinner and fires an immediate WindowSizeMsg.
// Call this from each screen's Init().
func (c *Core) Init() tea.Cmd {
	return tea.Batch(
		c.Thinking.Init(),
		tea.WindowSize(),
	)
}

// Relayout recalculates all panel dimensions from terminal size.
// Call this every time WindowSizeMsg arrives.
func (c *Core) Relayout() {
	c.Dims = layout.Compute(c.Width, c.Height)

	if c.Dims.IsWide() {
		logW := c.Dims.LogPaneWidth - 2
		logH := c.Dims.LogPaneHeight
		insW := c.Dims.InsightsPaneWidth - 2
		insH := c.Dims.InsightsPaneHeight

		if c.LogViewer == nil {
			lv := components.NewLogViewer(logW, logH)
			c.LogViewer = &lv
		} else {
			c.LogViewer.Resize(logW, logH)
		}

		if !c.AnswerReady {
			c.AnswerVP.Width  = insW
    	c.AnswerVP.Height = insH
			c.AnswerVP = viewport.New(insW, insH)
			c.AnswerReady = true
		} else {
			c.AnswerVP.Width = insW
			c.AnswerVP.Height = insH
		}
	} else {
		paneW := c.Dims.PaneWidth - 2
		paneH := c.Dims.PaneHeight

		if c.LogViewer == nil {
			lv := components.NewLogViewer(paneW, paneH)
			c.LogViewer = &lv
		} else {
			c.LogViewer.Resize(paneW, paneH)
		}

		if !c.AnswerReady {
			c.AnswerVP = viewport.New(paneW, paneH)
			c.AnswerReady = true
		} else {
			c.AnswerVP.Width = paneW
			c.AnswerVP.Height = paneH
		}
	}
}

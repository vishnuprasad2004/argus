package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/collectors/docker"
	"github.com/vishnuprasad2004/argus/internal/tui/components"
	"github.com/vishnuprasad2004/argus/internal/tui/layout"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
	"github.com/vishnuprasad2004/argus/internal/types"
)

// ── message types ─────────────────────────────────────────────────────────

type newLogEntryMsg types.LogEntry
type streamEndedMsg struct{ err error }
type queryResultMsg struct {
	result string
	err    error
}
type streamStartedMsg struct {
	logCh <-chan types.LogEntry
	errCh <-chan error
}
type logsLoadedMsg struct {
	logs      []types.LogEntry
	collector *docker.DockerCollector
}

type streamChunkMsg struct {
	chunk string
}

type appendAnswerMsg struct {
	content string
}

type clearAnswersMsg struct{}

// ── model ─────────────────────────────────────────────────────────────────

type ChatModel struct {
	width       int
	height      int
	initialized bool

	// components — logViewer is a pointer so AppendLog mutations stick
	logViewer *components.LogViewer
	queryBar  components.QueryBar
	thinking  components.ThinkingIndicator

	// state
	target docker.ContainerTarget
	orch   *agents.Orchestrator

	answerVP      viewport.Model
	answerContent []string // raw strings, re-rendered into viewport
	answerReady   bool

	// to manage which panel is focused for keyboard input when scrolling
	focusedPanel int // 0 = logs, 1 = answers
	dims         layout.Dims

	// live stream channels — set when streamStartedMsg arrives
	liveCh    <-chan types.LogEntry
	liveErrCh <-chan error

	ctx    context.Context
	cancel context.CancelFunc

	streamingAnswer string
}

func NewChatModel(target docker.ContainerTarget, llm *agents.GeminiClient) ChatModel {
	ctx, cancel := context.WithCancel(context.Background())

	return ChatModel{
		target:   target,
		queryBar: components.NewQueryBar(),
		thinking: components.NewThinkingIndicator(),
		orch:     agents.NewOrchestrator(llm),
		ctx:      ctx,
		cancel:   cancel,
		// logViewer intentionally nil here — initialized in relayout()
		// because we don't know terminal dimensions yet
	}
}

// ── init ──────────────────────────────────────────────────────────────────

func (m ChatModel) Init() tea.Cmd {
	// only start spinner here — wait for WindowSizeMsg before fetching logs
	// WindowSizeMsg always fires first in bubbletea, so this is safe
	return tea.Batch(
		m.thinking.Init(),
		tea.WindowSize(), // forces WindowSizeMsg immediately
	)
}

// ── update ────────────────────────────────────────────────────────────────

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// WindowSizeMsg fires immediately on start — this is where we bootstrap
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout() // creates logViewer with correct dimensions

		if !m.initialized {
			m.initialized = true
			return m, m.loadLogs() // trigger Docker fetch now that size is known
		}
		return m, nil

	// historical logs fetched — fill viewer, optionally start stream
	case logsLoadedMsg:
		for _, entry := range msg.logs {
			m.logViewer.AppendLog(entry)
		}
		if m.target.Status == "running" {
			return m, m.startLiveStream(msg.collector)
		}
		return m, nil

	// stream channels ready — store them and start read chain
	case streamStartedMsg:
		m.liveCh = msg.logCh
		m.liveErrCh = msg.errCh
		return m, m.waitForNextLog()

	// one live log line arrived — append and schedule next read
	case newLogEntryMsg:
		m.logViewer.AppendLog(types.LogEntry(msg))
		return m, m.waitForNextLog()

	// stream ended — container stopped or error
	case streamEndedMsg:
		warning := types.LogEntry{
			Level:  "WARN",
			Source: "argus",
			Message: func() string {
				if msg.err != nil {
					return fmt.Sprintf("stream ended: %v", msg.err)
				}
				return fmt.Sprintf("container %s stopped", m.target.Name)
			}(),
		}
		m.logViewer.AppendLog(warning)
		m.liveCh = nil
		m.liveErrCh = nil
		return m, nil

	// orchestrator finished — re-enable input and show answer
case queryResultMsg:
    m.queryBar.Enable()
    if msg.err != nil {
        // error path — streaming didn't handle this
        errLine := styles.LogError.Render("✗ Error: " + msg.err.Error())
        m.answerContent = append(m.answerContent, errLine)
        m.refreshAnswerVP()
    }
    // success path intentionally empty — EventAnswer handles it
    m.thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
    return m, nil

	case appendAnswerMsg:
		m.answerContent = append(m.answerContent, msg.content)
		m.refreshAnswerVP()
		return m, nil

	case clearAnswersMsg:
		m.answerContent = nil
		m.answerVP.SetContent("")
		return m, nil

	// agent event — forward to thinking indicator
	case components.AgentEventMsg:
		switch agents.EventType(msg.Type) {
		case agents.EventChunk:
			m.appendChunk(msg.Message)
			cmd := m.thinking.Update(msg)
			return m, tea.Batch(cmd, m.watchOrchestratorEvents())
		case agents.EventAnswer:
			m.finalizeStreamingAnswer()
			m.queryBar.Enable()
			m.thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
			return m, nil
		default:
			cmd := m.thinking.Update(msg)
			return m, tea.Batch(cmd, m.watchOrchestratorEvents())
		}
	// user submitted query or preset command
	case components.QuerySubmitMsg:
		return m, m.handleQuery(msg)

	// keyboard shortcuts
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focusedPanel = (m.focusedPanel + 1) % 2 // toggle between 0 and 1
			return m, nil
		case "esc":
			m.cancel()
			return m, func() tea.Msg { return SwitchToSourceSelect{} }
		}
	}

	// forward remaining messages to sub-components
	// route scroll events to the focused panel
	if m.focusedPanel == 0 {
		if m.logViewer != nil {
			logCmd := m.logViewer.Update(msg)
			cmds = append(cmds, logCmd)
		}
	} else {
		if m.answerReady {
			var cmd tea.Cmd
			m.answerVP, cmd = m.answerVP.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	queryCmd := m.queryBar.Update(msg)
	thinkingCmd := m.thinking.Update(msg)
	cmds = append(cmds, queryCmd, thinkingCmd)

	return m, tea.Batch(cmds...)
}

// ── commands ──────────────────────────────────────────────────────────────

// loadLogs fetches last 200 lines from Docker — runs in goroutine via tea.Cmd
func (m ChatModel) loadLogs() tea.Cmd {
	return func() tea.Msg {
		collector, err := docker.NewDockerCollector()
		if err != nil {
			return queryResultMsg{err: fmt.Errorf("docker: %w", err)}
		}

		logs, err := collector.FetchLogs(context.Background(), m.target, docker.FetchOptions{
			TailLines: "200",
		})
		if err != nil {
			return queryResultMsg{err: fmt.Errorf("fetch logs: %w", err)}
		}

		return logsLoadedMsg{logs: logs, collector: collector}
	}
}

// startLiveStream starts Docker log stream — returns channels as a message
// so they can be stored on the real model inside Update
func (m ChatModel) startLiveStream(collector *docker.DockerCollector) tea.Cmd {
	return func() tea.Msg {
		logCh, errCh := collector.Stream(m.ctx, m.target)
		return streamStartedMsg{logCh: logCh, errCh: errCh}
	}
}

// waitForNextLog blocks until one log line arrives then returns it
// Update reschedules this after each line — creates a continuous chain
func (m ChatModel) waitForNextLog() tea.Cmd {
	if m.liveCh == nil {
		return nil // no stream active
	}
	return func() tea.Msg {
		select {
		case entry, ok := <-m.liveCh:
			if !ok {
				return streamEndedMsg{}
			}
			return newLogEntryMsg(entry)
		case err := <-m.liveErrCh:
			return streamEndedMsg{err: err}
		case <-m.ctx.Done():
			return streamEndedMsg{}
		}
	}
}

// handleQuery routes /commands to presets and natural language to orchestrator
func (m ChatModel) handleQuery(msg components.QuerySubmitMsg) tea.Cmd {
	userLine := styles.Brand.Render("❯ you") + "\n" +
		styles.WrapText(msg.Input, m.width-4)

	// DON'T do: m.answerContent = append(m.answerContent, userLine)
	// that mutates a copy — use a message instead so Update() handles it

	var cmds []tea.Cmd

	// fire appendAnswerMsg so the real model gets the user line
	cmds = append(cmds, func() tea.Msg {
		return appendAnswerMsg{content: userLine}
	})

	if msg.IsPreset {
		switch msg.Input {
		case "/stats":
			logs := m.logViewer.Logs()
			cmds = append(cmds, func() tea.Msg {
				out, err := m.orch.RunStats(context.Background(), logs)
				if err != nil {
					return queryResultMsg{err: err}
				}
				return queryResultMsg{result: out.Result}
			})
			return tea.Batch(cmds...)

		case "/clear":
			cmds = append(cmds, func() tea.Msg {
				return clearAnswersMsg{}
			})
			return tea.Batch(cmds...)

		case "/quit":
			m.cancel()
			return tea.Quit

		default:
			cmds = append(cmds, func() tea.Msg {
				return queryResultMsg{result: "unknown command. available: /stats /clear /quit"}
			})
			return tea.Batch(cmds...)
		}
	}

	m.queryBar.Disable()
	logs := m.logViewer.Logs()
	cmds = append(cmds,
		m.watchOrchestratorEvents(),
		m.runOrchestrator(msg.Input, logs),
	)
	return tea.Batch(cmds...)
}

func (m ChatModel) runOrchestrator(query string, logs []types.LogEntry) tea.Cmd {
	return func() tea.Msg {
		if m.orch == nil {
			return queryResultMsg{
				err: fmt.Errorf("AI not configured — add GEMINI_API_KEY to ~/.argus/config.yaml"),
			}
		}
		_, err := m.orch.Run(m.ctx, query, logs, m.target.Name)
		if err != nil {
			// only send error result — success is handled by EventChunk/EventAnswer
			return queryResultMsg{err: err}
		}
		// success case: streaming already handled it via EventChunk → EventAnswer
		// return nil so no duplicate rendering happens
		return nil
	}
}

// watchOrchestratorEvents reads ONE event from orch.Events channel
// chat.go re-schedules this after each event so all events are shown
func (m ChatModel) watchOrchestratorEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.orch.Events:
			return components.AgentEventMsg(event)
		case <-m.ctx.Done():
			return nil
		}
	}
}

// ── layout ────────────────────────────────────────────────────────────────

func (m *ChatModel) relayout() {
	m.dims = layout.Compute(m.width, m.height)

	if m.dims.IsWide() {
		// wide: log pane left, insights pane right
		logW := m.dims.LogPaneWidth - 2
		logH := m.dims.LogPaneHeight
		insW := m.dims.InsightsPaneWidth - 2
		insH := m.dims.InsightsPaneHeight

		if m.logViewer == nil {
			lv := components.NewLogViewer(logW, logH)
			m.logViewer = &lv
		} else {
			m.logViewer.Resize(logW, logH)
		}

		if !m.answerReady {
			m.answerVP = viewport.New(insW, insH)
			m.answerReady = true
		} else {
			m.answerVP.Width = insW
			m.answerVP.Height = insH
		}

	} else {
		// narrow: single pane, tabs switch between log and insights
		paneW := m.dims.PaneWidth - 2
		paneH := m.dims.PaneHeight

		if m.logViewer == nil {
			lv := components.NewLogViewer(paneW, paneH)
			m.logViewer = &lv
		} else {
			m.logViewer.Resize(paneW, paneH)
		}

		if !m.answerReady {
			m.answerVP = viewport.New(paneW, paneH)
			m.answerReady = true
		} else {
			m.answerVP.Width = paneW
			m.answerVP.Height = paneH
		}
	}
}

// call this every time answerContent changes
func (m *ChatModel) refreshAnswerVP() {
	content := strings.Join(m.answerContent, "\n\n")
	m.answerVP.SetContent(content)
	m.answerVP.GotoBottom() // auto scroll to latest answer
}

// ── view ──────────────────────────────────────────────────────────────────

func (m ChatModel) View() string {
	if m.width == 0 || m.logViewer == nil {
		return "\n  " + styles.Muted.Render("initializing...")
	}

	// ── header — same as before ───────────────────────────────────────
	dot := styles.StatusDot(m.target.Status == "running")
	header := fmt.Sprintf("  %s  %s %s  %s  %s\n",
		styles.Brand.Render("argus"),
		dot,
		styles.Title.Render(m.target.Name),
		styles.Muted.Render(m.target.Image),
		styles.Muted.Render("tab: switch panel  esc: back  /stats /clear /quit"),
	)

	// ── thinking indicator ────────────────────────────────────────────
	thinkingStr := m.thinking.View()

	// ── query bar ─────────────────────────────────────────────────────
	queryStr := "\n" + m.queryBar.View() + "\n"

	// ── footer shortcuts ──────────────────────────────────────────────
	focused := map[int]string{0: "logs", 1: "answers"}[m.focusedPanel]
	shortcuts := []struct{ key, desc string }{
		{"↑↓", "scroll"},
		{"tab", "switch panel"},
		{"esc", "back"},
		{"/stats", "counts"},
		{"/clear", "clear"},
		{"/quit", "quit"},
	}
	var keys, descs strings.Builder
	for i, s := range shortcuts {
		keys.WriteString(styles.Brand.Render(s.key))
		descs.WriteString(styles.Muted.Render(s.desc))
		if i < len(shortcuts)-1 {
			keys.WriteString(styles.Muted.Render("  ·  "))
			descs.WriteString(styles.Muted.Render("  ·  "))
		}
	}
	footer := styles.HRuleStr(m.width) + "\n" +
		"  " + keys.String() + "\n" +
		"  " + descs.String() + "\n" +
		"  " + styles.Muted.Render("panel: "+focused) + "\n"

	// ── insights pane content ─────────────────────────────────────────
	var insightsContent string
	if len(m.answerContent) == 0 {
		insightsContent = "  " + styles.Muted.Render("ask a question below...")
	} else {
		insightsContent = m.answerVP.View()
	}
	if thinkingStr != "" {
		insightsContent += "\n" + thinkingStr
	}

	// ── assemble by layout mode ───────────────────────────────────────
	var b strings.Builder
	b.WriteString(header)

	if m.dims.IsWide() {
		// ── wide: log left, insights right ───────────────────────────
		logBorder := styles.HRuleStr(m.dims.LogPaneWidth)
		if m.focusedPanel == 0 {
			logBorder = styles.Brand.Render(strings.Repeat("─", m.dims.LogPaneWidth))
		}

		insBorder := styles.HRuleStr(m.dims.InsightsPaneWidth)
		if m.focusedPanel == 1 {
			insBorder = styles.Brand.Render(strings.Repeat("─", m.dims.InsightsPaneWidth))
		}

		// build left column: rule + log pane
		left := logBorder + "\n" + m.logViewer.View()

		// build right column: rule + insights
		right := insBorder + "\n" + insightsContent

		// join horizontally — lipgloss aligns tops
		divider := layout.Divider(m.dims.LogPaneHeight + 1) // +1 for rule row
		panes := layout.JoinPanes(left, divider, right)

		b.WriteString(panes)
		b.WriteString("\n")

	} else {
		// ── narrow: tab bar + single active pane ─────────────────────
		b.WriteString(layout.TabBar(m.dims.PaneWidth, m.focusedPanel))
		b.WriteString("\n")

		if m.focusedPanel == 0 {
			b.WriteString(styles.Brand.Render(strings.Repeat("─", m.width)) + "\n")
			b.WriteString(m.logViewer.View())
		} else {
			b.WriteString(styles.Brand.Render(strings.Repeat("─", m.width)) + "\n")
			b.WriteString(insightsContent)
		}
		b.WriteString("\n")
	}

	if thinkingStr != "" && m.dims.IsWide() {
		// in wide mode thinking already rendered in right pane
		// skip here to avoid duplication
	}

	b.WriteString(queryStr)
	b.WriteString(footer)

	return b.String()
}

// appendChunk adds a token to the currently-streaming answer
func (m *ChatModel) appendChunk(chunk string) {
	if m.streamingAnswer == "" {
		// first chunk — add the "◆ Argus" header
		m.answerContent = append(m.answerContent,
			styles.AgentAnswer.Render("◆ Argus")+"\n")
	}
	m.streamingAnswer += chunk

	// update the last answer entry with accumulated chunks
	last := len(m.answerContent) - 1
	m.answerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" +
		m.streamingAnswer // raw for now, render markdown on finalize

	m.refreshAnswerVP()
}

// finalizeStreamingAnswer renders markdown on the completed response
func (m *ChatModel) finalizeStreamingAnswer() {
	if m.streamingAnswer == "" {
		return
	}
	last := len(m.answerContent) - 1
	rendered := styles.RenderMarkdown(m.streamingAnswer, m.width-4)
	m.answerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" + rendered
	m.streamingAnswer = "" // reset for next answer
	m.refreshAnswerVP()
}

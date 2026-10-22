package screens

import (
	"context"
	"fmt"
	// "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	dockerclient "github.com/docker/docker/client"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/collectors/docker"
	"github.com/vishnuprasad2004/argus/internal/tui/components"
	// "github.com/vishnuprasad2004/argus/internal/tui/layout"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
	"github.com/vishnuprasad2004/argus/internal/tui/workspace"
	"github.com/vishnuprasad2004/argus/internal/types"
	"strings"
)

// ── message types ─────────────────────────────────────────────────────────

type newLogEntryMsg types.LogEntry
type streamEndedMsg struct{ err error }
type queryResultMsg struct {
	result string
	err    error
}

type streamReadyMsg struct{}

type logsLoadedMsg struct {
	logs      []types.LogEntry
	collector *docker.DockerCollector
}

// ── model ─────────────────────────────────────────────────────────────────

type ChatModel struct {
	workspace.Core // all the shared workspace state and logic

	target    docker.ContainerTarget // the container we're attached to
	logFeed   chan types.LogEntry    // channel for live log lines from Docker
	dockerCli *dockerclient.Client   // used for metrics
}

func NewChatModel(target docker.ContainerTarget, llm *agents.GeminiClient) ChatModel {
	ctx, cancel := context.WithCancel(context.Background())

	cli, _ := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)

	return ChatModel{
		target:    target,
		Core:      workspace.NewCore(ctx, cancel, llm),
		logFeed:   make(chan types.LogEntry, 500),
		dockerCli: cli,
		// logViewer intentionally nil here — initialized in relayout()
		// because we don't know terminal dimensions yet
	}
}

// ── init ──────────────────────────────────────────────────────────────────

func (m ChatModel) Init() tea.Cmd {
	// only start spinner here — wait for WindowSizeMsg before fetching logs
	// WindowSizeMsg always fires first in bubbletea, so this is safe
	cmds := []tea.Cmd{m.Core.Init()}
	if m.target.IsLive {
		cmds = append(cmds, workspace.StartMetricsTicker())
	}
	return tea.Batch(cmds...)
}

// ── update ────────────────────────────────────────────────────────────────

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// WindowSizeMsg fires immediately on start — this is where we bootstrap
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Relayout() // creates logViewer with correct dimensions

		if !m.Initialized {
			m.Initialized = true
			return m, m.loadLogs() // trigger Docker fetch now that size is known
		}
		return m, nil

	// historical logs fetched — fill viewer, optionally start stream
	case logsLoadedMsg:
		for _, entry := range msg.logs {
			m.LogViewer.AppendLog(entry)
		}
		if m.target.IsLive {
			return m, m.startLiveStream(msg.collector)
		}
		return m, nil

	// stream channels ready — store them and start read chain
	case streamReadyMsg:
		return m, m.waitForNextLog() // start the chain

	// one live log line arrived — append and schedule next read
	case newLogEntryMsg:
		m.LogViewer.AppendLog(types.LogEntry(msg))
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
		m.LogViewer.AppendLog(warning)
		m.logFeed = nil // disable further reads
		return m, nil

		// orchestrator finished — re-enable input and show answer
	case workspace.QueryResultMsg:
		m.QueryBar.Enable()
		if msg.Err != nil {
			// error path — streaming didn't handle this
			errLine := styles.LogError.Render("✗ Error: " + msg.Err.Error())
			m.AnswerContent = append(m.AnswerContent, errLine)
			m.refreshAnswerVP()
		}
		// success path intentionally empty — EventAnswer handles it
		m.Thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
		return m, nil

	case workspace.AppendAnswerMsg:
		m.AnswerContent = append(m.AnswerContent, msg.Content)
		m.RefreshAnswerVP()
		return m, nil

	case workspace.ClearAnswersMsg:
		m.AnswerContent = nil
		m.AnswerVP.SetContent("")
		return m, nil
	case workspace.MetricsTickMsg:
		return m, m.fetchMetrics()

	case workspace.MetricsUpdatedMsg:
		m.MetricsPanel.Update(msg.Metrics)
		m.ShowMetrics = true
		return m, workspace.StartMetricsTicker() // reschedule

	// agent event — forward to thinking indicator
	case components.AgentEventMsg:
		switch agents.EventType(msg.Type) {
		case agents.EventChunk:
			m.AppendChunk(msg.Message)
			cmd := m.Thinking.Update(msg)
			return m, tea.Batch(cmd, m.WatchOrchestratorEvents())
		case agents.EventAnswer:
			m.finalizeStreamingAnswer()
			m.QueryBar.Enable()
			m.Thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
			return m, nil
		default:
			cmd := m.Thinking.Update(msg)
			return m, tea.Batch(cmd, m.WatchOrchestratorEvents())
		}
	// user submitted query or preset command
	case components.QuerySubmitMsg:
		logs := m.LogViewer.Logs()
		return m, m.HandleQuery(msg, logs, m.presetHandler())

	// keyboard shortcuts
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+f":
			if m.FocusedPanel == 0 && m.LogViewer != nil {
				m.LogViewer.ToggleFollow()
			}
			return m, nil // return m, nil

		case "end":
			if m.FocusedPanel == 0 && m.LogViewer != nil {
				m.LogViewer.EnableFollow()
			}
			return m, nil
		case "tab":
			m.FocusedPanel = (m.FocusedPanel + 1) % 2 // toggle between 0 and 1
			return m, nil
		case "esc":
			m.Cancel()
			return m, func() tea.Msg { return SwitchToSourceSelect{} }
		}
	}

	// forward remaining messages to sub-components
	// route scroll events to the focused panel
	// scroll keys → focused panel only, NOT query bar
	if isScrollKey(msg) {
		if m.FocusedPanel == 0 && m.LogViewer != nil {
			logCmd := m.LogViewer.Update(msg)
			cmds = append(cmds, logCmd)
		} else if m.FocusedPanel == 1 && m.AnswerReady {
			var cmd tea.Cmd
			m.AnswerVP, cmd = m.AnswerVP.Update(msg)
			cmds = append(cmds, cmd)
		}
		thinkingCmd := m.Thinking.Update(msg)
		cmds = append(cmds, thinkingCmd)
		return m, tea.Batch(cmds...)
	}

	// everything else → query bar + thinking
	queryCmd := m.QueryBar.Update(msg)
	thinkingCmd := m.Thinking.Update(msg)
	cmds = append(cmds, queryCmd, thinkingCmd)

	return m, tea.Batch(cmds...)
}

// ── commands ──────────────────────────────────────────────────────────────

// loadLogs fetches last 200 lines from Docker — runs in goroutine via tea.Cmd
func (m ChatModel) loadLogs() tea.Cmd {
	return func() tea.Msg {
		// fmt.Fprintf(os.Stderr, "DEBUG: loadLogs() called\n")
		collector, err := docker.NewDockerCollector()
		if err != nil {
			// fmt.Fprintf(os.Stderr, "DEBUG: docker collector error: %v\n", err)
			return queryResultMsg{err: fmt.Errorf("docker: %w", err)}
		}

		logs, err := collector.FetchLogs(context.Background(), m.target, docker.FetchOptions{
			TailLines: "200",
		})
		// fmt.Fprintf(os.Stderr, "DEBUG: fetched %d logs, err=%v\n", len(logs), err)
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
		logCh, _ := collector.Stream(m.Ctx, m.target)

		go func() {
			for {
				select {
				case entry, ok := <-logCh:
					if !ok {
						return
					}
					m.logFeed <- entry
				case <-m.Ctx.Done():
					return
				}
			}
		}()

		return streamReadyMsg{}
	}
}

// waitForNextLog blocks until one log line arrives then returns it
// Update reschedules this after each line — creates a continuous chain
func (m ChatModel) waitForNextLog() tea.Cmd {
	if m.logFeed == nil {
		return nil
	}
	feed := m.logFeed // capture in closure — safe because chan is reference type
	ctx := m.Ctx

	return func() tea.Msg {
		select {
		case entry, ok := <-feed:
			if !ok {
				return streamEndedMsg{}
			}
			return newLogEntryMsg(entry)
		case <-ctx.Done():
			return streamEndedMsg{}
		}
	}
}

// // handleQuery routes /commands to presets and natural language to orchestrator
// func (m ChatModel) handleQuery(msg components.QuerySubmitMsg) tea.Cmd {
// 	userLine := styles.Brand.Render("❯ you") + "\n" +
// 		styles.WrapText(msg.Input, m.Width-4)

// 	// DON'T do: m.answerContent = append(m.answerContent, userLine)
// 	// that mutates a copy — use a message instead so Update() handles it

// 	var cmds []tea.Cmd

// 	// fire appendAnswerMsg so the real model gets the user line
// 	cmds = append(cmds, func() tea.Msg {
// 		return appendAnswerMsg{content: userLine}
// 	})

// 	if msg.IsPreset {
// 		switch msg.Input {
// 		case "/stats":
// 			logs := m.LogViewer.Logs()
// 			cmds = append(cmds, func() tea.Msg {
// 				out, err := m.Orch.RunStats(context.Background(), logs)
// 				if err != nil {
// 					return queryResultMsg{err: err}
// 				}
// 				return queryResultMsg{result: out.Result}
// 			})
// 			return tea.Batch(cmds...)

// 		case "/clear":
// 			cmds = append(cmds, func() tea.Msg {
// 				return clearAnswersMsg{}
// 			})
// 			return tea.Batch(cmds...)

// 		case "/quit":
// 			m.Cancel()
// 			return tea.Quit

// 		default:
// 			cmds = append(cmds, func() tea.Msg {
// 				return queryResultMsg{result: "unknown command. available: /stats /clear /quit"}
// 			})
// 			return tea.Batch(cmds...)
// 		}
// 	}

// 	m.QueryBar.Disable()
// 	logs := m.LogViewer.Logs()
// 	cmds = append(cmds,
// 		m.watchOrchestratorEvents(),
// 		m.runOrchestrator(msg.Input, logs),
// 	)
// 	return tea.Batch(cmds...)
// }

// func (m ChatModel) runOrchestrator(query string, logs []types.LogEntry) tea.Cmd {
// 	return func() tea.Msg {
// 		if m.Orch == nil {
// 			return queryResultMsg{
// 				err: fmt.Errorf("AI not configured — add GEMINI_API_KEY to ~/.argus/config.yaml"),
// 			}
// 		}
// 		_, err := m.Orch.Run(m.Ctx, query, logs, m.target.Name)
// 		if err != nil {
// 			// only send error result — success is handled by EventChunk/EventAnswer
// 			return queryResultMsg{err: err}
// 		}
// 		// success case: streaming already handled it via EventChunk → EventAnswer
// 		// return nil so no duplicate rendering happens
// 		return nil
// 	}
// }

// watchOrchestratorEvents reads ONE event from orch.Events channel
// chat.go re-schedules this after each event so all events are shown
// func (m ChatModel) watchOrchestratorEvents() tea.Cmd {
// 	return func() tea.Msg {
// 		select {
// 		case event := <-m.Orch.Events:
// 			return components.AgentEventMsg(event)
// 		case <-m.Ctx.Done():
// 			return nil
// 		}
// 	}
// }

// ── layout ────────────────────────────────────────────────────────────────

// func (m *ChatModel) relayout() {
// 	m.Dims = layout.Compute(m.Width, m.Height)

// 	if m.Dims.IsWide() {
// 		// wide: log pane left, insights pane right
// 		logW := m.Dims.LogPaneWidth - 2
// 		logH := m.Dims.LogPaneHeight
// 		insW := m.Dims.InsightsPaneWidth - 2
// 		insH := m.Dims.InsightsPaneHeight

// 		if m.LogViewer == nil {
// 			lv := components.NewLogViewer(logW, logH)
// 			m.LogViewer = &lv
// 		} else {
// 			m.LogViewer.Resize(logW, logH)
// 		}

// 		if !m.AnswerReady {
// 			m.AnswerVP = viewport.New(insW, insH)
// 			m.AnswerReady = true
// 		} else {
// 			m.AnswerVP.Width = insW
// 			m.AnswerVP.Height = insH
// 		}

// 	} else {
// 		// narrow: single pane, tabs switch between log and insights
// 		paneW := m.Dims.PaneWidth - 2
// 		paneH := m.Dims.PaneHeight

// 		if m.LogViewer == nil {
// 			lv := components.NewLogViewer(paneW, paneH)
// 			m.LogViewer = &lv
// 		} else {
// 			m.LogViewer.Resize(paneW, paneH)
// 		}

// 		if !m.AnswerReady {
// 			m.AnswerVP = viewport.New(paneW, paneH)
// 			m.AnswerReady = true
// 		} else {
// 			m.AnswerVP.Width = paneW
// 			m.AnswerVP.Height = paneH
// 		}
// 	}
// }

// call this every time answerContent changes
func (m *ChatModel) refreshAnswerVP() {
	content := strings.Join(m.AnswerContent, "\n\n")
	m.AnswerVP.SetContent(content)
	m.AnswerVP.GotoBottom() // auto scroll to latest answer
}

// ── view ──────────────────────────────────────────────────────────────────

func (m ChatModel) View() string {
	if m.Width == 0 || m.LogViewer == nil {
		return "\n  " + styles.Muted.Render("initializing...")
	}

	// ── header — same as before ───────────────────────────────────────
	dot := styles.StatusDot(m.target.IsLive)
	header := fmt.Sprintf("  %s  %s %s  %s  %s\n",
		styles.Brand.Render("argus"),
		dot,
		styles.Title.Render(m.target.Name),
		styles.Muted.Render(m.target.Image),
		styles.Muted.Render("tab: switch panel  esc: back  /stats /clear /quit"),
	)
	return m.Core.RenderView(header)
}

// appendChunk adds a token to the currently-streaming answer
// func (m *ChatModel) appendChunk(chunk string) {
// 	if m.StreamingAnswer == "" {
// 		// first chunk — add the "◆ Argus" header
// 		m.AnswerContent = append(m.AnswerContent,
// 			styles.AgentAnswer.Render("◆ Argus")+"\n")
// 	}
// 	m.StreamingaAnswer += chunk

// 	// update the last answer entry with accumulated chunks
// 	last := len(m.AnswerContent) - 1
// 	m.AnswerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" +
// 		m.StreamingAnswer // raw for now, render markdown on finalize

// 	m.refreshAnswerVP()
// }

// finalizeStreamingAnswer renders markdown on the completed response
func (m *ChatModel) finalizeStreamingAnswer() {
	if m.StreamingAnswer == "" {
		return
	}
	wrapWidth := m.Width - 4
	if m.Dims.IsWide() {
		wrapWidth = m.Dims.InsightsPaneWidth - 6
	}
	last := len(m.AnswerContent) - 1
	rendered := styles.RenderMarkdown(m.StreamingAnswer, wrapWidth)
	m.AnswerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" + rendered
	m.StreamingAnswer = "" // reset for next answer
	m.refreshAnswerVP()
}

// presetHandler returns docker-specific preset overrides
// Decision: callback pattern so Core.HandleQuery doesn't know about sources
func (m ChatModel) presetHandler() func(string) (tea.Cmd, bool) {
	return func(input string) (tea.Cmd, bool) {
		switch input {
		case "/quit":
			m.Cancel()
			return tea.Quit, true
		}
		return nil, false // not handled — Core handles the rest
	}
}

// isScrollKey returns true for keys that should scroll the focused panel
// all other keys go to the query bar only
func isScrollKey(msg tea.Msg) bool {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return false // non-key messages (spinner ticks etc) go everywhere
	}
	switch keyMsg.String() {
	case "up", "down",
		"pgup", "pgdown",
		"ctrl+u", "ctrl+d",
		"ctrl+b",
		"home", "end":
		return true
	}
	return false
}

func (m ChatModel) fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		metrics := docker.FetchMetrics(m.Ctx, m.dockerCli, m.target.ID)
		return workspace.MetricsUpdatedMsg{Metrics: metrics}
	}
}

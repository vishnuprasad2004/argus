package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/collectors/docker"
	"github.com/vishnuprasad2004/argus/internal/tui/components"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// ── message types ─────────────────────────────────────────────────────────

type newLogEntryMsg  agents.LogEntry
type streamEndedMsg  struct{ err error }
type queryResultMsg  struct{ result string; err error }
type streamStartedMsg struct {
	logCh <-chan agents.LogEntry
	errCh <-chan error
}
type logsLoadedMsg struct {
	logs      []agents.LogEntry
	collector *docker.DockerCollector
}

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
	target  docker.ContainerTarget
	orch    *agents.Orchestrator
	answers []string

	// live stream channels — set when streamStartedMsg arrives
	liveCh    <-chan agents.LogEntry
	liveErrCh <-chan error

	ctx    context.Context
	cancel context.CancelFunc
}

func NewChatModel(target docker.ContainerTarget, llm *googleai.GoogleAI) ChatModel {
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
		m.width  = msg.Width
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
		m.liveCh    = msg.logCh
		m.liveErrCh = msg.errCh
		return m, m.waitForNextLog()

	// one live log line arrived — append and schedule next read
	case newLogEntryMsg:
		m.logViewer.AppendLog(agents.LogEntry(msg))
		return m, m.waitForNextLog()

	// stream ended — container stopped or error
	case streamEndedMsg:
		warning := agents.LogEntry{
			Level:   "WARN",
			Source:  "argus",
			Message: func() string {
				if msg.err != nil {
					return fmt.Sprintf("stream ended: %v", msg.err)
				}
				return fmt.Sprintf("container %s stopped", m.target.Name)
			}(),
		}
		m.logViewer.AppendLog(warning)
		m.liveCh    = nil
		m.liveErrCh = nil
		return m, nil

	// orchestrator finished — re-enable input and show answer
	case queryResultMsg:
		m.queryBar.Enable()
    if msg.err != nil {
        errLine := styles.LogError.Render("✗ Error: " + msg.err.Error())
        m.answers = append(m.answers, errLine)
    } else {
        rendered := styles.RenderMarkdown(msg.result, m.width-4)
        answer := styles.AgentAnswer.Render("◆ Argus") + "\n" + rendered
        m.answers = append(m.answers, answer)
    }
    m.thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
    return m, nil


	// agent event — forward to thinking indicator
	case components.AgentEventMsg:
		cmd := m.thinking.Update(msg)
		cmds = append(cmds, cmd)

	// user submitted query or preset command
	case components.QuerySubmitMsg:
		return m, m.handleQuery(msg)

	// keyboard shortcuts
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.cancel()
			return m, func() tea.Msg { return SwitchToSourceSelect{} }
		}
	}

	// forward remaining messages to sub-components
	if m.logViewer != nil {
		logCmd := m.logViewer.Update(msg)
		cmds    = append(cmds, logCmd)
	}
	queryCmd    := m.queryBar.Update(msg)
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
	m.answers = append(m.answers, userLine)

	if msg.IsPreset {
		switch msg.Input {
		case "/stats":
			logs     := m.logViewer.Logs()
			out, err := m.orch.RunStats(context.Background(), logs)
			result   := ""
			if err != nil {
				result = "stats error: " + err.Error()
			} else {
				result = out.Result
			}
			return func() tea.Msg { return queryResultMsg{result: result} }

		case "/clear":
			m.answers = nil
			return nil

		case "/quit":
			m.cancel()
			return tea.Quit

		default:
			return func() tea.Msg {
				return queryResultMsg{result: "unknown command. available: /stats /clear /quit"}
			}
		}
	}

	// natural language — disable input while orchestrator runs
	m.queryBar.Disable()
	logs := m.logViewer.Logs()

	return tea.Batch(
		m.watchOrchestratorEvents(),        // forwards agent events to thinking indicator
		m.runOrchestrator(msg.Input, logs), // runs LLM query
	)
}

func (m ChatModel) runOrchestrator(query string, logs []agents.LogEntry) tea.Cmd {
	return func() tea.Msg {
		result, err := m.orch.Run(m.ctx, query, logs)
		return queryResultMsg{result: result, err: err}
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
	logHeight := int(float64(m.height) * 0.65)

	if m.logViewer == nil {
		// first time — create viewer with correct dimensions
		lv := components.NewLogViewer(m.width-4, logHeight)
		m.logViewer = &lv
	} else {
		// terminal resized — update dimensions
		m.logViewer.Resize(m.width-4, logHeight)
	}
}

// ── view ──────────────────────────────────────────────────────────────────

func (m ChatModel) View() string {
    if m.width == 0 || m.logViewer == nil {
        return "\n  " + styles.Muted.Render("initializing...")
    }

    var b strings.Builder

    // ── header — one line, no box ─────────────────────────────────────
    dot    := styles.StatusDot(m.target.Status == "running")
    source := styles.Brand.Render("argus")
    name   := styles.Title.Render(m.target.Name)
    image  := styles.Muted.Render(m.target.Image)
    keys   := styles.Muted.Render("esc back  /stats /clear /quit")
    header := fmt.Sprintf("  %s  %s %s  %s  %s\n",
        source, dot, name, image, keys)
    b.WriteString(header)
    b.WriteString(styles.HRuleStr(m.width) + "\n")

    // ── log panel ─────────────────────────────────────────────────────
    b.WriteString(m.logViewer.View())
    b.WriteString("\n")
    b.WriteString(styles.HRuleStr(m.width) + "\n")

    // ── conversation answers ──────────────────────────────────────────
    if len(m.answers) > 0 {
        b.WriteString("\n")
        b.WriteString(strings.Join(m.answers, "\n\n"))
        b.WriteString("\n\n")
        b.WriteString(styles.HRuleStr(m.width) + "\n")
    }

    // ── thinking indicator ────────────────────────────────────────────
    thinking := m.thinking.View()
    if thinking != "" {
        b.WriteString(thinking + "\n")
    }

    // ── query input ───────────────────────────────────────────────────
    b.WriteString("\n")
    b.WriteString(m.queryBar.View())
    b.WriteString("\n")

    return b.String()
}
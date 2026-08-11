package screens

import (
	"context"
	"fmt"
	"strings"
	"os"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/agents"
	filecollector "github.com/vishnuprasad2004/argus/internal/collectors/file"
	"github.com/vishnuprasad2004/argus/internal/tui/components"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
	"github.com/vishnuprasad2004/argus/internal/types"
)

type fileLogsLoadedMsg struct {
	logs []types.LogEntry
	err  error
}

type FileChatModel struct {
	width       int
	height      int
	initialized bool

	logViewer     *components.LogViewer
	queryBar      components.QueryBar
	thinking      components.ThinkingIndicator
	answerVP      viewport.Model
	answerContent []string
	answerReady   bool
	focusedPanel  int

	collector *filecollector.FileCollector
	opts      filecollector.FetchOptions
	orch      *agents.Orchestrator
	answers   []string

	ctx    context.Context
	cancel context.CancelFunc

	streamingAnswer string
}

func NewFileChatModel(
	collector *filecollector.FileCollector,
	fullFile bool,
	tailLines int,
	llm *agents.GeminiClient,
) FileChatModel {
	ctx, cancel := context.WithCancel(context.Background())
	return FileChatModel{
		collector: collector,
		opts: filecollector.FetchOptions{
			FullFile:  fullFile,
			TailLines: tailLines,
		},
		queryBar: components.NewQueryBar(),
		thinking: components.NewThinkingIndicator(),
		orch:     agents.NewOrchestrator(llm),
		ctx:      ctx,
		cancel:   cancel,
	}
}


func debugLog(format string, args ...any) {
    f, err := os.OpenFile("/tmp/argus-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return
    }
    defer f.Close()
    fmt.Fprintf(f, format+"\n", args...)
}

func (m FileChatModel) Init() tea.Cmd {
	debugLog("FileChatModel: Init() called")
	return tea.Batch(m.thinking.Init(), tea.WindowSize())
}

func (m FileChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		debugLog("FileChatModel: WindowSizeMsg %dx%d initialized=%v", msg.Width, msg.Height, m.initialized)
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		if !m.initialized {
			m.initialized = true
			debugLog("FileChatModel: calling loadFile()")
			return m, m.loadFile()
		}
		return m, nil

	case fileLogsLoadedMsg:
		debugLog("FileChatModel: fileLogsLoadedMsg err=%v logs=%d", msg.err, len(msg.logs))
		if msg.err != nil {
			m.answerContent = append(m.answerContent,
				styles.LogError.Render("✗ Failed to load file: "+msg.err.Error()))
			m.refreshAnswerVP()
			return m, nil
		}
		for _, entry := range msg.logs {
			m.logViewer.AppendLog(entry)
		}
		// show a summary line in answer panel
		summary := fmt.Sprintf("Loaded **%d lines** from `%s`",
			len(msg.logs), m.collector.Name())
		m.answerContent = append(m.answerContent,
			styles.AgentAnswer.Render("◆ Argus")+"\n"+
				styles.RenderMarkdown(summary, m.width-4))
		m.refreshAnswerVP()
		return m, nil

	case queryResultMsg:
		m.queryBar.Enable()
		if msg.err != nil {
			m.answerContent = append(m.answerContent,
				styles.LogError.Render("✗ Error: "+msg.err.Error()))
		} else {
			rendered := styles.RenderMarkdown(msg.result, m.width-4)
			m.answerContent = append(m.answerContent,
				styles.AgentAnswer.Render("◆ Argus")+"\n"+rendered)
		}
		m.refreshAnswerVP()
		m.thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
		return m, nil

	case components.AgentEventMsg:
    switch agents.EventType(msg.Type) {

    case agents.EventChunk:
        // append chunk to current streaming answer
        // if no streaming answer in progress, start one
        m.appendChunk(msg.Message)
        cmd := m.thinking.Update(msg)
        return m, tea.Batch(cmd, m.watchOrchestratorEvents())

    case agents.EventAnswer:
        // streaming done — finalize and re-enable input
        m.finalizeStreamingAnswer()
        m.queryBar.Enable()
        m.thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
        return m, nil

    default:
        cmd := m.thinking.Update(msg)
        return m, tea.Batch(cmd, m.watchOrchestratorEvents())
    }

	case components.QuerySubmitMsg:
		return m, m.handleQuery(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.focusedPanel == 0 {
				m.focusedPanel = 1
			} else {
				m.focusedPanel = 0
			}
			return m, nil
		case "esc":
			m.cancel()
			return m, func() tea.Msg { return SwitchToSourceSelect{} }
		}
	default:
    debugLog("FileChatModel: unhandled msg type=%T", msg)
	}

	// route scroll to focused panel
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

func (m FileChatModel) loadFile() tea.Cmd {
	return func() tea.Msg {
		logs, err := m.collector.FetchLogs(m.opts)
		return fileLogsLoadedMsg{logs: logs, err: err}
	}
}

func (m FileChatModel) handleQuery(msg components.QuerySubmitMsg) tea.Cmd {
	userLine := styles.Brand.Render("❯ you") + "\n" +
		styles.WrapText(msg.Input, m.width-4)
	m.answerContent = append(m.answerContent, userLine)
	m.refreshAnswerVP()

	if msg.IsPreset {
		switch msg.Input {
		case "/stats":
			logs := m.logViewer.Logs()
			out, err := m.orch.RunStats(context.Background(), logs)
			result := ""
			if err != nil {
				result = "stats error: " + err.Error()
			} else {
				result = out.Result
			}
			return func() tea.Msg { return queryResultMsg{result: result} }
		case "/clear":
			m.answerContent = nil
			m.answerVP.SetContent("")
			return nil
		case "/quit":
			m.cancel()
			return tea.Quit
		default:
			return func() tea.Msg {
				return queryResultMsg{result: "unknown command. try /stats /clear /quit"}
			}
		}
	}

	m.queryBar.Disable()
	logs := m.logViewer.Logs()
	return tea.Batch(
		m.watchOrchestratorEvents(),
		func() tea.Msg {
			result, err := m.orch.Run(m.ctx, msg.Input, logs)
			return queryResultMsg{result: result, err: err}
		},
	)
}

func (m FileChatModel) watchOrchestratorEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.orch.Events:
			return components.AgentEventMsg(event)
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *FileChatModel) refreshAnswerVP() {
	content := strings.Join(m.answerContent, "\n\n")
	m.answerVP.SetContent(content)
	m.answerVP.GotoBottom()
}

func (m *FileChatModel) relayout() {
	logHeight := int(float64(m.height) * 0.45)
	answerHeight := int(float64(m.height) * 0.35)

	if m.logViewer == nil {
		lv := components.NewLogViewer(m.width-4, logHeight)
		m.logViewer = &lv
	} else {
		m.logViewer.Resize(m.width-4, logHeight)
	}

	if !m.answerReady {
		m.answerVP = viewport.New(m.width-4, answerHeight)
		m.answerReady = true
	} else {
		m.answerVP.Width = m.width - 4
		m.answerVP.Height = answerHeight
	}
}

func (m FileChatModel) View() string {
	if m.width == 0 || m.logViewer == nil {
		return "\n  " + styles.Muted.Render("initializing...")
	}

	var b strings.Builder

	header := fmt.Sprintf("  %s  %s  %s  %s\n",
		styles.Brand.Render("argus"),
		styles.Title.Render(m.collector.Name()),
		styles.Muted.Render(m.collector.Path()),
		styles.Muted.Render("tab: switch panel  esc: back  /stats /clear /quit"),
	)
	b.WriteString(header)

	if m.focusedPanel == 0 {
		b.WriteString(styles.Brand.Render(strings.Repeat("─", m.width)) + "\n")
	} else {
		b.WriteString(styles.HRuleStr(m.width) + "\n")
	}
	b.WriteString(m.logViewer.View() + "\n")

	if m.focusedPanel == 1 {
		b.WriteString(styles.Brand.Render(strings.Repeat("─", m.width)) + "\n")
	} else {
		b.WriteString(styles.HRuleStr(m.width) + "\n")
	}

	if m.answerReady {
		if len(m.answerContent) == 0 {
			b.WriteString("  " + styles.Muted.Render("ask a question below...") + "\n")
		} else {
			b.WriteString(m.answerVP.View() + "\n")
		}
	}

	b.WriteString(styles.HRuleStr(m.width) + "\n")

	if t := m.thinking.View(); t != "" {
		b.WriteString(t + "\n")
	}

	b.WriteString("\n" + m.queryBar.View() + "\n")

	focused := map[int]string{0: "logs", 1: "answers"}[m.focusedPanel]
	shortcuts := []struct{ key, desc string }{
			{"↑↓",     "scroll"},
			{"tab",    "switch panel"},
			{"esc",    "back"},
			{"/stats", "error counts"},
			{"/clear", "clear chat"},
			{"/quit",  "quit"},
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

	b.WriteString(styles.HRuleStr(m.width) + "\n")
	b.WriteString("  " + keys.String() + "\n")
	b.WriteString("  " + descs.String() + "\n")
	b.WriteString("  " + styles.Muted.Render("panel: "+focused) + "\n")

	return b.String()
}

func (m *FileChatModel) appendChunk(chunk string) {
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
func (m *FileChatModel) finalizeStreamingAnswer() {
	if m.streamingAnswer == "" {
		return
	}
	last := len(m.answerContent) - 1
	rendered := styles.RenderMarkdown(m.streamingAnswer, m.width-4)
	m.answerContent[last] = styles.AgentAnswer.Render("◆ Argus") + "\n" + rendered
	m.streamingAnswer = "" // reset for next answer
	m.refreshAnswerVP()
}

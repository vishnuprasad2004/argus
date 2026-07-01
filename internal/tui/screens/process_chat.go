package screens

import (
    "context"
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/tmc/langchaingo/llms/googleai"
    "github.com/vishnuprasad2004/argus/agents"
    "github.com/vishnuprasad2004/argus/internal/collectors/process"
    "github.com/vishnuprasad2004/argus/internal/tui/components"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// ── message types ─────────────────────────────────────────────────────────

type procLogMsg     agents.LogEntry
type procExitedMsg  process.ProcessResult
type procStartedMsg struct {
    logCh    <-chan agents.LogEntry
    resultCh <-chan process.ProcessResult
    proc     *process.ProcessCollector
}

// ── model ─────────────────────────────────────────────────────────────────

type ProcessChatModel struct {
    width       int
    height      int
    initialized bool

    logViewer *components.LogViewer
    queryBar  components.QueryBar
    thinking  components.ThinkingIndicator

    command  string
    orch     *agents.Orchestrator
    answers  []string

    proc     *process.ProcessCollector
    logCh    <-chan agents.LogEntry
	resultCh <-chan process.ProcessResult
	exited   bool   // true once process has stopped

    ctx    context.Context
    cancel context.CancelFunc
}

func NewProcessChatModel(command string, llm *googleai.GoogleAI) ProcessChatModel {
    ctx, cancel := context.WithCancel(context.Background())
    return ProcessChatModel{
        command:  command,
        queryBar: components.NewQueryBar(),
        thinking: components.NewThinkingIndicator(),
        orch:     agents.NewOrchestrator(llm),
        ctx:      ctx,
        cancel:   cancel,
    }
}

func (m ProcessChatModel) Init() tea.Cmd {
    return tea.Batch(
        m.thinking.Init(),
        tea.WindowSize(), // forces immediate size — same fix as chat.go
    )
}

// ── update ────────────────────────────────────────────────────────────────

func (m ProcessChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.width  = msg.Width
        m.height = msg.Height
        m.relayout()

        if !m.initialized {
            m.initialized = true
            return m, m.startProcess()
        }
        return m, nil

    // process started — store channels, begin reading
    case procStartedMsg:
        m.proc     = msg.proc
        m.logCh    = msg.logCh
        m.resultCh = msg.resultCh
        return m, tea.Batch(
            m.waitForNextProcLog(),
            m.waitForProcExit(),
        )

    // one log line from the running process
    case procLogMsg:
        m.logViewer.AppendLog(agents.LogEntry(msg))
        if !m.exited {
            return m, m.waitForNextProcLog()
        }
        return m, nil

    // process exited — show stats automatically
    case procExitedMsg:
        m.exited = true
        result := process.ProcessResult(msg)

        var statusLine string
        if result.Err != nil {
            statusLine = styles.LogError.Render(fmt.Sprintf("✗ %s", result.Message))
        } else {
            statusLine = styles.LogWarn.Render(fmt.Sprintf("⚠ %s", result.Message))
        }
        warning := agents.LogEntry{Level: "WARN", Source: "argus", Message: result.Message}
        m.logViewer.AppendLog(warning)
        m.answers = append(m.answers, statusLine)

        // auto-run stats, no LLM cost
        return m, m.runAutoStats()

    case queryResultMsg:
        m.queryBar.Enable()
        if msg.err != nil {
            m.answers = append(m.answers, styles.LogError.Render("✗ Error: "+msg.err.Error()))
        } else {
            rendered := styles.RenderMarkdown(msg.result, m.width-4)
            m.answers = append(m.answers, styles.AgentAnswer.Render("◆ Argus")+"\n"+rendered)
        }
        m.thinking.Update(components.AgentEventMsg{Type: agents.EventAnswer})
        return m, nil

    case components.AgentEventMsg:
        cmd := m.thinking.Update(msg)
        cmds = append(cmds, cmd)

    case components.QuerySubmitMsg:
        return m, m.handleQuery(msg)

    case tea.KeyMsg:
        if msg.String() == "esc" {
            if m.proc != nil {
                m.proc.Kill()
            }
            m.cancel()
            return m, func() tea.Msg { return SwitchToSourceSelect{} }
        }
    }

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

func (m ProcessChatModel) startProcess() tea.Cmd {
    return func() tea.Msg {
        proc, err := process.NewProcessCollector()
        if err != nil {
            return queryResultMsg{err: fmt.Errorf("process: %w", err)}
        }
        logCh, resultCh, err := proc.Start(m.ctx, m.command)
        if err != nil {
            return queryResultMsg{err: fmt.Errorf("start: %w", err)}
        }
        return procStartedMsg{logCh: logCh, resultCh: resultCh, proc: proc}
    }
}

func (m ProcessChatModel) waitForNextProcLog() tea.Cmd {
    if m.logCh == nil {
        return nil
    }
    return func() tea.Msg {
        entry, ok := <-m.logCh
        if !ok {
            return nil // channel closed, process exit handled separately
        }
        return procLogMsg(entry)
    }
}

func (m ProcessChatModel) waitForProcExit() tea.Cmd {
    if m.resultCh == nil {
        return nil
    }
    return func() tea.Msg {
        result := <-m.resultCh
        return procExitedMsg(result)
    }
}

func (m ProcessChatModel) runAutoStats() tea.Cmd {
    return func() tea.Msg {
        logs := m.logViewer.Logs()
        out, err := m.orch.RunStats(context.Background(), logs)
        if err != nil {
            return queryResultMsg{err: err}
        }
        return queryResultMsg{result: out.Result}
    }
}

func (m ProcessChatModel) handleQuery(msg components.QuerySubmitMsg) tea.Cmd {
    userLine := styles.Brand.Render("❯ you") + "\n" + styles.WrapText(msg.Input, m.width-4)
    m.answers = append(m.answers, userLine)

    if msg.IsPreset {
        switch msg.Input {
        case "/stats":
            return m.runAutoStats()
        case "/clear":
            m.answers = nil
            return nil
        case "/quit":
            if m.proc != nil {
                m.proc.Kill()
            }
            m.cancel()
            return tea.Quit
        default:
            return func() tea.Msg {
                return queryResultMsg{result: "unknown command. available: /stats /clear /quit"}
            }
        }
    }

    m.queryBar.Disable()
    logs := m.logViewer.Logs()
    return tea.Batch(
        m.watchOrchestratorEvents(),
        m.runOrchestrator(msg.Input, logs),
    )
}

func (m ProcessChatModel) runOrchestrator(query string, logs []agents.LogEntry) tea.Cmd {
    return func() tea.Msg {
        result, err := m.orch.Run(m.ctx, query, logs)
        return queryResultMsg{result: result, err: err}
    }
}

func (m ProcessChatModel) watchOrchestratorEvents() tea.Cmd {
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

func (m *ProcessChatModel) relayout() {
    logHeight := int(float64(m.height) * 0.65)
    if m.logViewer == nil {
        lv := components.NewLogViewer(m.width-4, logHeight)
        m.logViewer = &lv
    } else {
        m.logViewer.Resize(m.width-4, logHeight)
    }
}

// ── view ──────────────────────────────────────────────────────────────────

func (m ProcessChatModel) View() string {
    if m.width == 0 || m.logViewer == nil {
        return "\n  " + styles.Muted.Render("initializing...")
    }

    var b strings.Builder

    status := "running"
    dot    := styles.StatusDot(!m.exited)
    if m.exited {
        status = "stopped"
    }
    header := fmt.Sprintf("  %s  %s %s  %s  %s\n",
        styles.Brand.Render("argus"),
        dot,
        styles.Title.Render(m.command),
        styles.Muted.Render(status),
        styles.Muted.Render("esc back  /stats /clear /quit"),
    )
    b.WriteString(header)
    b.WriteString(styles.HRuleStr(m.width) + "\n")
    b.WriteString(m.logViewer.View())
    b.WriteString("\n")
    b.WriteString(styles.HRuleStr(m.width) + "\n")

    if len(m.answers) > 0 {
        b.WriteString("\n" + strings.Join(m.answers, "\n\n") + "\n\n")
        b.WriteString(styles.HRuleStr(m.width) + "\n")
    }

    if t := m.thinking.View(); t != "" {
        b.WriteString(t + "\n")
    }

    b.WriteString("\n" + m.queryBar.View() + "\n")
    return b.String()
}
package components

import (
    "fmt"

    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/vishnuprasad2004/argus/agents"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

type LogViewer struct {
    viewport viewport.Model  // bubbles viewport = scrollable panel
    logs     []agents.LogEntry
    ready    bool            // false until we know terminal size
}

func NewLogViewer(width, height int) LogViewer {
    vp := viewport.New(width, height)
    vp.SetContent("Loading logs...")
    return LogViewer{
        viewport: vp,
        ready:    true,
    }
}

// AppendLog adds a new log entry and scrolls to bottom
func (l *LogViewer) AppendLog(entry agents.LogEntry) {
    l.logs = append(l.logs, entry)
    l.viewport.SetContent(l.renderLogs())
    l.viewport.GotoBottom() // auto scroll — like tail -f behaviour
}

// renderLogs turns []LogEntry into a styled string for the viewport
func (l *LogViewer) renderLogs() string {
    if len(l.logs) == 0 {
        return styles.Muted.Render("No logs yet...")
    }

    var out string
    for _, entry := range l.logs {
        // timestamp — always muted grey
        ts := styles.Muted.Render(entry.Timestamp.Format("15:04:05"))

        // level badge — colored by level
        level := styles.LogStyle(entry.Level).Render(fmt.Sprintf("%-5s", entry.Level))

        // source — muted
        source := styles.Muted.Render(entry.Source)

        // message — colored by level
        msg := styles.LogStyle(entry.Level).Render(entry.Message)

        out += fmt.Sprintf("%s  %s  %s  %s\n", ts, level, source, msg)
    }
    return out
}

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
    var cmd tea.Cmd
    l.viewport, cmd = l.viewport.Update(msg)
    return cmd
}

func (l *LogViewer) View() string {
    return l.viewport.View()
}

// Resize updates dimensions when terminal is resized
func (l *LogViewer) Resize(width, height int) {
    l.viewport.Width = width
    l.viewport.Height = height
    l.viewport.SetContent(l.renderLogs())
}

// Logs returns current log snapshot — for passing to orchestrator
func (l *LogViewer) Logs() []agents.LogEntry {
    return l.logs
}
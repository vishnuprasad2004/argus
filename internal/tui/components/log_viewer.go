package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
	"github.com/vishnuprasad2004/argus/internal/types"
)

type LogViewer struct {
    viewport viewport.Model  // bubbles viewport = scrollable panel
    logs     []types.LogEntry
    ready    bool            // false until we know terminal size

    following   bool // true = auto-scroll on new logs
    newLogCount int  // logs arrived while not following
}

func NewLogViewer(width, height int) LogViewer {
    vp := viewport.New(width, height)
    vp.SetContent("Loading logs...")
    return LogViewer{
        viewport:  vp,
        ready:     true,
        following: true, // start in follow mode
    }
}

// AppendLog adds a new log entry and scrolls to bottom
func (l *LogViewer) AppendLog(entry types.LogEntry) {
    l.logs = append(l.logs, entry)
    l.viewport.SetContent(l.renderLogs())

    if l.following {
        l.viewport.GotoBottom()
        l.newLogCount = 0 // reset counter when following
    } else {
        l.newLogCount++ // count what the user is missing
    }
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
    // detect scroll-up keys and disable follow
    if keyMsg, ok := msg.(tea.KeyMsg); ok {
        switch keyMsg.String() {
        case "up", "k", "pgup", "ctrl+b":
            // user is scrolling up — stop following
            if l.following {
                l.following = false
            }
        case "end", "G":
            // user jumped to bottom — resume following
            l.EnableFollow()
        }
    }

    var cmd tea.Cmd
    l.viewport, cmd = l.viewport.Update(msg)
    return cmd
}

func (l *LogViewer) View() string {
    pane := l.viewport.View()

    // show "↓ N new logs · press f to follow" when not following
    if !l.following && l.newLogCount > 0 {
        indicator := styles.Brand.Render(
            fmt.Sprintf("  ↓ %d new log", l.newLogCount))
        if l.newLogCount > 1 {
            indicator += styles.Brand.Render("s")
        }
        indicator += styles.Muted.Render(" · f to follow")
        return pane + "\n" + indicator
    }

    // show "FOLLOW" marker when following so user knows the mode
    if l.following {
        followBadge := styles.Muted.Render("  ↓ following")
        return pane + "\n" + followBadge
    }

    return pane
}

// Resize updates dimensions when terminal is resized
func (l *LogViewer) Resize(width, height int) {
    l.viewport.Width = width
    l.viewport.Height = height
    l.viewport.SetContent(l.renderLogs())
}

// Logs returns current log snapshot — for passing to orchestrator
func (l *LogViewer) Logs() []types.LogEntry {
    return l.logs
}

// ToggleFollow switches follow mode on or off
// called when user presses f
func (l *LogViewer) ToggleFollow() {
    l.following = !l.following
    if l.following {
        // jumping back to bottom — reset counter
        l.newLogCount = 0
        l.viewport.GotoBottom()
    }
}

// EnableFollow turns follow on and jumps to bottom
// called when user presses End
func (l *LogViewer) EnableFollow() {
    l.following = true
    l.newLogCount = 0
    l.viewport.GotoBottom()
}

// IsFollowing returns current follow state
func (l *LogViewer) IsFollowing() bool { return l.following }

// NewLogCount returns how many logs arrived while not following
func (l *LogViewer) NewLogCount() int { return l.newLogCount }
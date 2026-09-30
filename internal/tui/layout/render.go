package layout

import (
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// Divider returns a vertical divider string of the right height
// In wide mode this sits between the log pane and insights pane
func Divider(height int) string {
    line := styles.Subtle.Render("│")
    rows := make([]string, height)
    for i := range rows {
        rows[i] = line
    }
    return strings.Join(rows, "\n")
}

// TabBar renders the narrow-mode tab bar
// activeTab: 0=Logs 1=Insights 2=Metrics 3=Incidents
func TabBar(width, activeTab int) string {
    tabs := []struct {
        label    string
        key      string
        available bool
    }{
        {"Logs",      "1", true},
        {"Insights",  "2", true},
        {"Metrics",   "3", false}, // coming soon
        {"Incidents", "4", false}, // coming soon
    }

    var parts []string
    for i, t := range tabs {
        var rendered string
        if !t.available {
            rendered = styles.Muted.Render(t.key + " " + t.label)
        } else if i == activeTab {
            // active tab — accent color + underline
            rendered = lipgloss.NewStyle().
                Foreground(styles.ColorPrimary).
                Bold(true).
                Underline(true).
                Render(t.key + " " + t.label)
        } else {
            rendered = styles.Muted.Render(t.key + " " + t.label)
        }
        parts = append(parts, rendered)
    }

    bar := strings.Join(parts, styles.Muted.Render("  ·  "))

    // pad to full width
    return lipgloss.NewStyle().
        Width(width).
        Render(bar)
}

// JoinPanes combines left pane, divider, and right pane horizontally
// Each pane string must already be the correct height
func JoinPanes(left, divider, right string) string {
    return lipgloss.JoinHorizontal(lipgloss.Top, left, divider, right)
}
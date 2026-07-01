package screens

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// one item in the source menu
type sourceOption struct {
    label       string
    description string
    value       string // "docker", "process", "k8s"
    available   bool   // k8s will be false until implemented
}

type SourceSelectModel struct {
    options  []sourceOption
    cursor   int  // which item is highlighted
}

func NewSourceSelectModel() SourceSelectModel {
    return SourceSelectModel{
        cursor: 0,
        options: []sourceOption{
            {
                label:       "Docker Container",
                description: "Stream logs from a running or stopped container",
                value:       "docker",
                available:   true,
            },
            {
                label:       "Process",
                description: "Run a command (npm run dev, go run . etc) and capture its output",
                value:       "process",
                available:   true,
            },
            {
                label:       "Kubernetes Pod",
                description: "Stream logs from a pod in your cluster (coming soon)",
                value:       "k8s",
                available:   false, // greyed out until implemented
            },
        },
    }
}

func (m SourceSelectModel) Init() tea.Cmd { return nil }

func (m SourceSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {

        case "up", "k":   // k = vim up
            if m.cursor > 0 {
                m.cursor--
            }

        case "down", "j": // j = vim down
            if m.cursor < len(m.options)-1 {
                m.cursor++
            }

        case "enter":
            selected := m.options[m.cursor]
            if !selected.available {
                return m, nil // do nothing for unavailable options
            }
            // fire transition message up to root
            return m, func() tea.Msg {
                return SwitchToContainerSelect{Source: selected.value}
            }
        }
    }
    return m, nil
}

func (m SourceSelectModel) View() string {
    title := styles.Title.Render("Select Log Source")
    hint  := styles.Muted.Render("↑/↓ to move   enter to select")

    out := title + "\n" + hint + "\n\n"

    for i, opt := range m.options {
        var row string

        if !opt.available {
            // greyed out — not selectable
            row = styles.Muted.Render(fmt.Sprintf("  %s  %s (coming soon)",
                opt.label, opt.description))

        } else if i == m.cursor {
            // active/selected item
            cursor := styles.AgentTool.Render(styles.SelectorCursor)
            label  := styles.SelectorItemActive.Render(opt.label)
            desc   := styles.Muted.Render("  " + opt.description)
            row     = cursor + label + desc

        } else {
            // normal item
            label := styles.SelectorItem.Render(opt.label)
            desc  := styles.Muted.Render("  " + opt.description)
            row    = label + desc
        }

        out += row + "\n"
    }

    return out
}
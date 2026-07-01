package screens

import (
    "strings"

    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// SwitchToProcessChat — fires when user submits a command to run
type SwitchToProcessChat struct {
    Command string
}

type ProcessSetupModel struct {
    input textinput.Model
    err   string
}

func NewProcessSetupModel() ProcessSetupModel {
    ti := textinput.New()
    ti.Placeholder = "npm run dev"
    ti.Focus()
    ti.PromptStyle      = styles.Brand
    ti.TextStyle        = styles.Base
    ti.PlaceholderStyle = styles.Muted
    ti.Prompt           = "❯ "
    ti.CharLimit        = 200
    ti.Width            = 60

    return ProcessSetupModel{input: ti}
}

func (m ProcessSetupModel) Init() tea.Cmd {
    return textinput.Blink
}

func (m ProcessSetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "enter":
            cmd := strings.TrimSpace(m.input.Value())
            if cmd == "" {
                m.err = "please enter a command"
                return m, nil
            }
            return m, func() tea.Msg {
                return SwitchToProcessChat{Command: cmd}
            }
        case "esc":
            return m, func() tea.Msg { return SwitchToSourceSelect{} }
        }
    }

    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)
    return m, cmd
}

func (m ProcessSetupModel) View() string {
    var b strings.Builder

    b.WriteString("\n")
    b.WriteString(styles.HRuleStr(60) + "\n")
    b.WriteString("\n")
    b.WriteString("  " + styles.Title.Render("Run a process") + "\n\n")
    b.WriteString("  " + styles.Muted.Render(
        "Argus will run your command and capture all output.\n"+
        "  Make sure you're in your project directory.\n") + "\n")
    b.WriteString(styles.HRuleStr(60) + "\n\n")
    b.WriteString("  " + styles.Muted.Render("Command") + "\n")
    b.WriteString("  " + m.input.View() + "\n\n")

    if m.err != "" {
        b.WriteString("  " + styles.LogError.Render("✗ "+m.err) + "\n")
    }

    b.WriteString(styles.HRuleStr(60) + "\n")
    b.WriteString("  " + styles.Muted.Render("enter run   esc back") + "\n")

    return b.String()
}
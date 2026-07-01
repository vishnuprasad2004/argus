package components

import (
    "strings"

    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// QuerySubmitMsg is fired when user hits enter
// chat.go listens for this
type QuerySubmitMsg struct {
    Input   string
    IsPreset bool   // true if starts with /
}

type QueryBar struct {
    input    textinput.Model
    disabled bool  // true while agent is thinking
}

func NewQueryBar() QueryBar {
    ti := textinput.New()
    ti.Placeholder = "Ask anything about your logs, or type /stats /quit..."
    ti.Focus()  // start focused so user can type immediately

    // style the input itself
    ti.PromptStyle    = styles.AgentTool   // "> " prompt in blue
    ti.TextStyle      = styles.Base
    ti.PlaceholderStyle = styles.Muted
    ti.Prompt         = "> "
    ti.CharLimit      = 500

    return QueryBar{input: ti}
}

func (q *QueryBar) Update(msg tea.Msg) tea.Cmd {
    if q.disabled {
        return nil // ignore all input while agent is thinking
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "enter" {
            val := strings.TrimSpace(q.input.Value())
            if val == "" {
                return nil
            }
            q.input.SetValue("") // clear input after submit

            // fire submit message up to chat.go
            return func() tea.Msg {
                return QuerySubmitMsg{
                    Input:    val,
                    IsPreset: strings.HasPrefix(val, "/"),
                }
            }
        }
    }

    var cmd tea.Cmd
    q.input, cmd = q.input.Update(msg)
    return cmd
}

func (q *QueryBar) View() string {
    return styles.InputBar.Render(q.input.View())
}

// Disable prevents input while agent is thinking
func (q *QueryBar) Disable() { q.disabled = true;  q.input.Blur() }
func (q *QueryBar) Enable()  { q.disabled = false; q.input.Focus() }
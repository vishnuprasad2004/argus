package components

import (
	"fmt"
	"math/rand"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
	"strings"
)

// fun verbs shown while agent is working
var thinkingVerbs = []string{
    "Noodling",
    "Pondering",
    "Ruminating",
    "Cogitating",
    "Deliberating",
    "Shenaniganing",
    "Discombobulating",
    "Combobulating",
    "Contemplating",
    "Percolating",
    "Marinating",
    "Stewing",
    "Brewing",
}

type ThinkingIndicator struct {
	spinner spinner.Model
	events  []string // history of agent events shown
	visible bool     // only show while agent is running
	currentVerb string // picked once per query, stays stable during thinking
}
// ⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷
func NewThinkingIndicator() ThinkingIndicator {
    sp := spinner.New()
    sp.Spinner = spinner.MiniDot
    sp.Style   = styles.Muted

    return ThinkingIndicator{
        spinner:     sp,
        currentVerb: randomVerb(),
    }
}

func randomVerb() string {
    return thinkingVerbs[rand.Intn(len(thinkingVerbs))]
}

// AgentEventMsg wraps AgentEvent so bubbletea can route it
// sent from goroutine watching orch.Events channel
type AgentEventMsg agents.AgentEvent

func (t *ThinkingIndicator) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {

	case AgentEventMsg:
		switch agents.EventType(msg.Type) {
		case agents.EventToolCall:
			line := styles.AgentTool.Render(fmt.Sprintf("⚙  %s: %s", msg.Tool, msg.Message))
			t.events = append(t.events, line)
			t.visible = true
			t.currentVerb = randomVerb() // new verb per tool call
			line = fmt.Sprintf("%s  %s",
					styles.Brand.Render(t.currentVerb),
					styles.Muted.Render(msg.Tool),
			)
			t.events = append(t.events, line)

		case agents.EventAnswer:
			t.visible = false // hide spinner when answer arrives
			t.events = nil    // clear events

		case agents.EventWarning:
			line := styles.LogWarn.Render(fmt.Sprintf("⚠  %s", msg.Message))
			t.events = append(t.events, line)

		case agents.EventError:
			line := styles.LogError.Render(fmt.Sprintf("✗  %s", msg.Message))
			t.events = append(t.events, line)
			t.visible = false
		}
		return nil
	}

	// always tick spinner so animation is smooth
	var cmd tea.Cmd
	t.spinner, cmd = t.spinner.Update(msg)
	return cmd
}

func (t *ThinkingIndicator) View() string {
	if !t.visible && len(t.events) == 0 {
		return ""
	}

	var lines []string
	for _, e := range t.events {
		lines = append(lines, "  "+e)
	}
	if t.visible {
		// "⠋ Calling log_analysis..." — compact, inline
		lines = append(lines,
			"  "+t.spinner.View()+" "+styles.Muted.Render("thinking..."))
	}

	return strings.Join(lines, "\n")
}

func (t *ThinkingIndicator) Init() tea.Cmd {
	return t.spinner.Tick
}

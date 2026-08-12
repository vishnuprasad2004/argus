package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/internal/config"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
)

type wizardStep int

const (
	wizardStepWelcome wizardStep = iota
	wizardStepAPIKey
	wizardStepModel
	wizardStepSaving
	wizardStepDone
)

// WizardDoneMsg fires when wizard completes — root switches to normal flow
type WizardDoneMsg struct{}

type wizardSavedMsg struct{ err error }

type SetupWizardModel struct {
	step     wizardStep
	apiInput textinput.Model
	cursor   int // model selector cursor
	models   []config.ModelOption
	apiKey   string
	model    string
	err      string
	spinner  spinner.Model
	width    int
}

func NewSetupWizardModel() SetupWizardModel {
	ti := textinput.New()
	ti.Placeholder = "AIza..."
	ti.Focus()
	ti.PromptStyle      = styles.Brand
	ti.TextStyle        = styles.Base
	ti.PlaceholderStyle = styles.Muted
	ti.Prompt           = "❯ "
	ti.CharLimit        = 200
	ti.Width            = 55
	ti.EchoMode        = textinput.EchoPassword // hide key while typing

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style   = styles.Muted

	return SetupWizardModel{
		step:    wizardStepWelcome,
		apiInput: ti,
		models:  config.Models(),
		spinner: sp,
	}
}

func (m SetupWizardModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick, tea.WindowSize())
}

func (m SetupWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case wizardSavedMsg:
		if msg.err != nil {
			m.err  = msg.err.Error()
			m.step = wizardStepAPIKey // go back on error
			return m, nil
		}
		m.step = wizardStepDone
		// small delay so user sees "saved!" before switching
		return m, func() tea.Msg { return WizardDoneMsg{} }

	case tea.KeyMsg:
		switch m.step {

		case wizardStepWelcome:
			if msg.String() == "enter" || msg.String() == " " {
				m.step = wizardStepAPIKey
			}

		case wizardStepAPIKey:
			switch msg.String() {
			case "enter":
				key := strings.TrimSpace(m.apiInput.Value())
				if key == "" {
					m.err = "API key cannot be empty"
					return m, nil
				}
				// if !strings.HasPrefix(key, "AIza") {
				// 	m.err = "Gemini API keys start with 'AIza' — double check yours"
				// 	return m, nil
				// }
				m.apiKey = key
				m.err    = ""
				m.step   = wizardStepModel
				return m, nil
			case "esc":
				// allow going back to welcome
				m.step = wizardStepWelcome
				return m, nil
			}

		case wizardStepModel:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.models)-1 {
					m.cursor++
				}
			case "enter":
				m.model = m.models[m.cursor].ID
				m.step  = wizardStepSaving
				return m, m.saveConfig()
			case "esc":
				m.step = wizardStepAPIKey
				return m, nil
			}
		}
	}

	// update inputs and spinner
	var cmd tea.Cmd
	if m.step == wizardStepAPIKey {
		m.apiInput, cmd = m.apiInput.Update(msg)
	}
	if m.step == wizardStepSaving {
		m.spinner, cmd = m.spinner.Update(msg)
	}
	return m, cmd
}

func (m SetupWizardModel) saveConfig() tea.Cmd {
	return func() tea.Msg {
		err := config.Save(m.apiKey, m.model)
		return wizardSavedMsg{err: err}
	}
}

func (m SetupWizardModel) View() string {
	width := m.width
	if width == 0 {
		width = 80
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styles.HRuleStr(width) + "\n\n")

	switch m.step {

	case wizardStepWelcome:
		b.WriteString("  " + styles.Brand.Render("Welcome to Argus") + "\n\n")
		b.WriteString("  " + styles.Base.Render("AI-powered log analysis for SREs and developers.") + "\n")
		b.WriteString("  " + styles.Muted.Render("This wizard will set up your configuration in ~/.argus/config.yaml") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("You'll need a free Gemini API key to get started.") + "\n")
		b.WriteString("  " + styles.Muted.Render("Get one at: ") +
			styles.Brand.Render("https://aistudio.google.com") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n")
		b.WriteString("  " + styles.Muted.Render("press enter to continue") + "\n")

	case wizardStepAPIKey:
		b.WriteString("  " + styles.Brand.Render("Step 1 of 2 — Gemini API Key") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("Paste your API key below. It will be saved to ~/.argus/config.yaml") + "\n")
		b.WriteString("  " + styles.Muted.Render("(stored with 600 permissions — only you can read it)") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n\n")
		b.WriteString("  " + styles.Muted.Render("API Key") + "\n")
		b.WriteString("  " + m.apiInput.View() + "\n\n")
		if m.err != "" {
			b.WriteString("  " + styles.LogError.Render("✗ "+m.err) + "\n\n")
		}
		b.WriteString(styles.HRuleStr(width) + "\n")
		b.WriteString("  " + styles.Muted.Render("enter confirm   esc back") + "\n")

	case wizardStepModel:
		b.WriteString("  " + styles.Brand.Render("Step 2 of 2 — Choose a Model") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("You can change this later in ~/.argus/config.yaml") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n\n")

		for i, opt := range m.models {
			if i == m.cursor {
				cursor := styles.Brand.Render("❯ ")
				label  := styles.SelectorItemActive.Render(opt.Label)
				id     := styles.Muted.Render("  " + opt.ID)
				desc   := styles.Muted.Render("\n    " + opt.Description)
				b.WriteString("  " + cursor + label + id + desc + "\n\n")
			} else {
				label := styles.SelectorItem.Render(opt.Label)
				id    := styles.Muted.Render("  " + opt.ID)
				b.WriteString("    " + label + id + "\n\n")
			}
		}

		b.WriteString(styles.HRuleStr(width) + "\n")
		b.WriteString("  " + styles.Muted.Render("↑↓ navigate   enter select   esc back") + "\n")

	case wizardStepSaving:
		b.WriteString("  " + m.spinner.View() + " " +
			styles.Muted.Render("Saving configuration...") + "\n")

	case wizardStepDone:
		b.WriteString("  " + styles.Brand.Render("✓ All set!") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("Config saved to ~/.argus/config.yaml") + "\n")
		b.WriteString("  " + styles.Muted.Render("Launching Argus...") + "\n")
	}

	return b.String()
}
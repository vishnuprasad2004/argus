package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/internal/config"
	"github.com/vishnuprasad2004/argus/internal/memory"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
)

type wizardStep int

const (
	wizardStepWelcome wizardStep = iota
	wizardStepAPIKey
	wizardStepModel
	wizardStepStack
	wizardStepServices
	wizardStepPrefs
	wizardStepSaving
	wizardStepDone
)

// WizardDoneMsg fires when wizard completes — root switches to normal flow
type WizardDoneMsg struct{}

type wizardSavedMsg struct{ err error }

type SetupWizardModel struct {
	step          wizardStep
	apiInput      textinput.Model
	stackInput    textinput.Model
	servicesInput textinput.Model
	prefsInput    textinput.Model
	cursor        int // model selector cursor
	models        []config.ModelOption
	apiKey        string
	model         string
	err           string
	spinner       spinner.Model
	width         int
}

func NewSetupWizardModel() SetupWizardModel {
	ti := textinput.New()
	ti.Placeholder = "AIza..."
	ti.Focus()
	ti.PromptStyle = styles.Brand
	ti.TextStyle = styles.Base
	ti.PlaceholderStyle = styles.Muted
	ti.Prompt = "❯ "
	ti.CharLimit = 200
	ti.Width = 55
	ti.EchoMode = textinput.EchoPassword // hide key while typing

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = styles.Muted

	makeInput := func(placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.PromptStyle = styles.Brand
		ti.TextStyle = styles.Base
		ti.PlaceholderStyle = styles.Muted
		ti.Prompt = "❯ "
		ti.CharLimit = 200
		ti.Width = 55
		return ti
	}

	return SetupWizardModel{
		step:          wizardStepWelcome,
		apiInput:      ti,
		models:        config.Models(),
		stackInput:    makeInput("Node.js, Python, Go, MongoDB..."),
		servicesInput: makeInput("nginx, payments-api, auth-service..."),
		prefsInput:    makeInput("always suggest kubectl commands..."),
		spinner:       sp,
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
			m.err = msg.err.Error()
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
				m.err = ""
				m.step = wizardStepModel
				return m, nil
			case "esc":
				// allow going back to welcome
				m.step = wizardStepWelcome
				return m, nil
			}

		case wizardStepStack:
			switch msg.String() {
			case "enter":
				m.step = wizardStepServices
				m.stackInput.Blur()
				m.servicesInput.Focus()
			case "esc":
				m.step = wizardStepModel
			}

		case wizardStepServices:
			switch msg.String() {
			case "enter":
				m.step = wizardStepPrefs
				m.servicesInput.Blur()
				m.prefsInput.Focus()
			case "esc":
				m.step = wizardStepStack
			}

		case wizardStepPrefs:
			switch msg.String() {
			case "enter":
				m.step = wizardStepSaving
				return m, m.saveAll() // saves config + agent.md
			case "esc":
				m.step = wizardStepServices
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
				m.step = wizardStepSaving
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

	case wizardStepStack:
		b.WriteString("  " + styles.Brand.Render("Step 3 of 5 — Your Stack") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("What languages, frameworks, and databases do you use?") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n\n")
		b.WriteString("  " + m.stackInput.View() + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n")
		b.WriteString("  " + styles.Muted.Render("enter continue   esc back") + "\n")

	case wizardStepServices:
		b.WriteString("  " + styles.Brand.Render("Step 4 of 5 — Your Services") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("List your services, separated by commas") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n\n")
		b.WriteString("  " + m.servicesInput.View() + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n")
		b.WriteString("  " + styles.Muted.Render("enter continue   esc back") + "\n")

	case wizardStepPrefs:
		b.WriteString("  " + styles.Brand.Render("Step 5 of 5 — Preferences") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("Any preferences for how Argus should help? (optional)") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n\n")
		b.WriteString("  " + m.prefsInput.View() + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n")
		b.WriteString("  " + styles.Muted.Render("enter finish   esc back") + "\n")

	case wizardStepModel:
		b.WriteString("  " + styles.Brand.Render("Step 2 of 2 — Choose a Model") + "\n\n")
		b.WriteString("  " + styles.Muted.Render("You can change this later in ~/.argus/config.yaml") + "\n\n")
		b.WriteString(styles.HRuleStr(width) + "\n\n")

		for i, opt := range m.models {
			if i == m.cursor {
				cursor := styles.Brand.Render("❯ ")
				label := styles.SelectorItemActive.Render(opt.Label)
				id := styles.Muted.Render("  " + opt.ID)
				desc := styles.Muted.Render("\n    " + opt.Description)
				b.WriteString("  " + cursor + label + id + desc + "\n\n")
			} else {
				label := styles.SelectorItem.Render(opt.Label)
				id := styles.Muted.Render("  " + opt.ID)
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

func (m SetupWizardModel) saveAll() tea.Cmd {
	return func() tea.Msg {
		// save config.yaml
		if err := config.Save(m.apiKey, m.model); err != nil {
			return wizardSavedMsg{err: err}
		}

		// build and save agent.md from wizard answers
		store, err := memory.NewStore()
		if err != nil {
			return wizardSavedMsg{err: err}
		}

		agentContent := buildAgentMd(
			m.stackInput.Value(),
			m.servicesInput.Value(),
			m.prefsInput.Value(),
		)
		if err := store.WriteAgent(agentContent); err != nil {
			return wizardSavedMsg{err: err}
		}

		return wizardSavedMsg{err: nil}
	}
}

func buildAgentMd(stack, services, prefs string) string {
	var b strings.Builder
	b.WriteString("# Argus Agent Context\n")
	b.WriteString("> Auto-generated by setup wizard. Edit freely.\n\n")

	if stack != "" {
		b.WriteString("## My Stack\n")
		b.WriteString(stack + "\n\n")
	}

	if services != "" {
		b.WriteString("## Known Services\n")
		// split by comma and format as list
		for _, svc := range strings.Split(services, ",") {
			svc = strings.TrimSpace(svc)
			if svc != "" {
				b.WriteString("- " + svc + "\n")
			}
		}
		b.WriteString("\n")
	}

	if prefs != "" {
		b.WriteString("## Preferences\n")
		b.WriteString(prefs + "\n\n")
	}

	b.WriteString("## Team Context\n\n")
	return b.String()
}
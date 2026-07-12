package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/vishnuprasad2004/argus/internal/tui/screens"
)

type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenSourceSelect
	ScreenContainerSelect
	ScreenProcessSetup   // ← was missing
	ScreenProcessChat
	ScreenChat
)

type RootModel struct {
	screen int
	width  int
	height int

	welcome         screens.WelcomeModel
	sourceSelect    screens.SourceSelectModel
	containerSelect screens.ContainerSelectModel
	processSetup    screens.ProcessSetupModel  // ← was missing
	processChat     screens.ProcessChatModel
	chat            screens.ChatModel
	llm             *googleai.GoogleAI
}

func NewRootModel(llm *googleai.GoogleAI) RootModel {
	return RootModel{
		screen:  int(ScreenWelcome),
		welcome: screens.NewWelcomeModel(),
		llm:     llm,
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		m.welcome.Init(),
		tea.WindowSize(),
	)
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width  = msg.Width
		m.height = msg.Height
		// forward resize to whichever screen is active
		switch Screen(m.screen) {
		case ScreenChat:
			newModel, cmd := m.chat.Update(msg)
			m.chat = newModel.(screens.ChatModel)
			return m, cmd
		case ScreenProcessChat:
			newModel, cmd := m.processChat.Update(msg)
			m.processChat = newModel.(screens.ProcessChatModel)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	// ── screen transitions ────────────────────────────────────────────

	case screens.SwitchToSourceSelect:
		m.screen = int(ScreenSourceSelect)
		m.sourceSelect = screens.NewSourceSelectModel()
		return m, m.sourceSelect.Init()

	case screens.SwitchToContainerSelect:
		// ← THE KEY FIX — route process separately
		if msg.Source == "process" {
			m.screen = int(ScreenProcessSetup)
			m.processSetup = screens.NewProcessSetupModel()
			return m, m.processSetup.Init()
		}
		m.screen = int(ScreenContainerSelect)
		m.containerSelect = screens.NewContainerSelectModel(msg.Source)
		return m, m.containerSelect.Init()

	case screens.SwitchToProcessChat:
		m.screen = int(ScreenProcessChat)
		m.processChat = screens.NewProcessChatModel(msg.Command, m.llm)
		return m, m.processChat.Init()

	case screens.SwitchToChat:
		m.screen = int(ScreenChat)
		m.chat = screens.NewChatModel(msg.Target, m.llm)
		return m, m.chat.Init()
	}

	// ── delegate to active screen ─────────────────────────────────────

	switch Screen(m.screen) {
	case ScreenWelcome:
		newModel, cmd := m.welcome.Update(msg)
		m.welcome = newModel.(screens.WelcomeModel)
		return m, cmd

	case ScreenSourceSelect:
		newModel, cmd := m.sourceSelect.Update(msg)
		m.sourceSelect = newModel.(screens.SourceSelectModel)
		return m, cmd

	case ScreenContainerSelect:
		newModel, cmd := m.containerSelect.Update(msg)
		m.containerSelect = newModel.(screens.ContainerSelectModel)
		return m, cmd

	case ScreenProcessSetup:  // ← was missing
		newModel, cmd := m.processSetup.Update(msg)
		m.processSetup = newModel.(screens.ProcessSetupModel)
		return m, cmd

	case ScreenProcessChat:
		newModel, cmd := m.processChat.Update(msg)
		m.processChat = newModel.(screens.ProcessChatModel)
		return m, cmd

	case ScreenChat:
		newModel, cmd := m.chat.Update(msg)
		m.chat = newModel.(screens.ChatModel)
		return m, cmd
	}

	return m, nil
}

func (m RootModel) View() string {
	switch Screen(m.screen) {
	case ScreenWelcome:
		return m.welcome.View()
	case ScreenSourceSelect:
		return m.sourceSelect.View()
	case ScreenContainerSelect:
		return m.containerSelect.View()
	case ScreenProcessSetup:  // ← was missing
		return m.processSetup.View()
	case ScreenProcessChat:
		return m.processChat.View()
	case ScreenChat:
		return m.chat.View()
	}
	return ""
}
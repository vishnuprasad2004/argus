package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/vishnuprasad2004/argus/internal/tui/screens"
)

// Screen constants — which screen is active
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenSourceSelect
	ScreenContainerSelect
	ScreenChat
	ScreenProcessChat
)

// RootModel is the top-level bubbletea model
// Think of it like the root component in React
type RootModel struct {
	screen Screen // which screen is showing
	width  int    // terminal width — passed to all screens
	height int    // terminal height

	// sub-models — one per screen
	welcome         screens.WelcomeModel
	sourceSelect    screens.SourceSelectModel
	containerSelect screens.ContainerSelectModel
	chat            screens.ChatModel
	processChat     screens.ProcessChatModel
	llm             *googleai.GoogleAI
}

func NewRootModel(llm *googleai.GoogleAI) RootModel {
	return RootModel{
		screen:  ScreenWelcome,
		welcome: screens.NewWelcomeModel(),
		llm:     llm,
	}
}

// Init runs once on startup
func (m RootModel) Init() tea.Cmd {
    return tea.Batch(
        m.welcome.Init(),
        tea.WindowSize(), // ← this fires a real WindowSizeMsg immediately on start
    )
}

// Update handles all messages — routes to active screen
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// terminal resize — update dimensions everywhere
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.screen == ScreenChat {
        newModel, cmd := m.chat.Update(msg)
        m.chat = newModel.(screens.ChatModel)
        return m, cmd
    }

	case screens.SwitchToProcessChat:
    m.screen = ScreenProcessChat
    m.processChat = screens.NewProcessChatModel(msg.Command, m.llm)
    return m, m.processChat.Init()

	// ctrl+c anywhere = quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// screen transition messages — screens tell root to switch
		// in RootModel.Update, add this case alongside existing switch cases:
	case screens.SwitchToSourceSelect:
		m.screen = ScreenSourceSelect
		m.sourceSelect = screens.NewSourceSelectModel()
		return m, m.sourceSelect.Init()

	case screens.SwitchToContainerSelect:
		m.screen = ScreenContainerSelect
		m.containerSelect = screens.NewContainerSelectModel(msg.Source)
		return m, m.containerSelect.Init()

	case screens.SwitchToChat:
		m.screen = ScreenChat
		// msg.Target is already docker.ContainerTarget — no assertion needed
		m.chat = screens.NewChatModel(msg.Target, m.llm)
		return m, m.chat.Init()

	}

	// delegate update to active screen
	switch m.screen {
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

// View renders the active screen
func (m RootModel) View() string {
	switch m.screen {
	case ScreenWelcome:
		return m.welcome.View()
	case ScreenSourceSelect:
		return m.sourceSelect.View()
	case ScreenContainerSelect:
		return m.containerSelect.View()
	case ScreenProcessChat:
    return m.processChat.View()
	case ScreenChat:
		return m.chat.View()
	}
	return ""
}

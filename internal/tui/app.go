package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/config"
	"github.com/vishnuprasad2004/argus/internal/tui/screens"
)

type Screen int

const (
	ScreenWizard	Screen = iota
	ScreenWelcome
	ScreenSourceSelect
	ScreenContainerSelect
	ScreenProcessSetup
	ScreenProcessChat
	ScreenFileSetup 
  ScreenFileChat
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
	wizard screens.SetupWizardModel

	fileSetup screens.FileSetupModel
	fileChat  screens.FileChatModel

	chat            screens.ChatModel
	llm             *agents.GeminiClient
}

func NewRootModel(llm *agents.GeminiClient) RootModel {
	// show wizard on first run, welcome screen otherwise
	if config.IsFirstRun() {
		return RootModel{
			screen: int(ScreenWizard),
			wizard: screens.NewSetupWizardModel(),
			llm:    llm,
		}
	}

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
		case ScreenFileChat:                         
      newModel, cmd := m.fileChat.Update(msg)
      m.fileChat = newModel.(screens.FileChatModel)
      return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	// ── screen transitions ────────────────────────────────────────────

	case screens.WizardDoneMsg:
		m.screen  = int(ScreenWelcome)
		m.welcome = screens.NewWelcomeModel()
		return m, m.welcome.Init()

	case screens.SwitchToSourceSelect:
		m.screen = int(ScreenSourceSelect)
		m.sourceSelect = screens.NewSourceSelectModel()
		return m, m.sourceSelect.Init()

	case screens.SwitchToContainerSelect:
    switch msg.Source {
    case "process":
        m.screen = int(ScreenProcessSetup)
        m.processSetup = screens.NewProcessSetupModel()
        return m, m.processSetup.Init()
    case "file":           // ← new
        m.screen = int(ScreenFileSetup)
        m.fileSetup = screens.NewFileSetupModel()
        return m, m.fileSetup.Init()
    default:
        m.screen = int(ScreenContainerSelect)
        m.containerSelect = screens.NewContainerSelectModel(msg.Source)
        return m, m.containerSelect.Init()
    }

	case screens.SwitchToFileChat:
    m.screen = int(ScreenFileChat)
    m.fileChat = screens.NewFileChatModel(
        msg.Collector, msg.FullFile, msg.TailLines, m.llm)
    return m, m.fileChat.Init()


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
	case ScreenWizard:
		newModel, cmd := m.wizard.Update(msg)
		m.wizard = newModel.(screens.SetupWizardModel)
		return m, cmd
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
	
	case ScreenFileSetup:
    newModel, cmd := m.fileSetup.Update(msg)
    m.fileSetup = newModel.(screens.FileSetupModel)
    return m, cmd

	case ScreenFileChat:
		newModel, cmd := m.fileChat.Update(msg)
		m.fileChat = newModel.(screens.FileChatModel)
		return m, cmd
	}

	return m, nil
}

func (m RootModel) View() string {
	switch Screen(m.screen) {
	case ScreenWizard:
		return m.wizard.View()
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
	case ScreenFileSetup:
    return m.fileSetup.View()
	case ScreenFileChat:
    return m.fileChat.View()
	}
	return ""
}
package screens

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
)

type WelcomeModel struct{}

func NewWelcomeModel() WelcomeModel {
	return WelcomeModel{}
}

func (m WelcomeModel) Init() tea.Cmd {
	return nil
}

func (m WelcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			// tell root to switch to source select
			return m, func() tea.Msg { return SwitchToSourceSelect{} }
		}
	}
	return m, nil
}

// func (m WelcomeModel) View() string {
// 	banner := styles.Title.Render(`
//  █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
// ██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
// ███████║██████╔╝██║  ███╗██║   ██║███████╗
// ██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
// ██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
// ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝`)

// 	tagline := styles.Muted.Render("AI-powered log analysis for SREs and developers")
// 	prompt := styles.Base.Render("Press [enter] to start")

// 	return banner + "\n\n" + tagline + "\n\n" + prompt
// }

func (m WelcomeModel) View() string {
    // tight, no padding — like claude code's startup
    	banner := styles.Title.Render(`
 █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
███████║██████╔╝██║  ███╗██║   ██║███████╗
██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝`)

    version := styles.Muted.Render("v0.1.0 — AI-powered log analysis")
		link := styles.Link.Render("https://github.com/vishnuprasad2004/argus")
		short_description := styles.Muted.Render("Welcome to Argus! This tool helps you analyze logs using AI through your CLI itself.\nYou can check out the source code at ") + link

    tip := styles.Muted.Render("Press [enter] to continue ...")

    // no box, no border — just text, aligned left
    return fmt.Sprintf("\n  %s  \n%s\n%s\n\n  %s\n", banner, version, short_description, tip)
}
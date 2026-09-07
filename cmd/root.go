package cmd

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/vishnuprasad2004/argus/agents"
	"github.com/vishnuprasad2004/argus/internal/config"
	"github.com/vishnuprasad2004/argus/internal/tui"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "argus",
	Short: "AI-powered log analysis for SREs and developers",
	Long: `
 █████╗ ██████╗  ██████╗ ██╗   ██╗███████╗
██╔══██╗██╔══██╗██╔════╝ ██║   ██║██╔════╝
███████║██████╔╝██║  ███╗██║   ██║███████╗
██╔══██║██╔══██╗██║   ██║██║   ██║╚════██║
██║  ██║██║  ██║╚██████╔╝╚██████╔╝███████║
╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝  ╚═════╝ ╚══════╝

Argus — AI-powered log analysis for SREs and developers.
Analyze Docker containers, running processes, and Kubernetes pods.`,

	// SilenceUsage stops cobra printing usage on every error
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		// try to load config — if missing key, wizard handles it
		cfg, err := config.Load()
    if err != nil {
			// don't launch TUI with nil llm
			// IsFirstRun check handles wizard path
			if !config.IsFirstRun() {
					return fmt.Errorf("config error: %w\n\nRun argus again to go through setup.", err)
			}
			// first run — launch wizard with nil llm, wizard will init it
			p := tea.NewProgram(
					tui.NewRootModel(nil),
					tea.WithAltScreen(),
			)
			_, err := p.Run()
			return err
    }

    // config loaded — init LLM
    llm, err := agents.CreateAgentWithConfig(cfg)
		if llm == nil {
    	fmt.Println("DEBUG: llm is nil before launching TUI")
		}
    if err != nil {
      return fmt.Errorf("failed to connect to Gemini: %w", err)
    }

		p := tea.NewProgram(
			tui.NewRootModel(llm),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		return nil
	},
}

// Execute is called by main.go — single entry point
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already printed the error
		os.Exit(1)
	}
}
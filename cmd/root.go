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

		var llm *agents.GeminiClient

		if err != nil {
			// config missing or no key — launch with nil LLM
			// wizard will collect the key and restart
			llm = nil
		} else {
			llm, err = agents.CreateAgentWithConfig(cfg)
			if err != nil {
				return fmt.Errorf("failed to init Gemini: %w", err)
			}
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
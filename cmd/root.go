package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/vishnuprasad2004/argus/internal/config"
	"github.com/vishnuprasad2004/argus/internal/tui"
	"github.com/tmc/langchaingo/llms/googleai"
	"context"
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
		// load config — fails fast with helpful message if key missing
		cfg, err := config.Load()
		if err != nil {
			return err // cobra prints this cleanly
		}

		// init Gemini with config values
		llm, err := googleai.New(
			context.Background(),
			googleai.WithAPIKey(cfg.GeminiAPIKey),
			googleai.WithDefaultModel(cfg.Model),
		)
		if err != nil {
			return fmt.Errorf("failed to init Gemini: %w", err)
		}

		// start TUI
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
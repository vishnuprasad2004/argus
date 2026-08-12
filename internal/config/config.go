package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	GeminiAPIKey string
	Model        string
	LogTailLines string
	LocalMode    bool
}

var models = []ModelOption{
	{
		ID:          "gemini-2.5-flash-lite",
		Label:       "Flash Lite",
		Description: "Fastest, free tier — recommended for most users",
	},
	{
		ID:          "gemini-2.5-flash",
		Label:       "Flash",
		Description: "Smarter, slightly slower — better for complex logs",
	},
	{
		ID:          "gemini-2.5-pro",
		Label:       "Pro",
		Description: "Most capable, slower — best for deep RCA",
	},
}

type ModelOption struct {
	ID          string
	Label       string
	Description string
}

func Models() []ModelOption { return models }

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".argus"), nil
}

func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// IsFirstRun returns true if config file doesn't exist yet
func IsFirstRun() bool {
	path, err := ConfigFilePath()
	if err != nil {
		return true
	}
	_, err = os.Stat(path)
	return os.IsNotExist(err)
}

// Load reads config — call after wizard completes on first run
func Load() (*Config, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("ARGUS")
	viper.AutomaticEnv()
	viper.BindEnv("gemini_api_key", "GEMINI_API_KEY")

	viper.SetDefault("model", "gemini-2.5-flash-lite")
	viper.SetDefault("log_tail_lines", "200")

	// read config — don't fail if missing, env vars cover it
	viper.ReadInConfig()

	cfg := &Config{
		GeminiAPIKey: viper.GetString("gemini_api_key"),
		Model:        viper.GetString("model"),
		LogTailLines: viper.GetString("log_tail_lines"),
	}

	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf(
			"GEMINI_API_KEY not set\n" +
				"  Add it to ~/.argus/config.yaml or run: export GEMINI_API_KEY=your_key",
		)
	}

	return cfg, nil
}

// Save writes config to ~/.argus/config.yaml
func Save(apiKey, model string) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}

	path, err := ConfigFilePath()
	if err != nil {
		return err
	}

	content := fmt.Sprintf(`# Argus configuration
# https://github.com/vishnuprasad2004/argus

# Gemini API key (required)
# Get one free at https://aistudio.google.com
gemini_api_key: "%s"

# Model to use
# gemini-2.5-flash-lite = fastest, free tier
# gemini-2.5-flash      = smarter, slightly slower
# gemini-2.5-pro        = most capable, slowest
model: "%s"

# How many historical log lines to fetch on connect
log_tail_lines: "200"
`, apiKey, model)

	return os.WriteFile(path, []byte(content), 0600) // 0600 = owner read/write only
}
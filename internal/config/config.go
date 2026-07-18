package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all Argus configuration
// add new fields here as the app grows
type Config struct {
	GeminiAPIKey string // GEMINI_API_KEY
	Model        string // gemini model to use
	LogTailLines string // how many lines to fetch on connect
}

// Load reads config from ~/.argus/config.yaml and env vars
// env vars always win over config file
func Load() (*Config, error) {
	// set config file location
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home dir: %w", err)
	}

	configDir := filepath.Join(home, ".argus")
	configFile := filepath.Join(configDir, "config.yaml")

	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// env var prefix — ARGUS_GEMINI_API_KEY maps to gemini_api_key
	viper.SetEnvPrefix("ARGUS")
	viper.AutomaticEnv() // read ALL env vars automatically

	// also read bare GEMINI_API_KEY without prefix (your current .env)
	viper.BindEnv("gemini_api_key", "GEMINI_API_KEY")

	// defaults
	viper.SetDefault("model", "gemini-1.5-flash")
	viper.SetDefault("log_tail_lines", "200")

	// create config dir + file if they don't exist
	if err := ensureConfigExists(configDir, configFile); err != nil {
		return nil, err
	}

	// read config file — don't fail if missing, env vars are enough
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config read error: %w", err)
		}
	}

	cfg := &Config{
		GeminiAPIKey: viper.GetString("gemini_api_key"),
		Model:        viper.GetString("model"),
		LogTailLines: viper.GetString("log_tail_lines"),
	}

	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf(
			"GEMINI_API_KEY not set\n" +
			"Set it in ~/.argus/config.yaml or as an env var:\n" +
			"  export GEMINI_API_KEY=your_key_here",
		)
	}

	return cfg, nil
}

// ensureConfigExists creates ~/.argus/config.yaml on first run
func ensureConfigExists(configDir, configFile string) error {
	// create ~/.argus/ directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}

	// if config file already exists, do nothing
	if _, err := os.Stat(configFile); err == nil {
		return nil
	}

	// create default config file
	defaultConfig := `# Argus configuration
# https://github.com/vishnuprasad2004/argus

# Gemini API key (required)
# Get one free at https://aistudio.google.com
gemini_api_key: ""

# Gemini model to use
# gemini-1.5-flash = faster, free tier
# gemini-2.5-flash = smarter, slower
model: "gemini-1.5-flash"

# How many historical log lines to fetch on connect
log_tail_lines: "200"
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("cannot create config file: %w", err)
	}

	fmt.Printf("✓ Created config file: %s\n", configFile)
	fmt.Println("  Add your GEMINI_API_KEY to get started.")
	return nil
}
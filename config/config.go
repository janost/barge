package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all barge configuration.
type Config struct {
	Keybinds Keybinds `yaml:"keybinds"`
}

// Keybinds maps actions to key strings (matching tea.KeyMsg.String() values).
type Keybinds struct {
	Search   string `yaml:"search"`
	Quit     string `yaml:"quit"`
	Refresh  string `yaml:"refresh"`
	DrillAlt string `yaml:"drill_alt"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		Keybinds: Keybinds{
			Search:   "/",
			Quit:     "q",
			Refresh:  "r",
			DrillAlt: "shift+enter",
		},
	}
}

// Load reads configuration from ~/.config/barge/config.yaml,
// falling back to defaults for any unspecified fields.
func Load() Config {
	cfg := Default()
	data, err := os.ReadFile(filepath.Join(ConfigDir(), "config.yaml"))
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

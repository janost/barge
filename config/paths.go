package config

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "bridge")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "bridge")
}

func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "bridge")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "bridge")
}

func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

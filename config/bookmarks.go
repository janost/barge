package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Bookmark struct {
	Mode       string `yaml:"mode"`
	Cluster    string `yaml:"cluster,omitempty"`
	Service    string `yaml:"service,omitempty"`
	Container  string `yaml:"container,omitempty"`
	Command    string `yaml:"command,omitempty"`
	InstanceID string `yaml:"instance_id,omitempty"`
}

type configFile struct {
	Bookmarks map[string]Bookmark `yaml:"bookmarks"`
}

func configPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func loadConfig() (configFile, error) {
	var cfg configFile
	data, err := os.ReadFile(configPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return configFile{Bookmarks: make(map[string]Bookmark)}, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Bookmarks == nil {
		cfg.Bookmarks = make(map[string]Bookmark)
	}
	return cfg, nil
}

func saveConfig(cfg configFile) error {
	if err := EnsureDir(ConfigDir()); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

func LoadBookmarks() (map[string]Bookmark, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Bookmarks, nil
}

func SaveBookmark(name string, bm Bookmark) error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = configFile{Bookmarks: make(map[string]Bookmark)}
	}
	cfg.Bookmarks[name] = bm
	return saveConfig(cfg)
}

func RemoveBookmark(name string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Bookmarks[name]; !ok {
		return fmt.Errorf("bookmark %q not found", name)
	}
	delete(cfg.Bookmarks, name)
	return saveConfig(cfg)
}

func PurgeBookmarks() error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = configFile{}
	}
	cfg.Bookmarks = make(map[string]Bookmark)
	return saveConfig(cfg)
}

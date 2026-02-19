package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const MaxHistoryEntries = 50

type HistoryEntry struct {
	Mode       string    `json:"mode"`
	Cluster    string    `json:"cluster,omitempty"`
	Service    string    `json:"service,omitempty"`
	Task       string    `json:"task,omitempty"`
	Container  string    `json:"container,omitempty"`
	Command    string    `json:"command,omitempty"`
	InstanceID string    `json:"instance_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

func historyPath() string {
	return filepath.Join(StateDir(), "history.json")
}

func LoadHistory() ([]HistoryEntry, error) {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func AppendHistory(entry HistoryEntry) error {
	entries, err := LoadHistory()
	if err != nil {
		entries = nil
	}

	entries = append(entries, entry)

	if len(entries) > MaxHistoryEntries {
		entries = entries[len(entries)-MaxHistoryEntries:]
	}

	return writeHistory(entries)
}

func ClearHistory() error {
	path := historyPath()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func writeHistory(entries []HistoryEntry) error {
	dir := StateDir()
	if err := EnsureDir(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath(), data, 0o644)
}

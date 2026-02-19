package config

import (
	"testing"
	"time"
)

func TestHistory_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	entry := HistoryEntry{
		Mode:      "ecs",
		Cluster:   "prod",
		Service:   "api",
		Task:      "abc123",
		Container: "app",
		Command:   "/bin/sh",
		Timestamp: time.Now().Truncate(time.Second),
	}

	if err := AppendHistory(entry); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Cluster != "prod" {
		t.Errorf("Cluster = %q, want %q", entries[0].Cluster, "prod")
	}
}

func TestHistory_MaxEntries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for i := 0; i < MaxHistoryEntries+10; i++ {
		entry := HistoryEntry{
			Mode:      "ecs",
			Cluster:   "prod",
			Service:   "api",
			Timestamp: time.Now(),
		}
		if err := AppendHistory(entry); err != nil {
			t.Fatalf("AppendHistory: %v", err)
		}
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != MaxHistoryEntries {
		t.Errorf("got %d entries, want %d", len(entries), MaxHistoryEntries)
	}
}

func TestHistory_EC2Entry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	entry := HistoryEntry{
		Mode:       "ec2",
		InstanceID: "i-abc123",
		Timestamp:  time.Now().Truncate(time.Second),
	}

	if err := AppendHistory(entry); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if entries[0].InstanceID != "i-abc123" {
		t.Errorf("InstanceID = %q, want %q", entries[0].InstanceID, "i-abc123")
	}
}

func TestHistory_ClearHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	_ = AppendHistory(HistoryEntry{Mode: "ecs", Cluster: "x", Timestamp: time.Now()})

	if err := ClearHistory(); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries after clear, want 0", len(entries))
	}
}

func TestHistory_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

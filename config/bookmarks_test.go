package config

import (
	"testing"
)

func TestBookmarks_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	bm := Bookmark{
		Mode:      "ecs",
		Cluster:   "prod",
		Service:   "api-service",
		Container: "app",
		Command:   "/bin/sh",
	}

	if err := SaveBookmark("prod-api", bm); err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	bookmarks, err := LoadBookmarks()
	if err != nil {
		t.Fatalf("LoadBookmarks: %v", err)
	}
	got, ok := bookmarks["prod-api"]
	if !ok {
		t.Fatal("bookmark 'prod-api' not found")
	}
	if got.Cluster != "prod" {
		t.Errorf("Cluster = %q, want %q", got.Cluster, "prod")
	}
}

func TestBookmarks_EC2(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	bm := Bookmark{
		Mode:       "ec2",
		InstanceID: "i-abc123",
	}

	if err := SaveBookmark("bastion", bm); err != nil {
		t.Fatalf("SaveBookmark: %v", err)
	}

	bookmarks, err := LoadBookmarks()
	if err != nil {
		t.Fatalf("LoadBookmarks: %v", err)
	}
	if bookmarks["bastion"].InstanceID != "i-abc123" {
		t.Errorf("InstanceID = %q, want %q", bookmarks["bastion"].InstanceID, "i-abc123")
	}
}

func TestBookmarks_Remove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_ = SaveBookmark("x", Bookmark{Mode: "ecs", Cluster: "c"})
	_ = SaveBookmark("y", Bookmark{Mode: "ecs", Cluster: "d"})

	if err := RemoveBookmark("x"); err != nil {
		t.Fatalf("RemoveBookmark: %v", err)
	}

	bookmarks, _ := LoadBookmarks()
	if _, ok := bookmarks["x"]; ok {
		t.Error("bookmark 'x' should have been removed")
	}
	if _, ok := bookmarks["y"]; !ok {
		t.Error("bookmark 'y' should still exist")
	}
}

func TestBookmarks_RemoveNonExistent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	err := RemoveBookmark("nonexistent")
	if err == nil {
		t.Error("expected error removing nonexistent bookmark")
	}
}

func TestBookmarks_Purge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_ = SaveBookmark("a", Bookmark{Mode: "ecs", Cluster: "c"})
	_ = SaveBookmark("b", Bookmark{Mode: "ecs", Cluster: "d"})

	if err := PurgeBookmarks(); err != nil {
		t.Fatalf("PurgeBookmarks: %v", err)
	}

	bookmarks, _ := LoadBookmarks()
	if len(bookmarks) != 0 {
		t.Errorf("got %d bookmarks after purge, want 0", len(bookmarks))
	}
}

func TestBookmarks_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	bookmarks, err := LoadBookmarks()
	if err != nil {
		t.Fatalf("LoadBookmarks: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("got %d bookmarks, want 0", len(bookmarks))
	}
}

func TestBookmarks_MultiplePreserved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_ = SaveBookmark("a", Bookmark{Mode: "ecs", Cluster: "one"})
	_ = SaveBookmark("b", Bookmark{Mode: "ec2", InstanceID: "i-two"})

	bookmarks, _ := LoadBookmarks()
	if len(bookmarks) != 2 {
		t.Fatalf("got %d bookmarks, want 2", len(bookmarks))
	}
	if bookmarks["a"].Cluster != "one" {
		t.Errorf("a.Cluster = %q, want %q", bookmarks["a"].Cluster, "one")
	}
	if bookmarks["b"].InstanceID != "i-two" {
		t.Errorf("b.InstanceID = %q, want %q", bookmarks["b"].InstanceID, "i-two")
	}
}

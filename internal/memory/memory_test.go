package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	// Ensure the config dir exists
	if err := os.MkdirAll(filepath.Join(dir, ".ai-cli"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestAddAndList(t *testing.T) {
	setupTestDir(t)

	if err := Add("my-server", "kris@137.184.10.103"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Keyword != "my-server" || entries[0].Content != "kris@137.184.10.103" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}

func TestAddDuplicate(t *testing.T) {
	setupTestDir(t)

	if err := Add("foo", "bar"); err != nil {
		t.Fatal(err)
	}
	if err := Add("foo", "baz"); err == nil {
		t.Fatal("expected error for duplicate keyword")
	}
	// Case-insensitive duplicate
	if err := Add("FOO", "baz"); err == nil {
		t.Fatal("expected error for case-insensitive duplicate keyword")
	}
}

func TestRemove(t *testing.T) {
	setupTestDir(t)

	if err := Add("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := Add("b", "2"); err != nil {
		t.Fatal(err)
	}
	if err := Remove("a"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Keyword != "b" {
		t.Fatalf("unexpected entries after remove: %+v", entries)
	}
}

func TestRemoveNotFound(t *testing.T) {
	setupTestDir(t)

	if err := Remove("nonexistent"); err == nil {
		t.Fatal("expected error for removing nonexistent keyword")
	}
}

func TestFindMatching(t *testing.T) {
	entries := []Entry{
		{Keyword: "my-server", Content: "kris@137.184.10.103"},
		{Keyword: "db-host", Content: "localhost:5432"},
	}

	matches := FindMatching("connect to my-server please", entries)
	if len(matches) != 1 || matches[0].Keyword != "my-server" {
		t.Fatalf("expected my-server match, got %+v", matches)
	}

	matches = FindMatching("query the db-host and my-server", entries)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	matches = FindMatching("something unrelated", entries)
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestFindMatchingCaseInsensitive(t *testing.T) {
	entries := []Entry{
		{Keyword: "My-Server", Content: "kris@137.184.10.103"},
	}

	matches := FindMatching("connect to MY-SERVER", entries)
	if len(matches) != 1 {
		t.Fatalf("expected case-insensitive match, got %d", len(matches))
	}
}

func TestLoadEmptyFile(t *testing.T) {
	setupTestDir(t)

	entries, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty slice, got %d entries", len(entries))
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	memDir := filepath.Join(dir, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "memory.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_ReadError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	memDir := filepath.Join(dir, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Make memory.json a directory so ReadFile fails with a non-NotExist error
	if err := os.MkdirAll(filepath.Join(memDir, "memory.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when memory.json is a directory, got nil")
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	// Create a regular file where .ai-cli directory should be
	if err := os.WriteFile(filepath.Join(dir, ".ai-cli"), []byte("blocker"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Save([]Entry{{Keyword: "test", Content: "value"}})
	if err == nil {
		t.Fatal("expected error when .ai-cli is a file, got nil")
	}
}

func TestAdd_LoadError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	memDir := filepath.Join(dir, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Write invalid JSON so Load fails
	if err := os.WriteFile(filepath.Join(memDir, "memory.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Add("key", "value")
	if err == nil {
		t.Fatal("expected error from Add when Load fails, got nil")
	}
}

func TestRemove_LoadError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	memDir := filepath.Join(dir, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "memory.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Remove("key")
	if err == nil {
		t.Fatal("expected error from Remove when Load fails, got nil")
	}
}

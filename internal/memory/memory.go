package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kriserickson/ai-cli/internal/config"
)

// Entry represents a single memory key-value pair.
type Entry struct {
	Keyword string `json:"keyword"`
	Content string `json:"content"`
}

func memoryPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory.json"), nil
}

// Load reads all memory entries from disk. Returns empty slice if file not found.
func Load() ([]Entry, error) {
	path, err := memoryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse memory.json: %w", err)
	}
	return entries, nil
}

// Save writes memory entries to disk.
func Save(entries []Entry) error {
	path, err := memoryPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// Add appends a new memory entry. Returns an error if the keyword already exists.
func Add(keyword, content string) error {
	entries, err := Load()
	if err != nil {
		return err
	}

	lower := strings.ToLower(keyword)
	for _, e := range entries {
		if strings.ToLower(e.Keyword) == lower {
			return fmt.Errorf("memory %q already exists; remove it first to update", keyword)
		}
	}

	entries = append(entries, Entry{Keyword: keyword, Content: content})
	return Save(entries)
}

// Remove deletes a memory entry by keyword (case-insensitive).
func Remove(keyword string) error {
	entries, err := Load()
	if err != nil {
		return err
	}

	found := false
	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if strings.EqualFold(e.Keyword, keyword) {
			found = true
			continue
		}
		result = append(result, e)
	}

	if !found {
		return fmt.Errorf("memory %q not found", keyword)
	}

	return Save(result)
}

// List returns all memory entries.
func List() ([]Entry, error) {
	return Load()
}

// FindMatching returns entries whose keywords appear in the input (case-insensitive).
func FindMatching(input string, entries []Entry) []Entry {
	lower := strings.ToLower(input)
	var matches []Entry
	for _, e := range entries {
		if strings.Contains(lower, strings.ToLower(e.Keyword)) {
			matches = append(matches, e)
		}
	}
	return matches
}

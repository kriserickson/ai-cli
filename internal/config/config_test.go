package config

import (
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider.Default != "openrouter" {
		t.Errorf("default provider = %q, want %q", cfg.Provider.Default, "openrouter")
	}
	if cfg.Provider.Model != "anthropic/claude-3.5-sonnet" {
		t.Errorf("model = %q, want %q", cfg.Provider.Model, "anthropic/claude-3.5-sonnet")
	}
	if cfg.Safety.MinCertainty != 80 {
		t.Errorf("min_certainty = %d, want 80", cfg.Safety.MinCertainty)
	}
	if cfg.Safety.AlwaysConfirm {
		t.Error("always_confirm should default to false")
	}
	if cfg.Debug != "none" {
		t.Errorf("debug = %q, want %q", cfg.Debug, "none")
	}
	if len(cfg.Safety.WhitelistPrefixes) == 0 {
		t.Error("whitelist should not be empty by default")
	}
	if cfg.Provider.OpenAI.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("openai base_url = %q", cfg.Provider.OpenAI.BaseURL)
	}
	if cfg.Provider.OpenRouter.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("openrouter base_url = %q", cfg.Provider.OpenRouter.BaseURL)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use a temp dir to avoid touching the real config.
	// Set both HOME (Unix) and USERPROFILE (Windows) since os.UserHomeDir()
	// uses different env vars per platform.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := DefaultConfig()
	cfg.Provider.Model = "gpt-4o"
	cfg.Safety.MinCertainty = 90
	cfg.Debug = "file"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Verify the file was created
	path := filepath.Join(tmpDir, ".ai-cli", "config.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.Provider.Model != "gpt-4o" {
		t.Errorf("loaded model = %q, want %q", loaded.Provider.Model, "gpt-4o")
	}
	if loaded.Safety.MinCertainty != 90 {
		t.Errorf("loaded min_certainty = %d, want 90", loaded.Safety.MinCertainty)
	}
	if loaded.Debug != "file" {
		t.Errorf("loaded debug = %q, want %q", loaded.Debug, "file")
	}
}

func TestLoad_CreatesDefaultOnMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Should have created the file with defaults
	path := filepath.Join(tmpDir, ".ai-cli", "config.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("Load should create default config file when missing")
	}

	if cfg.Provider.Default != "openrouter" {
		t.Errorf("default provider = %q, want %q", cfg.Provider.Default, "openrouter")
	}
}

func TestTOMLRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Safety.WhitelistPrefixes = []string{"git", "ls"}

	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var loaded Config
	if err := toml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if loaded.Provider.OpenAI.APIKey != "test-key" {
		t.Errorf("api_key = %q, want %q", loaded.Provider.OpenAI.APIKey, "test-key")
	}
	if len(loaded.Safety.WhitelistPrefixes) != 2 {
		t.Errorf("whitelist len = %d, want 2", len(loaded.Safety.WhitelistPrefixes))
	}
}

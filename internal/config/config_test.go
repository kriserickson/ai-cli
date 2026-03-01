package config

import (
	"os"
	"path/filepath"
	"runtime"
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
	if len(cfg.Safety.AllowlistPrefixes) == 0 {
		t.Error("allowlist should not be empty by default")
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

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("config file mode = %o, want %o", got, 0o600)
		}
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

func TestLoad_WhitelistMigration(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Write a config with whitelist_prefixes (deprecated) and no allowlist_prefixes.
	tomlContent := `
debug = "none"

[provider]
default = "openrouter"
model = "anthropic/claude-3.5-sonnet"
[provider.openai]
api_key = ""
base_url = "https://api.openai.com/v1"
[provider.openrouter]
api_key = ""
base_url = "https://openrouter.ai/api/v1"

[safety]
always_confirm = false
min_certainty = 80
whitelist_prefixes = ["git", "ls", "cat"]
`
	dir := filepath.Join(tmpDir, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tomlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Allowlist should be migrated from whitelist
	if len(cfg.Safety.AllowlistPrefixes) != 3 {
		t.Errorf("allowlist len = %d, want 3", len(cfg.Safety.AllowlistPrefixes))
	}
	if cfg.Safety.AllowlistPrefixes[0] != "git" {
		t.Errorf("allowlist[0] = %q, want %q", cfg.Safety.AllowlistPrefixes[0], "git")
	}
	// Whitelist should be cleared
	if len(cfg.Safety.WhitelistPrefixes) != 0 {
		t.Errorf("whitelist should be nil after migration, got %v", cfg.Safety.WhitelistPrefixes)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	dir := filepath.Join(tmpDir, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid toml"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

func TestConfigDir_And_ConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir error: %v", err)
	}
	wantDir := filepath.Join(tmpDir, ".ai-cli")
	if dir != wantDir {
		t.Errorf("Dir() = %q, want %q", dir, wantDir)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path error: %v", err)
	}
	wantPath := filepath.Join(tmpDir, ".ai-cli", "config.toml")
	if path != wantPath {
		t.Errorf("Path() = %q, want %q", path, wantPath)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := DefaultConfig()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Verify directory and file exist
	dir := filepath.Join(tmpDir, ".ai-cli")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create a regular file where the directory should be, so MkdirAll fails
	aiCliPath := filepath.Join(tmpDir, ".ai-cli")
	if err := os.WriteFile(aiCliPath, []byte("blocker"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	err := Save(cfg)
	if err == nil {
		t.Fatal("expected error when .ai-cli is a file, got nil")
	}
}

func TestLoad_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create the config file as a directory to cause a read error
	dir := filepath.Join(tmpDir, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	configFilePath := filepath.Join(dir, "config.toml")
	// Make config.toml a directory instead of a file
	if err := os.MkdirAll(configFilePath, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when config.toml is a directory, got nil")
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Save once
	cfg := DefaultConfig()
	cfg.Provider.Model = "model-v1"
	if err := Save(cfg); err != nil {
		t.Fatalf("first Save error: %v", err)
	}

	// Save again with different value
	cfg.Provider.Model = "model-v2"
	if err := Save(cfg); err != nil {
		t.Fatalf("second Save error: %v", err)
	}

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Provider.Model != "model-v2" {
		t.Errorf("loaded model = %q, want %q", loaded.Provider.Model, "model-v2")
	}
}

func TestLoad_PreservesExistingValues(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Write a minimal config with some custom values
	customToml := `
debug = "file"

[provider]
default = "openai"
model = "gpt-4o"
[provider.openai]
api_key = "sk-test"
base_url = "https://api.openai.com/v1"
[provider.openrouter]
api_key = ""
base_url = "https://openrouter.ai/api/v1"

[safety]
always_confirm = true
min_certainty = 95
allowlist_prefixes = ["git", "ls"]
`
	dir := filepath.Join(tmpDir, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(customToml), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provider.Default != "openai" {
		t.Errorf("provider = %q, want openai", cfg.Provider.Default)
	}
	if cfg.Provider.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", cfg.Provider.Model)
	}
	if cfg.Provider.OpenAI.APIKey != "sk-test" {
		t.Errorf("api_key = %q, want sk-test", cfg.Provider.OpenAI.APIKey)
	}
	if cfg.Debug != "file" {
		t.Errorf("debug = %q, want file", cfg.Debug)
	}
	if !cfg.Safety.AlwaysConfirm {
		t.Error("always_confirm should be true")
	}
	if cfg.Safety.MinCertainty != 95 {
		t.Errorf("min_certainty = %d, want 95", cfg.Safety.MinCertainty)
	}
	if len(cfg.Safety.AllowlistPrefixes) != 2 {
		t.Errorf("allowlist len = %d, want 2", len(cfg.Safety.AllowlistPrefixes))
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input   string
		want    bool
		wantErr bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"True", true, false},
		{"FALSE", false, false},
		{"  true  ", true, false},
		{"yes", false, true},
		{"1", false, true},
		{"", false, true},
		{"maybe", false, true},
	}
	for _, tt := range tests {
		got, err := ParseBool(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseBool(%q) error = nil, want error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseBool(%q) error = %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestTOMLRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Safety.AllowlistPrefixes = []string{"git", "ls"}

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
	if len(loaded.Safety.AllowlistPrefixes) != 2 {
		t.Errorf("allowlist len = %d, want 2", len(loaded.Safety.AllowlistPrefixes))
	}
}

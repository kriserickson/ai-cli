package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// applyTestCfg returns a baseline config in a temp HOME so Save() works.
func applyTestCfg(t *testing.T) *Config {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	return DefaultConfig()
}

func TestApplyAction_SetModel(t *testing.T) {
	cfg := applyTestCfg(t)
	if err := ApplyAction(cfg, "set_model", "", "gpt-4o"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.Provider.Model, "gpt-4o")
	}
}

func TestApplyAction_SetProvider(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "openai", value: "openai", want: "openai"},
		{name: "openrouter", value: "openrouter", want: "openrouter"},
		{name: "invalid provider", value: "anthropic", wantErr: true},
		{name: "empty string", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyTestCfg(t)
			err := ApplyAction(cfg, "set_provider", "", tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Provider.Default != tt.want {
				t.Errorf("Default = %q, want %q", cfg.Provider.Default, tt.want)
			}
		})
	}
}

func TestApplyAction_SetKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		provider string // override default provider if non-empty
		check    func(*Config) string
		wantErr  bool
	}{
		{
			name:  "llm_key sets current provider (openrouter)",
			key:   "llm_key",
			value: "sk-test-llm",
			check: func(c *Config) string {
				if c.Provider.OpenRouter.APIKey != "sk-test-llm" {
					return "OpenRouter APIKey not set via llm_key"
				}
				return ""
			},
		},
		{
			name:     "llm_key sets openai when provider is openai",
			key:      "llm_key",
			value:    "sk-test-openai",
			provider: ProviderOpenAI,
			check: func(c *Config) string {
				if c.Provider.OpenAI.APIKey != "sk-test-openai" {
					return "OpenAI APIKey not set via llm_key"
				}
				return ""
			},
		},
		{
			name:     "llm_key sets local when provider is local",
			key:      "llm_key",
			value:    "sk-test-local",
			provider: ProviderLocal,
			check: func(c *Config) string {
				if c.Provider.Local.APIKey != "sk-test-local" {
					return "Local APIKey not set via llm_key"
				}
				return ""
			},
		},
		{
			name:    "unknown key",
			key:     "anthropic_key",
			value:   "sk-test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyTestCfg(t)
			if tt.provider != "" {
				cfg.Provider.Default = tt.provider
			}
			err := ApplyAction(cfg, "set_key", tt.key, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg := tt.check(cfg); msg != "" {
				t.Error(msg)
			}
		})
	}
}

func TestApplyAction_SetSafety_AlwaysConfirm(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "true", value: "true", want: true},
		{name: "false", value: "false", want: false},
		{name: "True (case-insensitive)", value: "True", want: true},
		{name: "FALSE (case-insensitive)", value: "FALSE", want: false},
		{name: "TRUE (all-caps)", value: "TRUE", want: true},
		{name: "invalid value", value: "yes", wantErr: true},
		{name: "empty string", value: "", wantErr: true},
		{name: "numeric 1", value: "1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyTestCfg(t)
			// Set to the opposite of expected so we can verify it changed.
			cfg.Safety.AlwaysConfirm = !tt.want
			err := ApplyAction(cfg, "set_safety", "always_confirm", tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Safety.AlwaysConfirm != tt.want {
				t.Errorf("AlwaysConfirm = %v, want %v", cfg.Safety.AlwaysConfirm, tt.want)
			}
		})
	}
}

func TestApplyAction_SetSafety_MinCertainty(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "valid 90", value: "90", want: 90},
		{name: "boundary 0", value: "0", want: 0},
		{name: "boundary 100", value: "100", want: 100},
		{name: "not a number", value: "abc", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "over 100", value: "101", wantErr: true},
		{name: "float", value: "80.5", wantErr: true},
		{name: "empty string", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyTestCfg(t)
			err := ApplyAction(cfg, "set_safety", "min_certainty", tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Safety.MinCertainty != tt.want {
				t.Errorf("MinCertainty = %d, want %d", cfg.Safety.MinCertainty, tt.want)
			}
		})
	}
}

func TestApplyAction_SetSafety_ToolCalling(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "never", value: ToolCallingNever, want: ToolCallingNever},
		{name: "always_prompt", value: ToolCallingAlwaysPrompt, want: ToolCallingAlwaysPrompt},
		{name: "dangerous_prompt", value: ToolCallingDangerousPrompt, want: ToolCallingDangerousPrompt},
		{name: "always_allow", value: ToolCallingAlwaysAllow, want: ToolCallingAlwaysAllow},
		{name: "invalid", value: "sometimes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyTestCfg(t)
			err := ApplyAction(cfg, "set_safety", "tool_calling", tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Safety.ToolCalling != tt.want {
				t.Errorf("ToolCalling = %q, want %q", cfg.Safety.ToolCalling, tt.want)
			}
		})
	}
}

func TestApplyAction_SetSafety_UnknownKey(t *testing.T) {
	cfg := applyTestCfg(t)
	err := ApplyAction(cfg, "set_safety", "nonexistent", "value")
	if err == nil {
		t.Fatal("expected error for unknown safety key, got nil")
	}
}

func TestApplyAction_SaveError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not reliable on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create a regular file where .ai-cli should be so Save fails.
	if err := os.WriteFile(filepath.Join(tmpDir, ".ai-cli"), []byte("blocker"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	err := ApplyAction(cfg, "set_model", "", "gpt-4o")
	if err == nil {
		t.Fatal("expected error when Save fails, got nil")
	}
}

func TestApplyAction_UnknownAction(t *testing.T) {
	cfg := applyTestCfg(t)
	err := ApplyAction(cfg, "do_something", "", "value")
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
}

func TestApplyAction_SavesPersistently(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := DefaultConfig()
	if err := ApplyAction(cfg, "set_model", "", "gpt-4o-mini"); err != nil {
		t.Fatalf("ApplyAction error: %v", err)
	}

	// Load from disk and verify the change persisted.
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Provider.Model != "gpt-4o-mini" {
		t.Errorf("persisted Model = %q, want %q", loaded.Provider.Model, "gpt-4o-mini")
	}
}

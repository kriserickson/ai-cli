package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

// runCmd executes rootCmd with the given args and returns captured stdout.
// Not safe for t.Parallel() — temporarily replaces os.Stdout.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs(args)
	rootCmd.SilenceUsage = true
	rootCmd.TraverseChildren = true
	debugFlag = ""
	execErr := rootCmd.Execute()

	w.Close()
	os.Stdout = saved

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String(), execErr
}

// tempHome redirects config reads/writes to a temp directory on both Unix and Windows.
func tempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

// --- maskKey ---

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "****"},
		{"short", "****"},
		{"12345678", "****"},         // exactly 8 chars: still masked
		{"123456789", "1234...6789"}, // 9 chars: shows prefix + suffix
		{"sk-longapikey1234", "sk-l...1234"},
	}
	for _, tt := range tests {
		got := maskKey(tt.input)
		if got != tt.want {
			t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- getConfigValue ---

func TestGetConfigValue(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.APIKey = "sk-longapikey1234" // 17 chars → "sk-l...1234"
	cfg.Provider.OpenRouter.APIKey = "short"         // ≤8 chars → "****"

	tests := []struct {
		key  string
		want string
	}{
		{"provider", "openrouter"},
		{"default", "openrouter"},
		{"model", "anthropic/claude-3.5-sonnet"},
		{"openai_key", "sk-l...1234"},
		{"openrouter_key", "****"},
		{"openai_url", "https://api.openai.com/v1"},
		{"openrouter_url", "https://openrouter.ai/api/v1"},
		{"always_confirm", "false"},
		{"min_certainty", "80"},
		{"allowlist", "git, ls, cat, echo, pwd, head, tail, wc, grep, find, which, man"},
		{"debug", "none"},
	}
	for _, tt := range tests {
		got, err := getConfigValue(cfg, tt.key)
		if err != nil {
			t.Errorf("getConfigValue(%q): unexpected error: %v", tt.key, err)
			continue
		}
		if got != tt.want {
			t.Errorf("getConfigValue(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

// --- llm_key / llm_url (current-provider aliases) ---

func TestGetConfigValue_LlmKey(t *testing.T) {
	cfg := config.DefaultConfig() // default provider is openrouter
	cfg.Provider.OpenRouter.APIKey = "sk-or-longkey1234"

	got, err := getConfigValue(cfg, "llm_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != maskKey("sk-or-longkey1234") {
		t.Errorf("llm_key = %q, want %q", got, maskKey("sk-or-longkey1234"))
	}

	// Switch provider to openai
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-openai-longkey1234"
	got, err = getConfigValue(cfg, "llm_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != maskKey("sk-openai-longkey1234") {
		t.Errorf("llm_key (openai) = %q, want %q", got, maskKey("sk-openai-longkey1234"))
	}
}

func TestGetConfigValue_LlmUrl(t *testing.T) {
	cfg := config.DefaultConfig() // default provider is openrouter
	got, err := getConfigValue(cfg, "llm_url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != cfg.Provider.OpenRouter.BaseURL {
		t.Errorf("llm_url = %q, want %q", got, cfg.Provider.OpenRouter.BaseURL)
	}
}

func TestSetConfigValue_LlmKey(t *testing.T) {
	cfg := config.DefaultConfig() // default provider is openrouter
	if err := setConfigValue(cfg, "llm_key", "sk-new-key"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider.OpenRouter.APIKey != "sk-new-key" {
		t.Errorf("OpenRouter.APIKey = %q, want %q", cfg.Provider.OpenRouter.APIKey, "sk-new-key")
	}

	// Switch to openai and set again
	cfg.Provider.Default = config.ProviderOpenAI
	if err := setConfigValue(cfg, "llm_key", "sk-openai-new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider.OpenAI.APIKey != "sk-openai-new" {
		t.Errorf("OpenAI.APIKey = %q, want %q", cfg.Provider.OpenAI.APIKey, "sk-openai-new")
	}
}

func TestSetConfigValue_LlmUrl(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := setConfigValue(cfg, "llm_url", "https://custom.example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Provider.OpenRouter.BaseURL != "https://custom.example.com" {
		t.Errorf("OpenRouter.BaseURL = %q, want %q", cfg.Provider.OpenRouter.BaseURL, "https://custom.example.com")
	}
}

func TestGetConfigValue_UnknownKey(t *testing.T) {
	_, err := getConfigValue(config.DefaultConfig(), "nonexistent")
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}

// --- setConfigValue ---

func TestSetConfigValue(t *testing.T) {
	tests := []struct {
		key   string
		value string
		check func(*config.Config) bool
	}{
		{"provider", "openai", func(c *config.Config) bool { return c.Provider.Default == "openai" }},
		{"default", "openai", func(c *config.Config) bool { return c.Provider.Default == "openai" }},
		{"model", "gpt-4o", func(c *config.Config) bool { return c.Provider.Model == "gpt-4o" }},
		{"openai_key", "sk-abc", func(c *config.Config) bool { return c.Provider.OpenAI.APIKey == "sk-abc" }},
		{"openrouter_key", "sk-or-abc", func(c *config.Config) bool { return c.Provider.OpenRouter.APIKey == "sk-or-abc" }},
		{"openai_url", "https://proxy.example.com", func(c *config.Config) bool { return c.Provider.OpenAI.BaseURL == "https://proxy.example.com" }},
		{"openrouter_url", "https://proxy.example.com", func(c *config.Config) bool { return c.Provider.OpenRouter.BaseURL == "https://proxy.example.com" }},
		{"always_confirm", "true", func(c *config.Config) bool { return c.Safety.AlwaysConfirm }},
		{"always_confirm", "false", func(c *config.Config) bool { return !c.Safety.AlwaysConfirm }},
		{"always_confirm", "TRUE", func(c *config.Config) bool { return c.Safety.AlwaysConfirm }},
		{"min_certainty", "95", func(c *config.Config) bool { return c.Safety.MinCertainty == 95 }},
		{"min_certainty", "0", func(c *config.Config) bool { return c.Safety.MinCertainty == 0 }},
		{"min_certainty", "100", func(c *config.Config) bool { return c.Safety.MinCertainty == 100 }},
		{"debug", "screen", func(c *config.Config) bool { return c.Debug == "screen" }},
		{"debug", "file", func(c *config.Config) bool { return c.Debug == "file" }},
		{"debug", "none", func(c *config.Config) bool { return c.Debug == "none" }},
	}
	for _, tt := range tests {
		cfg := config.DefaultConfig()
		if err := setConfigValue(cfg, tt.key, tt.value); err != nil {
			t.Errorf("setConfigValue(%q, %q): unexpected error: %v", tt.key, tt.value, err)
			continue
		}
		if !tt.check(cfg) {
			t.Errorf("setConfigValue(%q, %q): config not updated as expected", tt.key, tt.value)
		}
	}
}

func TestSetConfigValue_Validation(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"provider", "gpt"},         // invalid provider name
		{"debug", "verbose"},        // invalid debug mode
		{"min_certainty", "notnum"}, // not a number
		{"min_certainty", "-1"},     // out of range
		{"min_certainty", "101"},    // out of range
		{"always_confirm", "1"},     // invalid bool-like value
		{"unknown_key", "value"},    // unknown key
	}
	for _, tt := range tests {
		if err := setConfigValue(config.DefaultConfig(), tt.key, tt.value); err == nil {
			t.Errorf("setConfigValue(%q, %q): expected error, got nil", tt.key, tt.value)
		}
	}
}

// --- version command ---

func TestVersionCommand(t *testing.T) {
	out, err := runCmd(t, "version")
	if err != nil {
		t.Fatalf("version command error: %v", err)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("version output %q does not contain version %q", out, Version)
	}
}

// --- config subcommands ---

func TestConfigShow(t *testing.T) {
	tempHome(t)

	out, err := runCmd(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error: %v", err)
	}
	for _, want := range []string{"[provider]", "[safety]", "model", "default"} {
		if !strings.Contains(out, want) {
			t.Errorf("config show output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestConfigSetAndGet(t *testing.T) {
	tempHome(t)

	if _, err := runCmd(t, "config", "set", "model", "gpt-4o"); err != nil {
		t.Fatalf("config set model: %v", err)
	}

	out, err := runCmd(t, "config", "get", "model")
	if err != nil {
		t.Fatalf("config get model: %v", err)
	}
	if !strings.Contains(out, "gpt-4o") {
		t.Errorf("config get model = %q, want it to contain %q", strings.TrimSpace(out), "gpt-4o")
	}
}

func TestConfigGet_UnknownKey(t *testing.T) {
	tempHome(t)

	_, err := runCmd(t, "config", "get", "nonexistent")
	if err == nil {
		t.Error("expected error for unknown config key, got nil")
	}
}

func TestConfigSet_InvalidValue(t *testing.T) {
	tempHome(t)

	_, err := runCmd(t, "config", "set", "provider", "invalid")
	if err == nil {
		t.Error("expected error for invalid provider value, got nil")
	}
}

// --- hasNoglobAlias ---

func TestHasNoglobAlias(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"single-quoted", "alias ai='noglob ai'", true},
		{"double-quoted", `alias ai="noglob ai"`, true},
		{"indented", "  alias ai='noglob ai'", true},
		{"commented out", "# alias ai='noglob ai'", false},
		{"no alias", "export PATH=$PATH:/usr/local/bin", false},
		{"empty", "", false},
		{"different alias name", "alias gai='noglob ai'", false},
		{"among other lines", "export FOO=bar\nalias ai='noglob ai'\nexport BAZ=qux", true},
		{"comment then real alias", "# old\nalias ai='noglob ai'", true},
		{"only comment version", "# alias ai='noglob ai'\nexport FOO=bar", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasNoglobAlias(tt.content); got != tt.want {
				t.Errorf("hasNoglobAlias() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- debug flag ---

func TestDebugFlag_NoOptDefVal(t *testing.T) {
	// Verify that --debug without a value defaults to "screen" rather than
	// consuming the next argument as the flag value.
	f := rootCmd.Flags().Lookup("debug")
	if f == nil {
		t.Fatal("--debug flag not registered on rootCmd")
	}
	if f.NoOptDefVal != "screen" {
		t.Errorf("--debug NoOptDefVal = %q, want %q", f.NoOptDefVal, "screen")
	}
}

package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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
	_, _ = io.Copy(&buf, r)
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

// --- config completion functions ---

func TestConfigGetCompletion(t *testing.T) {
	// Find the "get" subcommand under "config"
	configCmd, _, _ := rootCmd.Find([]string{"config"})
	if configCmd == nil {
		t.Fatal("config command not found")
	}

	var getCmd *cobra.Command
	for _, c := range configCmd.Commands() {
		if c.Use == "get <key>" {
			getCmd = c
			break
		}
	}
	if getCmd == nil {
		t.Fatal("config get command not found")
	}

	if getCmd.ValidArgsFunction == nil {
		t.Fatal("ValidArgsFunction not set on config get")
	}

	// No args yet: should suggest configKeys
	completions, dir := getCmd.ValidArgsFunction(getCmd, nil, "")
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
	if len(completions) == 0 {
		t.Error("expected completions for config get with no args")
	}

	// Already have one arg: should return nil
	completions, _ = getCmd.ValidArgsFunction(getCmd, []string{"model"}, "")
	if completions != nil {
		t.Errorf("expected nil completions when arg already provided, got %v", completions)
	}
}

func TestConfigSetCompletion(t *testing.T) {
	configCmd, _, _ := rootCmd.Find([]string{"config"})
	if configCmd == nil {
		t.Fatal("config command not found")
	}

	var setCmd *cobra.Command
	for _, c := range configCmd.Commands() {
		if c.Use == "set <key> <value>" {
			setCmd = c
			break
		}
	}
	if setCmd == nil {
		t.Fatal("config set command not found")
	}

	if setCmd.ValidArgsFunction == nil {
		t.Fatal("ValidArgsFunction not set on config set")
	}

	// No args: should suggest keys
	completions, dir := setCmd.ValidArgsFunction(setCmd, nil, "")
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
	if len(completions) == 0 {
		t.Error("expected key completions")
	}

	// One arg "provider": should suggest values
	completions, dir = setCmd.ValidArgsFunction(setCmd, []string{"provider"}, "")
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
	if len(completions) != 2 {
		t.Errorf("expected 2 provider completions, got %d", len(completions))
	}

	// One arg "model": no fixed values
	completions, _ = setCmd.ValidArgsFunction(setCmd, []string{"model"}, "")
	if completions != nil {
		t.Errorf("expected nil completions for model value, got %v", completions)
	}

	// Two args: no more completions
	completions, _ = setCmd.ValidArgsFunction(setCmd, []string{"provider", "openai"}, "")
	if completions != nil {
		t.Errorf("expected nil completions with 2 args, got %v", completions)
	}
}

// --- config show/get/set via cobra ---

func TestConfigShow_ContainsAllSections(t *testing.T) {
	tempHome(t)

	out, err := runCmd(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error: %v", err)
	}
	// Verify TOML structure
	for _, want := range []string{"[provider]", "[safety]", "debug"} {
		if !strings.Contains(out, want) {
			t.Errorf("config show missing %q", want)
		}
	}
}

func TestConfigSet_DebugValue(t *testing.T) {
	tempHome(t)

	if _, err := runCmd(t, "config", "set", "debug", "screen"); err != nil {
		t.Fatalf("config set debug: %v", err)
	}

	out, err := runCmd(t, "config", "get", "debug")
	if err != nil {
		t.Fatalf("config get debug: %v", err)
	}
	if !strings.Contains(out, "screen") {
		t.Errorf("config get debug = %q, want 'screen'", strings.TrimSpace(out))
	}
}

func TestConfigSet_InvalidDebugValue(t *testing.T) {
	tempHome(t)

	_, err := runCmd(t, "config", "set", "debug", "verbose")
	if err == nil {
		t.Error("expected error for invalid debug value")
	}
}

func TestConfigSet_AlwaysConfirm(t *testing.T) {
	tempHome(t)

	if _, err := runCmd(t, "config", "set", "always_confirm", "true"); err != nil {
		t.Fatalf("config set always_confirm: %v", err)
	}

	out, err := runCmd(t, "config", "get", "always_confirm")
	if err != nil {
		t.Fatalf("config get always_confirm: %v", err)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("config get always_confirm = %q, want 'true'", strings.TrimSpace(out))
	}
}

func TestConfigSet_MinCertainty(t *testing.T) {
	tempHome(t)

	if _, err := runCmd(t, "config", "set", "min_certainty", "95"); err != nil {
		t.Fatalf("config set min_certainty: %v", err)
	}

	out, err := runCmd(t, "config", "get", "min_certainty")
	if err != nil {
		t.Fatalf("config get min_certainty: %v", err)
	}
	if !strings.Contains(out, "95") {
		t.Errorf("config get min_certainty = %q, want '95'", strings.TrimSpace(out))
	}
}

func TestConfigSet_URLs(t *testing.T) {
	tempHome(t)

	if _, err := runCmd(t, "config", "set", "openai_url", "https://proxy.example.com/v1"); err != nil {
		t.Fatalf("config set openai_url: %v", err)
	}
	out, err := runCmd(t, "config", "get", "openai_url")
	if err != nil {
		t.Fatalf("config get openai_url: %v", err)
	}
	if !strings.Contains(out, "https://proxy.example.com/v1") {
		t.Errorf("openai_url = %q", strings.TrimSpace(out))
	}
}

func TestConfigShow_LoadError(t *testing.T) {
	tempHome(t)

	// Write an invalid TOML file
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "config", "show")
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
}

func TestConfigGet_LoadError(t *testing.T) {
	tempHome(t)

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "config", "get", "model")
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
}

func TestConfigSet_LoadError(t *testing.T) {
	tempHome(t)

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "config", "set", "model", "gpt-4o")
	if err == nil {
		t.Error("expected error for invalid config, got nil")
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

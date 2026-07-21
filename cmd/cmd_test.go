package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/history"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
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
	retryOnErrorFlag = false
	retryDepthFlag = 0
	historyVerbose = false
	historyCount = 10
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

func seedHistory(t *testing.T, instructions ...string) []history.Session {
	t.Helper()

	cfg := config.DefaultConfig()
	sessions := make([]history.Session, 0, len(instructions))
	for _, instruction := range instructions {
		session := history.NewSession(instruction, t.TempDir(), cfg, shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh"})
		if err := history.Save(session); err != nil {
			t.Fatalf("history.Save(%q): %v", instruction, err)
		}
		sessions = append(sessions, *session)
	}
	return sessions
}

// --- maskKey ---

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
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
		{keyAlwaysConfirm, "false"},
		{"min_certainty", "80"},
		{"allowlist", "git, ls, cat, echo, pwd, head, tail, wc, grep, find, which, man"},
		{"debug", "none"},
		{"history_include_llm_output", "true"},
		{"history_include_debug", "false"},
		{"history_ask_on_error", "true"},
		{"history_auto_check_on_error", "false"},
		{"history_retry_max_attempts", "1"},
		{"history_retry_context_depth", "3"},
		{"debug_log_payloads", "false"},
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

func TestCurrentProviderDetail_Local(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderLocal
	cfg.Provider.Local.APIKey = "local-test-key"
	cfg.Provider.Local.BaseURL = "http://localhost:11434"

	pd := currentProviderDetail(cfg)
	if pd.APIKey != "local-test-key" {
		t.Errorf("local APIKey = %q, want %q", pd.APIKey, "local-test-key")
	}
	if pd.BaseURL != "http://localhost:11434" {
		t.Errorf("local BaseURL = %q, want %q", pd.BaseURL, "http://localhost:11434")
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
		{keyAlwaysConfirm, "true", func(c *config.Config) bool { return c.Safety.AlwaysConfirm }},
		{keyAlwaysConfirm, "false", func(c *config.Config) bool { return !c.Safety.AlwaysConfirm }},
		{keyAlwaysConfirm, "TRUE", func(c *config.Config) bool { return c.Safety.AlwaysConfirm }},
		{"min_certainty", "95", func(c *config.Config) bool { return c.Safety.MinCertainty == 95 }},
		{"min_certainty", "0", func(c *config.Config) bool { return c.Safety.MinCertainty == 0 }},
		{"min_certainty", "100", func(c *config.Config) bool { return c.Safety.MinCertainty == 100 }},
		{"debug", "screen", func(c *config.Config) bool { return c.Debug == "screen" }},
		{"debug", "file", func(c *config.Config) bool { return c.Debug == "file" }},
		{"debug", "none", func(c *config.Config) bool { return c.Debug == "none" }},
		{"history_include_llm_output", "false", func(c *config.Config) bool { return !c.History.IncludeLLMOutput }},
		{"history_include_debug", "true", func(c *config.Config) bool { return c.History.IncludeDebug }},
		{"history_ask_on_error", "false", func(c *config.Config) bool { return !c.History.AskOnError }},
		{"history_auto_check_on_error", "true", func(c *config.Config) bool { return c.History.AutoCheckOnError }},
		{"history_retry_max_attempts", "2", func(c *config.Config) bool { return c.History.RetryMaxAttempts == 2 }},
		{"history_retry_context_depth", "5", func(c *config.Config) bool { return c.History.RetryContextDepth == 5 }},
		{"debug_log_payloads", "true", func(c *config.Config) bool { return c.DebugLogPayloads }},
		{"debug_log_payloads", "false", func(c *config.Config) bool { return !c.DebugLogPayloads }},
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
		{"debug_log_payloads", "1"}, // invalid bool-like value
		{"min_certainty", "notnum"}, // not a number
		{"min_certainty", "-1"},     // out of range
		{"min_certainty", "101"},    // out of range
		{keyAlwaysConfirm, "1"},     // invalid bool-like value
		{"history_retry_max_attempts", "-1"},
		{"history_retry_context_depth", "0"},
		{"unknown_key", "value"}, // unknown key
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

	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-secret-openai"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	out, err := runCmd(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error: %v", err)
	}
	for _, want := range []string{"[provider]", "[safety]", "[history]", "model", "default"} {
		if !strings.Contains(out, want) {
			t.Errorf("config show output missing %q\nfull output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "sk-secret-openai") {
		t.Fatalf("config show leaked raw API key:\n%s", out)
	}
	if !strings.Contains(out, config.RedactSecret("sk-secret-openai")) {
		t.Fatalf("config show should contain masked API key:\n%s", out)
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
	if len(completions) != 3 {
		t.Errorf("expected 3 provider completions, got %d", len(completions))
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
	for _, want := range []string{"[provider]", "[safety]", "[history]", "debug"} {
		if !strings.Contains(out, want) {
			t.Errorf("config show missing %q", want)
		}
	}
}

func TestConfigShow_RedactsAllProviderKeys(t *testing.T) {
	tempHome(t)

	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.APIKey = "sk-openai-secret"
	cfg.Provider.OpenRouter.APIKey = "sk-openrouter-secret"
	cfg.Provider.Local.APIKey = "local-secret"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	out, err := runCmd(t, "config", "show")
	if err != nil {
		t.Fatalf("config show error: %v", err)
	}

	for _, raw := range []string{"sk-openai-secret", "sk-openrouter-secret", "local-secret"} {
		if strings.Contains(out, raw) {
			t.Fatalf("config show leaked raw secret %q:\n%s", raw, out)
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

func TestConfigSet_DebugLogPayloads(t *testing.T) {
	tempHome(t)

	if _, err := runCmd(t, "config", "set", "debug_log_payloads", "true"); err != nil {
		t.Fatalf("config set debug_log_payloads: %v", err)
	}

	out, err := runCmd(t, "config", "get", "debug_log_payloads")
	if err != nil {
		t.Fatalf("config get debug_log_payloads: %v", err)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("config get debug_log_payloads = %q, want 'true'", strings.TrimSpace(out))
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

	if _, err := runCmd(t, "config", "set", keyAlwaysConfirm, "true"); err != nil {
		t.Fatalf("config set always_confirm: %v", err)
	}

	out, err := runCmd(t, "config", "get", keyAlwaysConfirm)
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

	if _, err := runCmd(t, "config", "set", "llm_url", "https://proxy.example.com/v1"); err != nil {
		t.Fatalf("config set llm_url: %v", err)
	}
	out, err := runCmd(t, "config", "get", "llm_url")
	if err != nil {
		t.Fatalf("config get llm_url: %v", err)
	}
	if !strings.Contains(out, "https://proxy.example.com/v1") {
		t.Errorf("llm_url = %q", strings.TrimSpace(out))
	}
}

func TestHistoryListAndShow(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Model = "gpt-4o-mini"

	session := history.NewSession("on the remote server my-server download the latest backup", t.TempDir(), cfg, shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh"})
	session.Status = "completed"
	session.RecordExchange("initial", 1, "system", session.Instruction, &llm.ChatResult{
		Response: &llm.Response{
			Type: responseCommands,
			Commands: []llm.Command{
				{Command: "scp kris@example:backup.sql.gz ."},
			},
		},
	}, nil, cfg.History)
	session.Executions = []history.CommandAttempt{
		{
			Attempt:   1,
			Index:     0,
			Command:   "scp kris@137.184.10.103:~/database-backup/latest.sql.gz .",
			ExitCode:  0,
			StartedAt: time.Now(),
		},
	}
	session.UpdatedAt = time.Date(2026, 3, 1, 18, 19, 13, 0, time.Local)
	if err := history.Save(session); err != nil {
		t.Fatalf("history.Save(session): %v", err)
	}

	fallback := history.NewSession("second command", t.TempDir(), cfg, shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh"})
	fallback.RecordExchange("initial", 1, "system", fallback.Instruction, &llm.ChatResult{
		Response: &llm.Response{
			Type: responseCommands,
			Commands: []llm.Command{
				{Command: "echo second"},
			},
		},
	}, nil, cfg.History)
	if err := history.Save(fallback); err != nil {
		t.Fatalf("history.Save(fallback): %v", err)
	}

	out, err := runCmd(t, "history", "list")
	if err != nil {
		t.Fatalf("history list: %v", err)
	}
	for _, want := range []string{
		"2026-03-01 18:19:13 : completed retries=0 model=gpt-4o-mini",
		"  prompt: on the remote server my-server download the latest backup",
		"  command: scp kris@137.184.10.103:~/database-backup/latest.sql.gz .",
		"  prompt: second command",
		"  command: echo second",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("history list output missing %q:\n%s", want, out)
		}
	}

	out, err = runCmd(t, "history", "show", session.ID)
	if err != nil {
		t.Fatalf("history show: %v", err)
	}
	if !strings.Contains(out, "Instruction: "+session.Instruction) {
		t.Fatalf("history show missing instruction:\n%s", out)
	}
	if !strings.Contains(out, "ID:") {
		t.Fatalf("history show missing ID:\n%s", out)
	}
}

func TestHistoryListDefaultCountVerboseAndCountFlag(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Model = "gpt-4o-mini"

	for i := range 12 {
		session := history.NewSession(fmt.Sprintf("prompt %02d", i), t.TempDir(), cfg, shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh"})
		session.Status = "completed"
		session.UpdatedAt = time.Date(2026, 3, 2, 10, i, 0, 0, time.UTC)
		session.CreatedAt = session.UpdatedAt
		session.ID = fmt.Sprintf("id-%02d", i)
		session.Exchanges = []history.Exchange{
			{
				Attempt:     1,
				Kind:        "initial",
				UserMessage: session.Instruction,
				Response: &llm.Response{
					Type:        responseCommands,
					Explanation: fmt.Sprintf("explanation %02d", i),
					Commands: []llm.Command{
						{Command: fmt.Sprintf("echo generated-%02d", i)},
					},
				},
				CreatedAt: session.UpdatedAt,
			},
		}
		session.Executions = []history.CommandAttempt{
			{
				Attempt:    1,
				Index:      0,
				Command:    fmt.Sprintf("echo executed-%02d", i),
				Confirmed:  true,
				ExitCode:   i % 2,
				Stdout:     fmt.Sprintf("stdout %02d", i),
				Stderr:     fmt.Sprintf("stderr %02d", i),
				StartedAt:  session.UpdatedAt,
				DurationMS: int64(i + 1),
			},
		}
		if err := history.Save(session); err != nil {
			t.Fatalf("history.Save(%d): %v", i, err)
		}
	}

	out, err := runCmd(t, "history")
	if err != nil {
		t.Fatalf("history default list: %v", err)
	}
	if got := strings.Count(out, "  prompt: "); got != 10 {
		t.Fatalf("default history count = %d, want 10\n%s", got, out)
	}
	if strings.Contains(out, "  prompt: prompt 00") || strings.Contains(out, "  prompt: prompt 01") {
		t.Fatalf("default history output should omit oldest sessions:\n%s", out)
	}
	if !strings.Contains(out, "  prompt: prompt 11") {
		t.Fatalf("default history output missing newest session:\n%s", out)
	}

	out, err = runCmd(t, "history", "--count", "3")
	if err != nil {
		t.Fatalf("history --count 3: %v", err)
	}
	if got := strings.Count(out, "  prompt: "); got != 3 {
		t.Fatalf("history --count 3 entries = %d, want 3\n%s", got, out)
	}

	out, err = runCmd(t, "history", "--verbose", "--count", "1")
	if err != nil {
		t.Fatalf("history --verbose --count 1: %v", err)
	}
	for _, want := range []string{
		"  id: ",
		"  provider: ",
		"  shell: /bin/zsh (zsh)",
		"  directory: ",
		"  exchange: attempt=1 kind=initial",
		"  explanation: explanation 11",
		"  result: exit=1 skipped=false confirmed=true",
		"  stdout: stdout 11",
		"  stderr: stderr 11",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("verbose history output missing %q:\n%s", want, out)
		}
	}

	_, err = runCmd(t, "history", "--count", "0")
	if err == nil {
		t.Fatal("history --count 0 error = nil, want error")
	}
}

func TestHistoryListDoesNotTruncatePromptOrCommand(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Model = "gpt-4o-mini"

	longPrompt := "on the remote server my-server in the ~/database-backup download the latest mysql-backup-yyyy-mm-dd.sql.gz file without shortening any part of this request because the list output should keep the whole prompt"
	longCommand := "scp kris@137.184.10.103:~/database-backup/$(ssh kris@137.184.10.103 'cd ~/database-backup && ls -t mysql-backup-*.sql.gz | head -n 1') ."

	session := history.NewSession(longPrompt, t.TempDir(), cfg, shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh"})
	session.Status = "completed"
	session.Executions = []history.CommandAttempt{{Attempt: 1, Index: 0, Command: longCommand, ExitCode: 0, StartedAt: time.Now()}}
	if err := history.Save(session); err != nil {
		t.Fatalf("history.Save(session): %v", err)
	}

	out, err := runCmd(t, "history", "--count", "1")
	if err != nil {
		t.Fatalf("history --count 1: %v", err)
	}
	if !strings.Contains(out, "  prompt: "+longPrompt) {
		t.Fatalf("history prompt was truncated:\n%s", out)
	}
	if !strings.Contains(out, "  command: "+longCommand) {
		t.Fatalf("history command was truncated:\n%s", out)
	}
}

func TestHistoryVerboseDoesNotTruncateExplanation(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Model = "gpt-4o-mini"

	explanation := "this explanation should remain completely intact in verbose mode even when it is long enough that the one-line helper would normally shorten it for display in other fields"
	session := history.NewSession("prompt", t.TempDir(), cfg, shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh"})
	session.Exchanges = []history.Exchange{
		{
			Attempt:     1,
			Kind:        "retry",
			UserMessage: "prompt",
			Response: &llm.Response{
				Type:        responseCommands,
				Explanation: explanation,
				Commands: []llm.Command{
					{Command: "echo ok"},
				},
			},
			CreatedAt: time.Now(),
		},
	}
	if err := history.Save(session); err != nil {
		t.Fatalf("history.Save(session): %v", err)
	}

	out, err := runCmd(t, "history", "--verbose", "--count", "1")
	if err != nil {
		t.Fatalf("history --verbose --count 1: %v", err)
	}
	if !strings.Contains(out, "  explanation: "+explanation) {
		t.Fatalf("verbose explanation was truncated:\n%s", out)
	}
}

func TestHistoryRemoveAndClear(t *testing.T) {
	tempHome(t)
	sessions := seedHistory(t, "remove me", "keep me")

	if _, err := runCmd(t, "history", "remove", sessions[0].ID); err != nil {
		t.Fatalf("history remove: %v", err)
	}

	out, err := runCmd(t, "history", "list")
	if err != nil {
		t.Fatalf("history list after remove: %v", err)
	}
	if strings.Contains(out, "remove me") {
		t.Fatalf("history remove did not remove session:\n%s", out)
	}

	if _, err := runCmd(t, "history", "clear"); err != nil {
		t.Fatalf("history clear: %v", err)
	}

	out, err = runCmd(t, "history")
	if err != nil {
		t.Fatalf("history default list after clear: %v", err)
	}
	if !strings.Contains(out, "No AI history stored.") {
		t.Fatalf("history clear output unexpected:\n%s", out)
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

// --- memory commands ---

func TestMemoryAddListRemove(t *testing.T) {
	tempHome(t)

	// Add a memory
	if _, err := runCmd(t, "memory", commandAdd, "docker", "always use docker compose v2"); err != nil {
		t.Fatalf("memory add: %v", err)
	}

	// List memories
	out, err := runCmd(t, "memory", "list")
	if err != nil {
		t.Fatalf("memory list: %v", err)
	}
	if !strings.Contains(out, "docker") {
		t.Errorf("memory list output missing 'docker'\n%s", out)
	}
	if !strings.Contains(out, "always use docker compose v2") {
		t.Errorf("memory list output missing content\n%s", out)
	}

	// Remove the memory
	if _, err := runCmd(t, "memory", "remove", "docker"); err != nil {
		t.Fatalf("memory remove: %v", err)
	}

	// List should now be empty
	out, err = runCmd(t, "memory", "list")
	if err != nil {
		t.Fatalf("memory list after remove: %v", err)
	}
	if !strings.Contains(out, "No memories stored") {
		t.Errorf("expected 'No memories stored' after remove\n%s", out)
	}
}

func TestMemoryListEmpty(t *testing.T) {
	tempHome(t)

	out, err := runCmd(t, "memory", "list")
	if err != nil {
		t.Fatalf("memory list: %v", err)
	}
	if !strings.Contains(out, "No memories stored") {
		t.Errorf("expected 'No memories stored'\n%s", out)
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

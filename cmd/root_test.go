package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/interactive"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

func captureCmdOutput(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func withCmdStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	_ = w.Close()

	fn()
}

func TestRunRoot_LoadError(t *testing.T) {
	tempHome(t)
	debugFlag = ""

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runRoot(nil, []string{"test"})
	if err == nil {
		t.Fatal("runRoot() error = nil, want config load error")
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("error = %q, want failed to load config", err.Error())
	}
}

func TestRunRootReturnsNoAPIKeyError(t *testing.T) {
	tempHome(t)
	debugFlag = ""

	err := runRoot(nil, nil)
	if err == nil {
		t.Fatal("runRoot() error = nil, want missing API key error")
	}
	if !strings.Contains(err.Error(), "no API key configured") {
		t.Fatalf("runRoot() error = %q, want missing API key error", err.Error())
	}
}

func TestRunRootSingleShotReturnsNoAPIKeyError(t *testing.T) {
	tempHome(t)
	debugFlag = ""

	err := runRoot(nil, []string{"list", "files"})
	if err == nil {
		t.Fatal("runRoot() error = nil, want missing API key error")
	}
	if !strings.Contains(err.Error(), "no API key configured") {
		t.Fatalf("runRoot() error = %q, want missing API key error", err.Error())
	}
}

// --- interactive mode tests ---

func TestRunRoot_InteractiveMode_ConfigRunShow(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test-interactive"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	origRun := interactiveRun
	t.Cleanup(func() { interactiveRun = origRun })

	var capturedCmds interactive.BuiltinCommands
	interactiveRun = func(_ string, cmds interactive.BuiltinCommands, _ *config.Config, _ llm.Client, _ shell.Info, _ bool) error {
		capturedCmds = cmds
		return nil
	}

	debugFlag = ""
	if err := runRoot(nil, nil); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}

	// Test ConfigRun: show
	if err := capturedCmds.ConfigRun([]string{"show"}); err != nil {
		t.Fatalf("ConfigRun(show) error: %v", err)
	}

	// Test ConfigRun: get
	if err := capturedCmds.ConfigRun([]string{"get", "model"}); err != nil {
		t.Fatalf("ConfigRun(get model) error: %v", err)
	}

	// Test ConfigRun: set
	if err := capturedCmds.ConfigRun([]string{"set", "model", "gpt-4o"}); err != nil {
		t.Fatalf("ConfigRun(set model) error: %v", err)
	}

	// Test ConfigRun: no args
	if err := capturedCmds.ConfigRun(nil); err == nil {
		t.Fatal("ConfigRun(nil) error = nil, want error")
	}

	// Test ConfigRun: unknown subcommand
	if err := capturedCmds.ConfigRun([]string{"unknown"}); err == nil {
		t.Fatal("ConfigRun(unknown) error = nil, want error")
	}

	// Test ConfigRun: get missing key
	if err := capturedCmds.ConfigRun([]string{"get"}); err == nil {
		t.Fatal("ConfigRun(get) error = nil, want error")
	}

	// Test ConfigRun: set missing value
	if err := capturedCmds.ConfigRun([]string{"set", "model"}); err == nil {
		t.Fatal("ConfigRun(set model) error = nil, want error")
	}
}

func TestRunRoot_InteractiveMode_MemoryRun(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test-interactive"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	origRun := interactiveRun
	t.Cleanup(func() { interactiveRun = origRun })

	var capturedCmds interactive.BuiltinCommands
	interactiveRun = func(_ string, cmds interactive.BuiltinCommands, _ *config.Config, _ llm.Client, _ shell.Info, _ bool) error {
		capturedCmds = cmds
		return nil
	}

	debugFlag = ""
	if err := runRoot(nil, nil); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}

	// Test MemoryRun: list (no args defaults to list)
	if err := capturedCmds.MemoryRun(nil); err != nil {
		t.Fatalf("MemoryRun(nil) error: %v", err)
	}

	// Test MemoryRun: list
	if err := capturedCmds.MemoryRun([]string{"list"}); err != nil {
		t.Fatalf("MemoryRun(list) error: %v", err)
	}

	// Test MemoryRun: add
	if err := capturedCmds.MemoryRun([]string{"add", "docker", "use compose v2"}); err != nil {
		t.Fatalf("MemoryRun(add) error: %v", err)
	}

	// Test MemoryRun: remove
	if err := capturedCmds.MemoryRun([]string{"remove", "docker"}); err != nil {
		t.Fatalf("MemoryRun(remove) error: %v", err)
	}

	// Test MemoryRun: add missing args
	if err := capturedCmds.MemoryRun([]string{"add"}); err == nil {
		t.Fatal("MemoryRun(add) error = nil, want error")
	}

	// Test MemoryRun: remove missing args
	if err := capturedCmds.MemoryRun([]string{"remove"}); err == nil {
		t.Fatal("MemoryRun(remove) error = nil, want error")
	}

	// Test MemoryRun: unknown subcommand
	if err := capturedCmds.MemoryRun([]string{"unknown"}); err == nil {
		t.Fatal("MemoryRun(unknown) error = nil, want error")
	}
}

func TestRunRoot_InteractiveMode_StatusDoctorSetModel(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test-interactive"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	origRun := interactiveRun
	t.Cleanup(func() { interactiveRun = origRun })

	var capturedCmds interactive.BuiltinCommands
	interactiveRun = func(_ string, cmds interactive.BuiltinCommands, _ *config.Config, _ llm.Client, _ shell.Info, _ bool) error {
		capturedCmds = cmds
		return nil
	}

	debugFlag = ""
	if err := runRoot(nil, nil); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}

	// Test Status callback
	if err := capturedCmds.Status(); err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	// Test Doctor callback
	t.Setenv("SHELL", "")
	if err := capturedCmds.Doctor(); err != nil {
		t.Fatalf("Doctor() error: %v", err)
	}
}

func TestRunRoot_InteractiveError(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test-interactive"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	origRun := interactiveRun
	t.Cleanup(func() { interactiveRun = origRun })

	interactiveRun = func(_ string, _ interactive.BuiltinCommands, _ *config.Config, _ llm.Client, _ shell.Info, _ bool) error {
		return errors.New("interactive failed")
	}

	debugFlag = ""
	err := runRoot(nil, nil)
	if err == nil {
		t.Fatal("runRoot() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "interactive failed") {
		t.Fatalf("error = %q, want interactive error", err.Error())
	}
}

func TestRunRoot_InteractiveMode_ExplainFlag(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test-interactive"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	origRun := interactiveRun
	origExplain := explainFlag
	t.Cleanup(func() {
		interactiveRun = origRun
		explainFlag = origExplain
	})

	var capturedExplain bool
	interactiveRun = func(_ string, _ interactive.BuiltinCommands, _ *config.Config, _ llm.Client, _ shell.Info, explain bool) error {
		capturedExplain = explain
		return nil
	}

	debugFlag = ""
	explainFlag = false
	if err := runRoot(nil, nil); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
	if capturedExplain {
		t.Fatal("explain = true, want false (default)")
	}

	explainFlag = true
	if err := runRoot(nil, nil); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
	if !capturedExplain {
		t.Fatal("explain = false, want true")
	}
}

func TestRunRoot_DebugFileError(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Set debugFlag to "file" but make the config dir unwritable
	debugFlag = "file"
	defer func() { debugFlag = "" }()

	// Remove the config dir to cause an error
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".ai-cli")
	os.RemoveAll(configDir)
	// Create a file at the dir path so MkdirAll fails
	if err := os.WriteFile(configDir, []byte("block"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := runRoot(nil, []string{"test"})
	if err == nil {
		t.Fatal("runRoot() error = nil, want debug file error")
	}
}

func TestRunRootSingleShot_LocalProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ollamaResp := map[string]interface{}{
			"response": `{"type":"commands","commands":[]}`,
			"done":     true,
		}
		if err := json.NewEncoder(w).Encode(ollamaResp); err != nil {
			t.Fatalf("Encode response: %v", err)
		}
	}))
	defer server.Close()

	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderLocal
	cfg.Provider.Local.BaseURL = server.URL
	cfg.Provider.Model = "llama3"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	debugFlag = ""
	if err := runRoot(nil, []string{"list", "files"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}

func TestRunRoot_ChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.OpenAI.APIKey = "sk-test"
	cfg.Provider.OpenAI.BaseURL = server.URL
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	debugFlag = ""
	err := runRoot(nil, []string{"test"})
	if err == nil {
		t.Fatal("runRoot() error = nil, want chat error")
	}
}

func TestHandleConfig_DefaultApply(t *testing.T) {
	// Test that pressing Enter (empty input) applies the config change
	tempHome(t)
	cfg := config.DefaultConfig()

	withCmdStdin(t, "\n", func() {
		if err := handleConfig(&llm.Response{Action: "set_model", Value: "gpt-4o"}, cfg); err != nil {
			t.Fatalf("handleConfig() error: %v", err)
		}
	})

	if cfg.Provider.Model != "gpt-4o" {
		t.Fatalf("model = %q, want %q", cfg.Provider.Model, "gpt-4o")
	}
}

func TestHandleConfig_YesApply(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()

	withCmdStdin(t, "yes\n", func() {
		if err := handleConfig(&llm.Response{Action: "set_model", Value: "gpt-4o"}, cfg); err != nil {
			t.Fatalf("handleConfig() error: %v", err)
		}
	})

	if cfg.Provider.Model != "gpt-4o" {
		t.Fatalf("model = %q, want %q", cfg.Provider.Model, "gpt-4o")
	}
}

func TestHandleConfig(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		tempHome(t)
		cfg := config.DefaultConfig()
		orig := cfg.Provider.Model

		withCmdStdin(t, "n\n", func() {
			if err := handleConfig(&llm.Response{Action: "set_model", Value: "gpt-4o"}, cfg); err != nil {
				t.Fatalf("handleConfig() error: %v", err)
			}
		})

		if cfg.Provider.Model != orig {
			t.Fatalf("model changed on skip: got %q want %q", cfg.Provider.Model, orig)
		}
	})

	t.Run("apply", func(t *testing.T) {
		tempHome(t)
		cfg := config.DefaultConfig()

		withCmdStdin(t, "y\n", func() {
			if err := handleConfig(&llm.Response{Action: "set_model", Value: "gpt-4o-mini"}, cfg); err != nil {
				t.Fatalf("handleConfig() error: %v", err)
			}
		})

		if cfg.Provider.Model != "gpt-4o-mini" {
			t.Fatalf("model = %q, want %q", cfg.Provider.Model, "gpt-4o-mini")
		}
	})

	t.Run("apply action error", func(t *testing.T) {
		tempHome(t)
		cfg := config.DefaultConfig()

		withCmdStdin(t, "y\n", func() {
			err := handleConfig(&llm.Response{Action: "invalid_action", Key: "x", Value: "y"}, cfg)
			if err == nil {
				t.Fatal("handleConfig() error = nil, want error")
			}
			if !strings.Contains(err.Error(), "unknown config action") {
				t.Fatalf("handleConfig() error = %q, want ApplyAction error", err.Error())
			}
		})
	})
}

func TestHandleConfig_RedactsDisplayedKeyValue(t *testing.T) {
	tempHome(t)
	cfg := config.DefaultConfig()

	out := captureCmdOutput(t, func() {
		withCmdStdin(t, "n\n", func() {
			if err := handleConfig(&llm.Response{Action: "set_key", Key: "llm_key", Value: "sk-super-secret-1234"}, cfg); err != nil {
				t.Fatalf("handleConfig() error: %v", err)
			}
		})
	})

	if strings.Contains(out, "sk-super-secret-1234") {
		t.Fatalf("handleConfig leaked raw key in output: %q", out)
	}
	if !strings.Contains(out, config.RedactSecret("sk-super-secret-1234")) {
		t.Fatalf("handleConfig should show masked key in output: %q", out)
	}
}

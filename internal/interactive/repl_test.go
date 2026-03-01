package interactive

import (
	"os"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	_ = w.Close()

	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	fn()
}

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	return config.DefaultConfig()
}

func TestPrintHelp(_ *testing.T) {
	printHelp()
}

func TestHandleResponseCommands(t *testing.T) {
	err := handleResponse(&llm.Response{Type: "commands"}, testCfg(t), shell.Info{Shell: "cmd"}, false)
	if err != nil {
		t.Fatalf("handleResponse(commands) error = %v", err)
	}
}

func TestHandleResponseConfig(t *testing.T) {
	cfg := testCfg(t)
	resp := &llm.Response{
		Type:   "config",
		Action: "set_model",
		Key:    "",
		Value:  "gpt-4o-mini",
	}

	withStdin(t, "y\n", func() {
		if err := handleResponse(resp, cfg, shell.Info{}, false); err != nil {
			t.Fatalf("handleResponse(config) error = %v", err)
		}
	})

	if cfg.Provider.Model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want %q", cfg.Provider.Model, "gpt-4o-mini")
	}
}

func TestHandleResponseUnknownType(t *testing.T) {
	err := handleResponse(&llm.Response{Type: "mystery"}, testCfg(t), shell.Info{}, false)
	if err == nil {
		t.Fatal("handleResponse() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown response type") {
		t.Fatalf("handleResponse() error = %q, want unknown type error", err.Error())
	}
}

func TestApplyConfigSkip(t *testing.T) {
	cfg := testCfg(t)
	original := cfg.Provider.Model

	resp := &llm.Response{
		Action: "set_model",
		Value:  "should-not-apply",
	}
	withStdin(t, "n\n", func() {
		if err := applyConfig(resp, cfg); err != nil {
			t.Fatalf("applyConfig() error = %v", err)
		}
	})

	if cfg.Provider.Model != original {
		t.Fatalf("model changed on skip: got %q want %q", cfg.Provider.Model, original)
	}
}

func TestApplyConfigReturnsApplyActionError(t *testing.T) {
	cfg := testCfg(t)
	resp := &llm.Response{
		Action: "invalid_action",
		Key:    "",
		Value:  "value",
	}

	withStdin(t, "y\n", func() {
		err := applyConfig(resp, cfg)
		if err == nil {
			t.Fatal("applyConfig() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "unknown config action") {
			t.Fatalf("applyConfig() error = %q, want ApplyAction error", err.Error())
		}
	})
}

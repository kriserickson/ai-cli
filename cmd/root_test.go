package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

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

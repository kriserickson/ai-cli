package cmd

import (
	"os"
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

package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

func captureStdout(t *testing.T, fn func()) string {
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

func writeConfigForDoctor(t *testing.T, mutate func(*config.Config)) *config.Config {
	t.Helper()
	tempHome(t)

	cfg := config.DefaultConfig()
	if mutate != nil {
		mutate(cfg)
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	return cfg
}

func TestPrintCheckAndWarning(t *testing.T) {
	out := captureStdout(t, func() {
		printCheck("Label", true, "ok detail")
		printCheck("Label", false, "bad detail")
		printWarning("Warn", "warning detail")
	})

	for _, want := range []string{"Label", "ok detail", "bad detail", "Warn", "warning detail"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
}

func TestRunDoctor_AllChecksPassWithAPIKey(t *testing.T) {
	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenAI
		cfg.Provider.OpenAI.APIKey = "sk-test-doctor-123456"
		cfg.Provider.Model = "gpt-4o-mini"
	})
	t.Setenv("SHELL", "")

	out := captureStdout(t, func() {
		if err := runDoctor(nil, nil); err != nil {
			t.Fatalf("runDoctor() error: %v", err)
		}
	})

	for _, want := range []string{"Checking configuration...", "Config file", "API key", "Model", "All checks passed!"} {
		if !strings.Contains(out, want) {
			t.Fatalf("runDoctor output missing %q\n%s", want, out)
		}
	}
}

func TestCheckNoglobAlias(t *testing.T) {
	t.Run("non-zsh shell skips check", func(t *testing.T) {
		tempHome(t)
		t.Setenv("SHELL", "/bin/bash")
		if got := checkNoglobAlias(); !got {
			t.Fatal("checkNoglobAlias() = false, want true for non-zsh shell")
		}
	})

	t.Run("missing shell env skips check", func(t *testing.T) {
		tempHome(t)
		t.Setenv("SHELL", "")
		if got := checkNoglobAlias(); !got {
			t.Fatal("checkNoglobAlias() = false, want true when SHELL is empty")
		}
	})

	t.Run("zsh missing zshrc warns", func(t *testing.T) {
		tempHome(t)
		t.Setenv("SHELL", "/bin/zsh")
		if got := checkNoglobAlias(); got {
			t.Fatal("checkNoglobAlias() = true, want false when ~/.zshrc is missing")
		}
	})

	t.Run("zsh with alias passes", func(t *testing.T) {
		tempHome(t)
		t.Setenv("SHELL", "/bin/zsh")
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("alias ai='noglob ai'\n"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if got := checkNoglobAlias(); !got {
			t.Fatal("checkNoglobAlias() = false, want true when alias is present")
		}
	})

	t.Run("zsh without alias warns", func(t *testing.T) {
		tempHome(t)
		t.Setenv("SHELL", "/bin/zsh")
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("os.UserHomeDir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("export PATH=$PATH:/usr/local/bin\n"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if got := checkNoglobAlias(); got {
			t.Fatal("checkNoglobAlias() = true, want false when alias is absent")
		}
	})
}

func TestRunDoctorSkipsNoglobCheckOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific behavior")
	}

	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.OpenRouter.APIKey = "sk-or-test-123456"
	})
	t.Setenv("SHELL", "/bin/zsh")

	if err := runDoctor(nil, nil); err != nil {
		t.Fatalf("runDoctor() error: %v", err)
	}
}

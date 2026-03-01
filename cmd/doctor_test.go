package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
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

func writeConfigForDoctor(t *testing.T, mutate func(*config.Config)) {
	t.Helper()
	tempHome(t)

	cfg := config.DefaultConfig()
	if mutate != nil {
		mutate(cfg)
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
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

func TestRunDoctor_MissingAPIKeyRunsWizard(t *testing.T) {
	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenAI
		cfg.Provider.OpenAI.APIKey = ""
		cfg.Provider.Model = "gpt-4o-mini"
	})
	t.Setenv("SHELL", "")

	stubWizardHooks(t)
	call := 0
	wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		switch call {
		case 0: // provider
			*(response.(*int)) = 0 // OpenAI
		case 1: // API key prompt (since it's missing)
			*(response.(*string)) = "sk-wizard-key"
		case 2: // model
			*(response.(*int)) = 0
		}
		call++
		return nil
	}
	wizardFetchOpenAIModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
		return []llm.ModelInfo{{ID: "gpt-4o", Name: "gpt-4o", Company: "OpenAI"}}, nil
	}
	wizardSaveConfig = func(_ *config.Config) error {
		return nil
	}

	out := captureStdout(t, func() {
		if err := runDoctor(nil, nil); err != nil {
			t.Fatalf("runDoctor() error: %v", err)
		}
	})

	if !strings.Contains(out, "Running setup wizard") {
		t.Fatalf("runDoctor output should mention setup wizard\n%s", out)
	}
}

func TestRunDoctor_MissingAPIKeyWizardError(t *testing.T) {
	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenRouter
		cfg.Provider.OpenRouter.APIKey = ""
	})
	t.Setenv("SHELL", "")

	stubWizardHooks(t)
	wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
		return errors.New("wizard canceled")
	}

	captureStdout(t, func() {
		err := runDoctor(nil, nil)
		if err == nil {
			t.Fatal("runDoctor() error = nil, want wizard error")
		}
		if !strings.Contains(err.Error(), "wizard canceled") {
			t.Fatalf("error = %q, want wizard error", err.Error())
		}
	})
}

func TestRunDoctor_OpenRouterAPIKeyPresent(t *testing.T) {
	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenRouter
		cfg.Provider.OpenRouter.APIKey = "sk-or-test-doctor-1234"
		cfg.Provider.Model = "anthropic/claude-3.5-sonnet"
	})
	t.Setenv("SHELL", "")

	out := captureStdout(t, func() {
		if err := runDoctor(nil, nil); err != nil {
			t.Fatalf("runDoctor() error: %v", err)
		}
	})

	if !strings.Contains(out, "openrouter") {
		t.Fatalf("runDoctor output should mention openrouter\n%s", out)
	}
	if !strings.Contains(out, "All checks passed!") {
		t.Fatalf("runDoctor output missing 'All checks passed!'\n%s", out)
	}
}

func TestRunDoctor_WithWarnings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("noglob check only runs on non-windows")
	}

	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenAI
		cfg.Provider.OpenAI.APIKey = "sk-test-doctor-123456"
	})
	t.Setenv("SHELL", "/bin/zsh")
	// The temp home won't have a .zshrc, so the noglob check should produce a warning

	out := captureStdout(t, func() {
		if err := runDoctor(nil, nil); err != nil {
			t.Fatalf("runDoctor() error: %v", err)
		}
	})

	if !strings.Contains(out, "recommendation") {
		t.Fatalf("runDoctor output should mention recommendations\n%s", out)
	}
}

func TestRunDoctor_LoadError(t *testing.T) {
	tempHome(t)
	t.Setenv("SHELL", "")

	// Corrupt the config file
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() {
		err := runDoctor(nil, nil)
		if err == nil {
			t.Fatal("runDoctor() error = nil, want load error")
		}
	})
}

func TestRunDoctor_LocalProviderWithBaseURL(t *testing.T) {
	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderLocal
		cfg.Provider.Local.BaseURL = "http://localhost:11434/api/generate"
		cfg.Provider.Model = "llama3"
	})
	t.Setenv("SHELL", "")

	out := captureStdout(t, func() {
		if err := runDoctor(nil, nil); err != nil {
			t.Fatalf("runDoctor() error: %v", err)
		}
	})

	if !strings.Contains(out, "Local server") {
		t.Fatalf("runDoctor output should mention Local server\n%s", out)
	}
	if !strings.Contains(out, "All checks passed!") {
		t.Fatalf("runDoctor output missing 'All checks passed!'\n%s", out)
	}
}

func TestRunDoctor_LocalProviderNoBaseURL(t *testing.T) {
	writeConfigForDoctor(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderLocal
		cfg.Provider.Local.BaseURL = ""
		cfg.Provider.Model = "llama3"
	})
	t.Setenv("SHELL", "")

	stubWizardHooks(t)
	call := 0
	wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		switch call {
		case 0: // provider
			*(response.(*int)) = 0 // OpenAI
		case 1: // API key prompt
			*(response.(*string)) = "sk-wizard-key"
		case 2: // model
			*(response.(*int)) = 0
		}
		call++
		return nil
	}
	wizardFetchOpenAIModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
		return []llm.ModelInfo{{ID: "gpt-4o", Name: "gpt-4o", Company: "OpenAI"}}, nil
	}
	wizardSaveConfig = func(_ *config.Config) error {
		return nil
	}

	out := captureStdout(t, func() {
		if err := runDoctor(nil, nil); err != nil {
			t.Fatalf("runDoctor() error: %v", err)
		}
	})

	if !strings.Contains(out, "No base URL") {
		t.Fatalf("runDoctor output should mention missing base URL\n%s", out)
	}
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

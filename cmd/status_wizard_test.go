package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

func saveCmdConfig(t *testing.T, mutate func(*config.Config)) *config.Config {
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

func TestRunStatus_LoadError(t *testing.T) {
	tempHome(t)

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600)

	err := runStatus(nil, nil)
	if err == nil {
		t.Fatal("runStatus() error = nil, want load error")
	}
}

func TestRunStatus_NoLog_APIKeyMissing(t *testing.T) {
	saveCmdConfig(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenRouter
		cfg.Provider.OpenRouter.APIKey = ""
		cfg.Provider.Model = "anthropic/test-model"
	})

	out := captureStdout(t, func() {
		if err := runStatus(nil, nil); err != nil {
			t.Fatalf("runStatus() error: %v", err)
		}
	})

	for _, want := range []string{"Config:", "Log:", "not created yet", "Provider: openrouter", "Model:    anthropic/test-model", "API Key:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("runStatus output missing %q\n%s", want, out)
		}
	}
	if !strings.Contains(out, "not set") {
		t.Fatalf("runStatus output should show missing API key\n%s", out)
	}
}

func TestRunStatus_LogExists_APIKeyPresent(t *testing.T) {
	saveCmdConfig(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenAI
		cfg.Provider.OpenAI.APIKey = "sk-status-test-123456789"
		cfg.Provider.Model = "gpt-4o-mini"
	})

	configDir, err := config.ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir(): %v", err)
	}
	logPath := filepath.Join(configDir, "llm.log")
	if err := os.WriteFile(logPath, []byte("log"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", logPath, err)
	}

	out := captureStdout(t, func() {
		if err := runStatus(nil, nil); err != nil {
			t.Fatalf("runStatus() error: %v", err)
		}
	})

	for _, want := range []string{"Log:", "exists", "Provider: openai", "Model:    gpt-4o-mini", "API Key:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("runStatus output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "not set") {
		t.Fatalf("runStatus output should show masked key, not 'not set'\n%s", out)
	}
}

func TestSelectFromList_NoOptions(t *testing.T) {
	_, err := selectFromList("pick one", nil)
	if err == nil {
		t.Fatal("selectFromList() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "no options available") {
		t.Fatalf("selectFromList() error = %q, want no options error", err.Error())
	}
}

func TestEnsureAPIKey_ExistingKeySkipsPrompt(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		setup    func(*config.Config)
		check    func(*config.Config) string
	}{
		{
			name:     "openai existing key",
			provider: config.ProviderOpenAI,
			setup: func(cfg *config.Config) {
				cfg.Provider.OpenAI.APIKey = "sk-openai-existing"
			},
			check: func(cfg *config.Config) string {
				if cfg.Provider.OpenAI.APIKey != "sk-openai-existing" {
					return "openai key changed unexpectedly"
				}
				return ""
			},
		},
		{
			name:     "openrouter existing key",
			provider: config.ProviderOpenRouter,
			setup: func(cfg *config.Config) {
				cfg.Provider.OpenRouter.APIKey = "sk-openrouter-existing"
			},
			check: func(cfg *config.Config) string {
				if cfg.Provider.OpenRouter.APIKey != "sk-openrouter-existing" {
					return "openrouter key changed unexpectedly"
				}
				return ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tt.setup(cfg)
			if err := ensureAPIKey(cfg, tt.provider); err != nil {
				t.Fatalf("ensureAPIKey() error: %v", err)
			}
			if msg := tt.check(cfg); msg != "" {
				t.Fatal(msg)
			}
		})
	}
}

func TestPickModel_UnknownProvider(t *testing.T) {
	_, err := pickModel(config.DefaultConfig(), "unknown")
	if err == nil {
		t.Fatal("pickModel() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("pickModel() error = %q, want unknown provider error", err.Error())
	}
}

func TestPickModel_FetchErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.BaseURL = "://bad-url"
	cfg.Provider.OpenAI.APIKey = "sk-test"
	cfg.Provider.OpenRouter.BaseURL = "://bad-url"
	cfg.Provider.OpenRouter.APIKey = "sk-test"

	tests := []struct {
		name     string
		provider string
	}{
		{name: "openai fetch error", provider: config.ProviderOpenAI},
		{name: "openrouter fetch error", provider: config.ProviderOpenRouter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pickModel(cfg, tt.provider)
			if err == nil {
				t.Fatal("pickModel() error = nil, want fetch error")
			}
			if !strings.Contains(err.Error(), "fetch models:") {
				t.Fatalf("pickModel() error = %q, want wrapped fetch error", err.Error())
			}
		})
	}
}

func TestRunStatus_OpenAIWithAPIKey(t *testing.T) {
	saveCmdConfig(t, func(cfg *config.Config) {
		cfg.Provider.Default = config.ProviderOpenAI
		cfg.Provider.OpenAI.APIKey = "sk-status-openai-123456"
		cfg.Provider.Model = "gpt-4o"
	})

	out := captureStdout(t, func() {
		if err := runStatus(nil, nil); err != nil {
			t.Fatalf("runStatus() error: %v", err)
		}
	})

	if !strings.Contains(out, "Provider: openai") {
		t.Fatalf("expected Provider: openai\n%s", out)
	}
	if !strings.Contains(out, "Model:    gpt-4o") {
		t.Fatalf("expected Model: gpt-4o\n%s", out)
	}
}

func TestRunSetModel_LoadError(t *testing.T) {
	tempHome(t)

	// Corrupt the config file
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".ai-cli")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{invalid"), 0o600)

	err := runSetModel(nil, nil)
	if err == nil {
		t.Fatal("runSetModel() error = nil, want config load error")
	}
}

func TestRunSetModel_LoadsConfigAndRunsWizard(t *testing.T) {
	tempHome(t)

	// Save a valid config
	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.APIKey = "sk-test-set-model"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	stubWizardHooks(t)
	call := 0
	wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		switch call {
		case 0: // provider
			*(response.(*int)) = 0 // OpenAI
		case 1: // model
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

	if err := runSetModel(nil, nil); err != nil {
		t.Fatalf("runSetModel() error: %v", err)
	}
}

func TestPickModel_SelectCompanyError(t *testing.T) {
	stubWizardHooks(t)
	wizardFetchORModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
		return []llm.ModelInfo{
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude", Company: "Anthropic"},
		}, nil
	}
	wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
		return errors.New("user canceled company select")
	}

	cfg := config.DefaultConfig()
	cfg.Provider.OpenRouter.BaseURL = "https://example.test"
	cfg.Provider.OpenRouter.APIKey = "sk-test"
	_, err := pickModel(cfg, config.ProviderOpenRouter)
	if err == nil {
		t.Fatal("pickModel() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "user canceled company select") {
		t.Fatalf("error = %q, want company select error", err.Error())
	}
}

func TestPickModel_SelectModelError(t *testing.T) {
	stubWizardHooks(t)
	wizardFetchORModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
		return []llm.ModelInfo{
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude", Company: "Anthropic"},
		}, nil
	}
	call := 0
	wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		if call == 0 {
			*(response.(*int)) = 0 // company select
			call++
			return nil
		}
		return errors.New("user canceled model select")
	}

	cfg := config.DefaultConfig()
	cfg.Provider.OpenRouter.BaseURL = "https://example.test"
	cfg.Provider.OpenRouter.APIKey = "sk-test"
	_, err := pickModel(cfg, config.ProviderOpenRouter)
	if err == nil {
		t.Fatal("pickModel() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "user canceled model select") {
		t.Fatalf("error = %q, want model select error", err.Error())
	}
}

func TestPickModel_OpenAISelectError(t *testing.T) {
	stubWizardHooks(t)
	wizardFetchOpenAIModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
		return []llm.ModelInfo{{ID: "gpt-4o", Name: "gpt-4o", Company: "OpenAI"}}, nil
	}
	wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
		return errors.New("canceled")
	}

	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.BaseURL = "https://example.test"
	cfg.Provider.OpenAI.APIKey = "sk-test"
	_, err := pickModel(cfg, config.ProviderOpenAI)
	if err == nil {
		t.Fatal("pickModel() error = nil, want error")
	}
}

func TestPickModel_NoModelsBranches(t *testing.T) {
	t.Run("openrouter no models", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
		}))
		defer server.Close()

		cfg := config.DefaultConfig()
		cfg.Provider.OpenRouter.BaseURL = server.URL
		cfg.Provider.OpenRouter.APIKey = "sk-test"

		_, err := pickModel(cfg, config.ProviderOpenRouter)
		if err == nil {
			t.Fatal("pickModel() error = nil, want no models error")
		}
		if !strings.Contains(err.Error(), "no models available") {
			t.Fatalf("pickModel() error = %q, want no models error", err.Error())
		}
	})

	t.Run("openai no gpt models", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"text-embedding-3-small","created":1}]}`))
		}))
		defer server.Close()

		cfg := config.DefaultConfig()
		cfg.Provider.OpenAI.BaseURL = server.URL
		cfg.Provider.OpenAI.APIKey = "sk-test"

		_, err := pickModel(cfg, config.ProviderOpenAI)
		if err == nil {
			t.Fatal("pickModel() error = nil, want no GPT models error")
		}
		if !strings.Contains(err.Error(), "no GPT models available") {
			t.Fatalf("pickModel() error = %q, want no GPT models error", err.Error())
		}
	})
}

package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

func saveCmdConfig(t *testing.T, mutate func(*config.Config)) {
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

	for _, want := range []string{"Config:", "Log:", "not created yet", "Provider: openrouter", "Model:    anthropic/test-model", "Shell:", "API Key:"} {
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

	for _, want := range []string{"Log:", "exists", "Provider: openai", "Model:    gpt-4o-mini", "Shell:", "API Key:"} {
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

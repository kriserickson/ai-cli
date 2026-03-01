package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

func stubWizardHooks(t *testing.T) {
	t.Helper()

	origAskOne := wizardAskOne
	origFetchAI := wizardFetchOpenAIModels
	origFetchOR := wizardFetchORModels
	origFetchLocal := wizardFetchLocalModels
	origSave := wizardSaveConfig

	t.Cleanup(func() {
		wizardAskOne = origAskOne
		wizardFetchOpenAIModels = origFetchAI
		wizardFetchORModels = origFetchOR
		wizardFetchLocalModels = origFetchLocal
		wizardSaveConfig = origSave
	})
}

func TestSelectFromList_SuccessAndError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			sel, ok := p.(*survey.Select)
			if !ok {
				t.Fatalf("prompt type = %T, want *survey.Select", p)
			}
			if sel.Message != "Pick:" {
				t.Fatalf("select message = %q, want %q", sel.Message, "Pick:")
			}
			ptr, ok := response.(*int)
			if !ok {
				t.Fatalf("response type = %T, want *int", response)
			}
			*ptr = 1
			return nil
		}

		got, err := selectFromList("Pick:", []string{"a", "b"})
		if err != nil {
			t.Fatalf("selectFromList() error = %v", err)
		}
		if got != 1 {
			t.Fatalf("selectFromList() = %d, want 1", got)
		}
	})

	t.Run("ask error", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
			return errors.New("prompt failed")
		}

		_, err := selectFromList("Pick:", []string{"a"})
		if err == nil {
			t.Fatal("selectFromList() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "prompt failed") {
			t.Fatalf("selectFromList() error = %q, want prompt error", err.Error())
		}
	})
}

func TestPickProvider(t *testing.T) {
	tests := []struct {
		name    string
		idx     int
		want    string
		wantErr bool
	}{
		{name: "openai", idx: 0, want: config.ProviderOpenAI},
		{name: "openrouter", idx: 1, want: config.ProviderOpenRouter},
		{name: "error", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubWizardHooks(t)
			wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
				if tt.wantErr {
					return errors.New("select failed")
				}
				*(response.(*int)) = tt.idx
				return nil
			}

			got, err := pickProvider()
			if tt.wantErr {
				if err == nil {
					t.Fatal("pickProvider() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("pickProvider() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("pickProvider() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPromptAPIKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			if _, ok := p.(*survey.Password); !ok {
				t.Fatalf("prompt type = %T, want *survey.Password", p)
			}
			*(response.(*string)) = "sk-test-key"
			return nil
		}

		got, err := promptAPIKey("openai")
		if err != nil {
			t.Fatalf("promptAPIKey() error = %v", err)
		}
		if got != "sk-test-key" {
			t.Fatalf("promptAPIKey() = %q, want %q", got, "sk-test-key")
		}
	})

	t.Run("error", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
			return errors.New("canceled")
		}

		_, err := promptAPIKey("openrouter")
		if err == nil {
			t.Fatal("promptAPIKey() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "canceled") {
			t.Fatalf("promptAPIKey() error = %q, want prompt error", err.Error())
		}
	})
}

func TestEnsureAPIKey_MissingKeyPromptsAndSets(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		check    func(*config.Config) string
	}{
		{
			name:     "openai",
			provider: config.ProviderOpenAI,
			check: func(cfg *config.Config) string {
				if cfg.Provider.OpenAI.APIKey != "sk-new-key" {
					return fmt.Sprintf("OpenAI key = %q", cfg.Provider.OpenAI.APIKey)
				}
				return ""
			},
		},
		{
			name:     "openrouter",
			provider: config.ProviderOpenRouter,
			check: func(cfg *config.Config) string {
				if cfg.Provider.OpenRouter.APIKey != "sk-new-key" {
					return fmt.Sprintf("OpenRouter key = %q", cfg.Provider.OpenRouter.APIKey)
				}
				return ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubWizardHooks(t)
			wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
				*(response.(*string)) = "sk-new-key"
				return nil
			}

			cfg := config.DefaultConfig()
			cfg.Provider.OpenAI.APIKey = ""
			cfg.Provider.OpenRouter.APIKey = ""
			if err := ensureAPIKey(cfg, tt.provider); err != nil {
				t.Fatalf("ensureAPIKey() error = %v", err)
			}
			if msg := tt.check(cfg); msg != "" {
				t.Fatal(msg)
			}
		})
	}
}

func TestEnsureAPIKey_PromptError(t *testing.T) {
	stubWizardHooks(t)
	wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
		return errors.New("prompt failed")
	}

	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.APIKey = ""
	err := ensureAPIKey(cfg, config.ProviderOpenAI)
	if err == nil {
		t.Fatal("ensureAPIKey() error = nil, want error")
	}
}

func TestPickModel_SuccessPaths(t *testing.T) {
	t.Run("openai", func(t *testing.T) {
		stubWizardHooks(t)
		wizardFetchOpenAIModels = func(baseURL, apiKey string) ([]llm.ModelInfo, error) {
			if baseURL != "https://example.test" || apiKey != "sk-ai" {
				t.Fatalf("unexpected fetch args: %q %q", baseURL, apiKey)
			}
			return []llm.ModelInfo{
				{ID: "gpt-4o-mini", Name: "gpt-4o-mini", Company: "OpenAI"},
				{ID: "gpt-4.1", Name: "gpt-4.1", Company: "OpenAI"},
			}, nil
		}
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			*(response.(*int)) = 1
			return nil
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenAI.BaseURL = "https://example.test"
		cfg.Provider.OpenAI.APIKey = "sk-ai"
		got, err := pickModel(cfg, config.ProviderOpenAI)
		if err != nil {
			t.Fatalf("pickModel() error = %v", err)
		}
		if got != "gpt-4.1" {
			t.Fatalf("pickModel() = %q, want %q", got, "gpt-4.1")
		}
	})

	t.Run("openrouter", func(t *testing.T) {
		stubWizardHooks(t)
		wizardFetchORModels = func(baseURL, apiKey string) ([]llm.ModelInfo, error) {
			if baseURL != "https://or.example.test" || apiKey != "sk-or" {
				t.Fatalf("unexpected fetch args: %q %q", baseURL, apiKey)
			}
			return []llm.ModelInfo{
				{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Company: "Anthropic"},
				{ID: "openai/gpt-4o-mini", Name: "GPT-4o mini", Company: "Openai"},
			}, nil
		}
		call := 0
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			ptr := response.(*int)
			*ptr = 0 // first selection in each prompt (company then model)
			call++
			return nil
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenRouter.BaseURL = "https://or.example.test"
		cfg.Provider.OpenRouter.APIKey = "sk-or"
		got, err := pickModel(cfg, config.ProviderOpenRouter)
		if err != nil {
			t.Fatalf("pickModel() error = %v", err)
		}
		if got != "anthropic/claude-3.5-sonnet" {
			t.Fatalf("pickModel() = %q, want anthropic model", got)
		}
	})
	t.Run("local", func(t *testing.T) {
		stubWizardHooks(t)
		wizardFetchLocalModels = func(baseURL string) ([]llm.ModelInfo, error) {
			if baseURL != "http://localhost:11434/api/generate" {
				t.Fatalf("unexpected baseURL: %q", baseURL)
			}
			return []llm.ModelInfo{
				{ID: "llama3", Name: "llama3"},
			}, nil
		}
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			*(response.(*int)) = 0
			return nil
		}

		cfg := config.DefaultConfig()
		got, err := pickModel(cfg, config.ProviderLocal)
		if err != nil {
			t.Fatalf("pickModel() error = %v", err)
		}
		if got != "llama3" {
			t.Fatalf("pickModel() = %q, want %q", got, "llama3")
		}
	})
}

func TestPromptLocalBaseURL(t *testing.T) {
	t.Run("accepts default", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			inp, ok := p.(*survey.Input)
			if !ok {
				t.Fatalf("prompt type = %T, want *survey.Input", p)
			}
			if inp.Default != "http://localhost:11434/api/generate" {
				t.Fatalf("default = %q", inp.Default)
			}
			*(response.(*string)) = inp.Default
			return nil
		}

		got, err := promptLocalBaseURL("http://localhost:11434/api/generate")
		if err != nil {
			t.Fatalf("promptLocalBaseURL() error = %v", err)
		}
		if got != "http://localhost:11434/api/generate" {
			t.Fatalf("promptLocalBaseURL() = %q", got)
		}
	})

	t.Run("custom url", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			*(response.(*string)) = "http://myhost:11434/api/generate"
			return nil
		}

		got, err := promptLocalBaseURL("http://localhost:11434/api/generate")
		if err != nil {
			t.Fatalf("promptLocalBaseURL() error = %v", err)
		}
		if got != "http://myhost:11434/api/generate" {
			t.Fatalf("promptLocalBaseURL() = %q, want custom url", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
			return errors.New("canceled")
		}

		_, err := promptLocalBaseURL("http://localhost:11434/api/generate")
		if err == nil {
			t.Fatal("promptLocalBaseURL() error = nil, want error")
		}
	})
}

func TestRunModelWizard(t *testing.T) {
	t.Run("success openai existing key", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			switch call {
			case 0: // provider
				if _, ok := p.(*survey.Select); !ok {
					t.Fatalf("first prompt type = %T", p)
				}
				*(response.(*int)) = 0 // OpenAI
			case 1: // model
				*(response.(*int)) = 0
			default:
				t.Fatalf("unexpected extra prompt call %d", call)
			}
			call++
			return nil
		}
		wizardFetchOpenAIModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
			return []llm.ModelInfo{{ID: "gpt-4o-mini", Name: "gpt-4o-mini", Company: "OpenAI"}}, nil
		}
		saved := false
		wizardSaveConfig = func(cfg *config.Config) error {
			saved = true
			if cfg.Provider.Default != config.ProviderOpenAI {
				t.Fatalf("saved provider = %q", cfg.Provider.Default)
			}
			if cfg.Provider.Model != "gpt-4o-mini" {
				t.Fatalf("saved model = %q", cfg.Provider.Model)
			}
			return nil
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenAI.APIKey = "sk-existing"
		if err := RunModelWizard(cfg); err != nil {
			t.Fatalf("RunModelWizard() error = %v", err)
		}
		if !saved {
			t.Fatal("RunModelWizard() did not call save")
		}
	})

	t.Run("pick provider error propagates", func(t *testing.T) {
		stubWizardHooks(t)
		wizardAskOne = func(_ survey.Prompt, _ interface{}, _ ...survey.AskOpt) error {
			return errors.New("user canceled")
		}

		cfg := config.DefaultConfig()
		err := RunModelWizard(cfg)
		if err == nil {
			t.Fatal("RunModelWizard() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "user canceled") {
			t.Fatalf("RunModelWizard() error = %q, want user canceled", err.Error())
		}
	})

	t.Run("ensure API key error propagates", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			switch call {
			case 0: // provider
				*(response.(*int)) = 0 // OpenAI
			case 1: // API key prompt
				return errors.New("key prompt failed")
			}
			call++
			return nil
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenAI.APIKey = "" // force API key prompt
		err := RunModelWizard(cfg)
		if err == nil {
			t.Fatal("RunModelWizard() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "key prompt failed") {
			t.Fatalf("RunModelWizard() error = %q, want key prompt error", err.Error())
		}
	})

	t.Run("pick model error propagates", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			if call == 0 {
				*(response.(*int)) = 0 // OpenAI
			}
			call++
			return nil
		}
		wizardFetchOpenAIModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
			return nil, errors.New("network error")
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenAI.APIKey = "sk-existing"
		err := RunModelWizard(cfg)
		if err == nil {
			t.Fatal("RunModelWizard() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "fetch models") {
			t.Fatalf("RunModelWizard() error = %q, want fetch error", err.Error())
		}
	})

	t.Run("success openrouter with new key", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			switch call {
			case 0: // provider
				*(response.(*int)) = 1 // OpenRouter
			case 1: // API key (since it's missing)
				*(response.(*string)) = "sk-new-or-key"
			case 2: // company select
				*(response.(*int)) = 0
			case 3: // model select
				*(response.(*int)) = 0
			}
			call++
			return nil
		}
		wizardFetchORModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
			return []llm.ModelInfo{
				{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Company: "Anthropic"},
			}, nil
		}
		saved := false
		wizardSaveConfig = func(cfg *config.Config) error {
			saved = true
			if cfg.Provider.Default != config.ProviderOpenRouter {
				t.Fatalf("saved provider = %q, want openrouter", cfg.Provider.Default)
			}
			if cfg.Provider.OpenRouter.APIKey != "sk-new-or-key" {
				t.Fatalf("saved API key = %q, want sk-new-or-key", cfg.Provider.OpenRouter.APIKey)
			}
			return nil
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenRouter.APIKey = "" // force API key prompt
		if err := RunModelWizard(cfg); err != nil {
			t.Fatalf("RunModelWizard() error = %v", err)
		}
		if !saved {
			t.Fatal("RunModelWizard() did not call save")
		}
	})

	t.Run("success local with custom url", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			switch call {
			case 0: // provider
				*(response.(*int)) = 2 // Local
			case 1: // base URL prompt
				if _, ok := p.(*survey.Input); !ok {
					t.Fatalf("call 1 prompt type = %T, want *survey.Input", p)
				}
				*(response.(*string)) = "http://remotehost:11434/api/generate"
			case 2: // model
				*(response.(*int)) = 0
			default:
				t.Fatalf("unexpected prompt call %d", call)
			}
			call++
			return nil
		}
		wizardFetchLocalModels = func(baseURL string) ([]llm.ModelInfo, error) {
			if baseURL != "http://remotehost:11434/api/generate" {
				t.Fatalf("fetch called with baseURL = %q, want custom url", baseURL)
			}
			return []llm.ModelInfo{{ID: "llama3", Name: "llama3"}}, nil
		}
		saved := false
		wizardSaveConfig = func(cfg *config.Config) error {
			saved = true
			if cfg.Provider.Default != config.ProviderLocal {
				t.Fatalf("saved provider = %q, want local", cfg.Provider.Default)
			}
			if cfg.Provider.Local.BaseURL != "http://remotehost:11434/api/generate" {
				t.Fatalf("saved base URL = %q", cfg.Provider.Local.BaseURL)
			}
			if cfg.Provider.Model != "llama3" {
				t.Fatalf("saved model = %q", cfg.Provider.Model)
			}
			return nil
		}

		cfg := config.DefaultConfig()
		if err := RunModelWizard(cfg); err != nil {
			t.Fatalf("RunModelWizard() error = %v", err)
		}
		if !saved {
			t.Fatal("RunModelWizard() did not call save")
		}
	})

	t.Run("local base URL prompt error propagates", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			if call == 0 {
				*(response.(*int)) = 2 // Local
				call++
				return nil
			}
			return errors.New("url prompt failed")
		}

		cfg := config.DefaultConfig()
		err := RunModelWizard(cfg)
		if err == nil {
			t.Fatal("RunModelWizard() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "url prompt failed") {
			t.Fatalf("RunModelWizard() error = %q, want url prompt error", err.Error())
		}
	})

	t.Run("save error propagates", func(t *testing.T) {
		stubWizardHooks(t)
		call := 0
		wizardAskOne = func(_ survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
			switch call {
			case 0:
				*(response.(*int)) = 0 // OpenAI
			case 1:
				*(response.(*int)) = 0 // model
			default:
				t.Fatalf("unexpected prompt call %d", call)
			}
			call++
			return nil
		}
		wizardFetchOpenAIModels = func(_ string, _ string) ([]llm.ModelInfo, error) {
			return []llm.ModelInfo{{ID: "gpt-4o", Name: "gpt-4o", Company: "OpenAI"}}, nil
		}
		wizardSaveConfig = func(_ *config.Config) error {
			return errors.New("save failed")
		}

		cfg := config.DefaultConfig()
		cfg.Provider.OpenAI.APIKey = "sk-existing"
		err := RunModelWizard(cfg)
		if err == nil {
			t.Fatal("RunModelWizard() error = nil, want save error")
		}
		if !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("RunModelWizard() error = %q, want save error", err.Error())
		}
	})
}

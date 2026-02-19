package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

func selectFromList(prompt string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("no options available")
	}
	var idx int
	err := survey.AskOne(&survey.Select{
		Message:  prompt,
		Options:  options,
		PageSize: 16,
	}, &idx, survey.WithValidator(survey.Required))
	if err != nil {
		return 0, err
	}
	return idx, nil
}

func pickProvider() (string, error) {
	idx, err := selectFromList("Select a provider:", []string{"OpenAI", "OpenRouter"})
	if err != nil {
		return "", err
	}
	if idx == 0 {
		return "openai", nil
	}
	return "openrouter", nil
}

func promptAPIKey(provider string) (string, error) {
	var key string
	err := survey.AskOne(&survey.Password{
		Message: fmt.Sprintf("API key for %s:", provider),
	}, &key, survey.WithValidator(survey.Required))
	if err != nil {
		return "", err
	}
	return key, nil
}

func ensureAPIKey(cfg *config.Config, provider string) error {
	var existing string
	switch provider {
	case "openai":
		existing = cfg.Provider.OpenAI.APIKey
	case "openrouter":
		existing = cfg.Provider.OpenRouter.APIKey
	}
	if existing != "" {
		return nil
	}
	key, err := promptAPIKey(provider)
	if err != nil {
		return err
	}
	switch provider {
	case "openai":
		cfg.Provider.OpenAI.APIKey = key
	case "openrouter":
		cfg.Provider.OpenRouter.APIKey = key
	}
	return nil
}

func pickModel(cfg *config.Config, provider string) (string, error) {
	switch provider {
	case "openrouter":
		baseURL := cfg.Provider.OpenRouter.BaseURL
		apiKey := cfg.Provider.OpenRouter.APIKey
		fmt.Println("Fetching available models from OpenRouter...")
		models, err := llm.FetchOpenRouterModels(baseURL, apiKey)
		if err != nil {
			return "", fmt.Errorf("fetch models: %w", err)
		}
		groups := llm.GroupByCompany(models)
		if len(groups) == 0 {
			return "", fmt.Errorf("no models available")
		}
		companyNames := make([]string, len(groups))
		for i, g := range groups {
			companyNames[i] = g.Company
		}
		compIdx, err := selectFromList("Select a company:", companyNames)
		if err != nil {
			return "", err
		}
		selectedGroup := groups[compIdx]
		modelNames := make([]string, len(selectedGroup.Models))
		for i, m := range selectedGroup.Models {
			modelNames[i] = m.Name
		}
		modelIdx, err := selectFromList("Select a model:", modelNames)
		if err != nil {
			return "", err
		}
		return selectedGroup.Models[modelIdx].ID, nil

	case "openai":
		baseURL := cfg.Provider.OpenAI.BaseURL
		apiKey := cfg.Provider.OpenAI.APIKey
		fmt.Println("Fetching available models from OpenAI...")
		models, err := llm.FetchOpenAIModels(baseURL, apiKey)
		if err != nil {
			return "", fmt.Errorf("fetch models: %w", err)
		}
		if len(models) == 0 {
			return "", fmt.Errorf("no GPT models available")
		}
		modelIDs := make([]string, len(models))
		for i, m := range models {
			modelIDs[i] = m.ID
		}
		idx, err := selectFromList("Select a model:", modelIDs)
		if err != nil {
			return "", err
		}
		return models[idx].ID, nil

	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

// RunModelWizard runs the full interactive setup: provider → API key (if missing) → model.
// It saves the config once at the end.
func RunModelWizard(cfg *config.Config) error {
	provider, err := pickProvider()
	if err != nil {
		return err
	}

	if err := ensureAPIKey(cfg, provider); err != nil {
		return err
	}

	model, err := pickModel(cfg, provider)
	if err != nil {
		return err
	}

	cfg.Provider.Default = provider
	cfg.Provider.Model = model

	return config.Save(cfg)
}

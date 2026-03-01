package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

var (
	wizardAskOne            = survey.AskOne
	wizardFetchOpenAIModels = llm.FetchOpenAIModels
	wizardFetchORModels     = llm.FetchOpenRouterModels
	wizardFetchLocalModels  = llm.FetchLocalModels
	wizardSaveConfig        = config.Save
)

func selectFromList(prompt string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("no options available")
	}
	var idx int
	err := wizardAskOne(&survey.Select{
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
	idx, err := selectFromList("Select a provider:", []string{"OpenAI", "OpenRouter", "Local"})
	if err != nil {
		return "", err
	}
	switch idx {
	case 0:
		return config.ProviderOpenAI, nil
	case 1:
		return config.ProviderOpenRouter, nil
	default:
		return config.ProviderLocal, nil
	}
}

func promptAPIKey(provider string) (string, error) {
	var key string
	err := wizardAskOne(&survey.Password{
		Message: fmt.Sprintf("API key for %s:", provider),
	}, &key, survey.WithValidator(survey.Required))
	if err != nil {
		return "", err
	}
	return key, nil
}

func ensureAPIKey(cfg *config.Config, provider string) error {
	if provider == config.ProviderLocal {
		// API key is optional for local providers
		return nil
	}
	var existing string
	switch provider {
	case config.ProviderOpenAI:
		existing = cfg.Provider.OpenAI.APIKey
	case config.ProviderOpenRouter:
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
	case config.ProviderOpenAI:
		cfg.Provider.OpenAI.APIKey = key
	case config.ProviderOpenRouter:
		cfg.Provider.OpenRouter.APIKey = key
	}
	return nil
}

func pickModel(cfg *config.Config, provider string) (string, error) {
	switch provider {
	case config.ProviderOpenRouter:
		baseURL := cfg.Provider.OpenRouter.BaseURL
		apiKey := cfg.Provider.OpenRouter.APIKey
		fmt.Println("Fetching available models from OpenRouter...")
		models, err := wizardFetchORModels(baseURL, apiKey)
		if err != nil {
			return "", fmt.Errorf("fetch models: %w", err)
		}
		groups := llm.GroupByCompany(models)
		if len(groups) == 0 {
			return "", errors.New("no models available")
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

	case config.ProviderOpenAI:
		baseURL := cfg.Provider.OpenAI.BaseURL
		apiKey := cfg.Provider.OpenAI.APIKey
		fmt.Println("Fetching available models from OpenAI...")
		models, err := wizardFetchOpenAIModels(baseURL, apiKey)
		if err != nil {
			return "", fmt.Errorf("fetch models: %w", err)
		}
		if len(models) == 0 {
			return "", errors.New("no GPT models available")
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

	case config.ProviderLocal:
		baseURL := cfg.Provider.Local.BaseURL
		fmt.Println("Fetching available models from local server...")
		models, err := wizardFetchLocalModels(baseURL)
		if err != nil {
			return "", fmt.Errorf("fetch models: %w", err)
		}
		if len(models) == 0 {
			return "", errors.New("no models available on local server")
		}
		modelNames := make([]string, len(models))
		for i, m := range models {
			modelNames[i] = m.Name
		}
		idx, err := selectFromList("Select a model:", modelNames)
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

	return wizardSaveConfig(cfg)
}

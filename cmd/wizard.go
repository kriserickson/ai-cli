package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

const providerLabelLocal = "Local"
const providerLabelOpenAI = "OpenAI"

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
	idx, err := selectFromList("Select a provider:", []string{providerLabelOpenAI, "OpenRouter", providerLabelLocal})
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

func promptLocalBaseURL(current string) (string, error) {
	var url string
	err := wizardAskOne(&survey.Input{
		Message: "Local server base URL:",
		Default: current,
	}, &url, survey.WithValidator(survey.Required))
	if err != nil {
		return "", err
	}
	return url, nil
}

func promptModelParameters(provider, model string, current map[string]any) (map[string]any, error) {
	fmt.Printf("Parameter help: %s\n", llm.ModelParameterHelp(provider, model))
	defaultValue := ""
	if len(current) > 0 {
		data, err := json.Marshal(current)
		if err != nil {
			return nil, fmt.Errorf("format current model parameters: %w", err)
		}
		defaultValue = string(data)
	}
	var value string
	err := wizardAskOne(&survey.Input{
		Message: "Model parameters as JSON (blank for provider defaults):",
		Default: defaultValue,
	}, &value)
	if err != nil {
		return nil, err
	}
	return config.ParseModelParameters(value)
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

// RunModelWizard configures the default (medium) model tier.
func RunModelWizard(cfg *config.Config) error {
	return RunModelWizardForLevel(cfg, config.ModelLevelDefault)
}

// RunModelWizardForLevel runs provider -> API key -> model -> parameters setup
// for one of the light, default, or high tiers. It saves once at the end.
func RunModelWizardForLevel(cfg *config.Config, level string) error {
	if !config.ValidModelLevel(level) {
		return fmt.Errorf("invalid model level %q: must be light, default, or high", level)
	}
	fmt.Printf("Configuring %s model level.\n", level)
	provider, err := pickProvider()
	if err != nil {
		return err
	}

	if err := ensureAPIKey(cfg, provider); err != nil {
		return err
	}

	if provider == config.ProviderLocal {
		url, err := promptLocalBaseURL(cfg.Provider.Local.BaseURL)
		if err != nil {
			return err
		}
		cfg.Provider.Local.BaseURL = url
	}

	model, err := pickModel(cfg, provider)
	if err != nil {
		return err
	}
	current := cfg.Provider.ModelParameters
	switch level {
	case config.ModelLevelLight:
		current = cfg.Provider.ParametersLight
	case config.ModelLevelHigh:
		current = cfg.Provider.ParametersHigh
	}
	parameters, err := promptModelParameters(provider, model, current)
	if err != nil {
		return fmt.Errorf("model parameters: %w", err)
	}

	switch level {
	case config.ModelLevelLight:
		cfg.Provider.ProviderLight = provider
		cfg.Provider.ModelLight = model
		cfg.Provider.ParametersLight = parameters
	case config.ModelLevelHigh:
		cfg.Provider.ProviderHigh = provider
		cfg.Provider.ModelHigh = model
		cfg.Provider.ParametersHigh = parameters
	default:
		cfg.Provider.Default = provider
		cfg.Provider.Model = model
		cfg.Provider.ModelParameters = parameters
	}

	return wizardSaveConfig(cfg)
}

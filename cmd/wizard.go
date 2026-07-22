package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

const (
	providerLabelLocal  = "Local"
	providerLabelOpenAI = "OpenAI"
	paramTemperature    = "temperature"
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

func pickProvider(current string) (string, error) {
	options := []string{providerLabelOpenAI, "OpenRouter", providerLabelLocal}
	defaultProvider := ""
	switch current {
	case config.ProviderOpenAI:
		defaultProvider = options[0]
	case config.ProviderOpenRouter:
		defaultProvider = options[1]
	case config.ProviderLocal:
		defaultProvider = options[2]
	}

	var idx int
	err := wizardAskOne(&survey.Select{
		Message:  "Select a provider:",
		Options:  options,
		Default:  defaultProvider,
		PageSize: 16,
	}, &idx, survey.WithValidator(survey.Required))
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

	parameters := make(map[string]any, len(current))
	for key, value := range current {
		parameters[key] = value
	}

	for {
		parameterNames := modelParameterNames(provider, model)
		options := []string{"Done", "Use provider defaults (clear all)"}
		for _, name := range parameterNames {
			label := strings.ReplaceAll(name, "_", " ")
			if value, ok := parameters[name]; ok {
				label += fmt.Sprintf(" (current: %v)", value)
			}
			options = append(options, label)
		}

		idx, err := selectFromList("Choose a model parameter:", options)
		if err != nil {
			return nil, err
		}
		switch idx {
		case 0:
			return parameters, nil
		case 1:
			return map[string]any{}, nil
		}

		name := parameterNames[idx-2]
		values := modelParameterValues(name)
		valueLabels := make([]string, len(values))
		for i, value := range values {
			valueLabels[i] = value.label
		}
		valueIdx, err := selectFromList(fmt.Sprintf("Select %s:", strings.ReplaceAll(name, "_", " ")), valueLabels)
		if err != nil {
			return nil, err
		}
		if valueIdx == 0 {
			delete(parameters, name)
		} else {
			parameters[name] = values[valueIdx].value
		}
	}
}

type modelParameterValue struct {
	label string
	value any
}

func modelParameterNames(provider, model string) []string {
	lower := strings.ToLower(model)
	if provider == config.ProviderLocal {
		return []string{paramTemperature, "top_p", "top_k", "num_predict"}
	}

	isReasoning := strings.Contains(lower, "gpt-5") || strings.HasPrefix(lower, "o1") ||
		strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4") ||
		strings.Contains(lower, "/o1") || strings.Contains(lower, "/o3") || strings.Contains(lower, "/o4")
	isClaude := strings.Contains(lower, "anthropic/") || strings.Contains(lower, "claude")
	isGemini := strings.Contains(lower, "google/") || strings.Contains(lower, "gemini")

	var names []string
	if isReasoning || isClaude || isGemini {
		names = append(names, "reasoning_effort")
	}
	if !isReasoning && (!isGemini || (!strings.Contains(lower, "gemini-3") && !strings.Contains(lower, "gemini-4"))) {
		names = append(names, paramTemperature, "top_p")
		if isClaude || isGemini {
			names = append(names, "top_k")
		}
	}
	names = append(names, "max_tokens")
	return names
}

func modelParameterValues(name string) []modelParameterValue {
	values := []modelParameterValue{{label: "Provider default (remove parameter)"}}
	switch name {
	case "reasoning_effort":
		return append(values,
			modelParameterValue{label: "minimal", value: "minimal"},
			modelParameterValue{label: "low", value: "low"},
			modelParameterValue{label: "medium", value: "medium"},
			modelParameterValue{label: config.ModelLevelHigh, value: config.ModelLevelHigh},
			modelParameterValue{label: "xhigh", value: "xhigh"},
		)
	case paramTemperature:
		for _, value := range []float64{0, 0.2, 0.5, 0.7, 1, 1.5, 2} {
			values = append(values, modelParameterValue{label: fmt.Sprint(value), value: value})
		}
	case "top_p":
		for _, value := range []float64{0.1, 0.5, 0.8, 0.9, 0.95, 1} {
			values = append(values, modelParameterValue{label: fmt.Sprint(value), value: value})
		}
	case "top_k":
		for _, value := range []int{1, 10, 20, 40, 50, 100} {
			values = append(values, modelParameterValue{label: strconv.Itoa(value), value: value})
		}
	case "max_tokens":
		for _, value := range []int{512, 1024, 2048, 4096, 8192, 16384} {
			values = append(values, modelParameterValue{label: strconv.Itoa(value), value: value})
		}
	case "num_predict":
		for _, value := range []int{256, 512, 1024, 2048, 4096, 8192} {
			values = append(values, modelParameterValue{label: strconv.Itoa(value), value: value})
		}
	}
	return values
}

func configuredProviderForLevel(cfg *config.Config, level string) string {
	provider := cfg.Provider.Default
	switch level {
	case config.ModelLevelLight:
		if cfg.Provider.ProviderLight != "" {
			provider = cfg.Provider.ProviderLight
		}
	case config.ModelLevelHigh:
		if cfg.Provider.ProviderHigh != "" {
			provider = cfg.Provider.ProviderHigh
		}
	}
	return provider
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
	provider, err := pickProvider(configuredProviderForLevel(cfg, level))
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

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
)

const (
	keyProvider                 = "provider"
	keyModel                    = "model"
	keyProviderLight            = "provider_light"
	keyModelLight               = "model_light"
	keyProviderHigh             = "provider_high"
	keyModelHigh                = "model_high"
	keyUpgradeModelOnFail       = "upgrade_model_on_fail"
	keyLLMKey                   = "llm_key"
	keyLLMURL                   = "llm_url"
	keyToolCalling              = "tool_calling"
	keyMinCertainty             = "min_certainty"
	keyHistoryIncludeLLMOutput  = "history_include_llm_output"
	keyHistoryRetryMaxAttempts  = "history_retry_max_attempts"
	keyHistoryRetryContextDepth = "history_retry_context_depth"
	keyAlwaysConfirm            = "always_confirm"
	keyModelParameters          = "model_parameters"
	keyParametersLight          = "parameters_light"
	keyParametersHigh           = "parameters_high"
	keyDebug                    = "debug"
	keyHistoryIncludeDebug      = "history_include_debug"
	keyHistoryAskOnError        = "history_ask_on_error"
	keyHistoryAutoCheckOnError  = "history_auto_check_on_error"
	keyDebugLogPayloads         = "debug_log_payloads"
	boolTrue                    = "true"
	boolFalse                   = "false"
	subCmdConfig                = "config"
	keyDefault                  = "default"
)

// configKeys is the canonical list of settable/gettable config keys used for shell completion.
var configKeys = []string{
	keyProvider,
	keyModel,
	keyProviderLight,
	keyModelLight,
	keyProviderHigh,
	keyModelHigh,
	keyModelParameters,
	keyParametersLight,
	keyParametersHigh,
	keyUpgradeModelOnFail,
	keyLLMKey,
	keyLLMURL,
	keyAlwaysConfirm,
	keyToolCalling,
	keyMinCertainty,
	keyDebug,
	keyHistoryIncludeLLMOutput,
	keyHistoryIncludeDebug,
	keyHistoryAskOnError,
	keyHistoryAutoCheckOnError,
	keyHistoryRetryMaxAttempts,
	keyHistoryRetryContextDepth,
	keyDebugLogPayloads,
}

// configKeyValues provides completion values for keys that have a fixed set of valid inputs.
var configKeyValues = map[string][]string{
	keyProvider:                {config.ProviderOpenAI, config.ProviderOpenRouter, config.ProviderLocal},
	keyProviderLight:           {config.ProviderOpenAI, config.ProviderOpenRouter, config.ProviderLocal},
	keyProviderHigh:            {config.ProviderOpenAI, config.ProviderOpenRouter, config.ProviderLocal},
	keyAlwaysConfirm:           {boolTrue, boolFalse},
	keyToolCalling:             {config.ToolCallingNever, config.ToolCallingAlwaysPrompt, config.ToolCallingDangerousPrompt, config.ToolCallingAlwaysAllow},
	keyDebug:                   {config.DebugNone, config.DebugScreen, config.DebugFile},
	keyHistoryIncludeLLMOutput: {boolTrue, boolFalse},
	keyHistoryIncludeDebug:     {boolTrue, boolFalse},
	keyHistoryAskOnError:       {boolTrue, boolFalse},
	keyHistoryAutoCheckOnError: {boolTrue, boolFalse},
	keyDebugLogPayloads:        {boolTrue, boolFalse},
	keyUpgradeModelOnFail:      {boolTrue, boolFalse},
}

func init() {
	configCmd := &cobra.Command{
		Use:   subCmdConfig,
		Short: "Manage AI CLI configuration",
	}

	configCmd.AddCommand(
		&cobra.Command{
			Use:   subCmdShow,
			Short: "Show current configuration",
			RunE: func(_ *cobra.Command, _ []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				data, err := toml.Marshal(config.RedactedCopy(cfg))
				if err != nil {
					return err
				}
				fmt.Print(string(data))
				return nil
			},
		},
		&cobra.Command{
			Use:   "get <key>",
			Short: "Get a config value",
			Args:  cobra.ExactArgs(1),
			ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
				if len(args) == 0 {
					return configKeys, cobra.ShellCompDirectiveNoFileComp
				}
				return nil, cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(_ *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				val, err := getConfigValue(cfg, args[0])
				if err != nil {
					return err
				}
				fmt.Println(val)
				return nil
			},
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set a config value",
			Args:  cobra.ExactArgs(2),
			ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
				switch len(args) {
				case 0:
					return configKeys, cobra.ShellCompDirectiveNoFileComp
				case 1:
					if vals, ok := configKeyValues[args[0]]; ok {
						return vals, cobra.ShellCompDirectiveNoFileComp
					}
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				return nil, cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(_ *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				if err := setConfigValue(cfg, args[0], args[1]); err != nil {
					return err
				}
				return config.Save(cfg)
			},
		},
	)

	rootCmd.AddCommand(configCmd)
}

// currentProviderDetail returns a pointer to the ProviderDetail for the
// currently selected provider (cfg.Provider.Default).
func currentProviderDetail(cfg *config.Config) *config.ProviderDetail {
	switch cfg.Provider.Default {
	case config.ProviderOpenAI:
		return &cfg.Provider.OpenAI
	case config.ProviderLocal:
		return &cfg.Provider.Local
	default: // openrouter (the default)
		return &cfg.Provider.OpenRouter
	}
}

func getConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case keyProvider, keyDefault:
		return cfg.Provider.Default, nil
	case keyModel:
		return cfg.Provider.Model, nil
	case keyProviderLight:
		return cfg.Provider.ProviderLight, nil
	case keyModelLight:
		return cfg.Provider.ModelLight, nil
	case keyProviderHigh:
		return cfg.Provider.ProviderHigh, nil
	case keyModelHigh:
		return cfg.Provider.ModelHigh, nil
	case keyModelParameters:
		return formatModelParameters(cfg.Provider.ModelParameters)
	case keyParametersLight:
		return formatModelParameters(cfg.Provider.ParametersLight)
	case keyParametersHigh:
		return formatModelParameters(cfg.Provider.ParametersHigh)
	case keyUpgradeModelOnFail:
		return strconv.FormatBool(cfg.Provider.UpgradeModelOnFail), nil
	case keyLLMKey:
		return maskKey(currentProviderDetail(cfg).APIKey), nil
	case keyLLMURL:
		return currentProviderDetail(cfg).BaseURL, nil
	case keyAlwaysConfirm:
		return strconv.FormatBool(cfg.Safety.AlwaysConfirm), nil
	case keyToolCalling:
		return cfg.Safety.ToolCalling, nil
	case keyMinCertainty:
		return strconv.Itoa(cfg.Safety.MinCertainty), nil
	case "allowlist":
		return strings.Join(cfg.Safety.AllowlistPrefixes, ", "), nil
	case keyDebug:
		return cfg.Debug, nil
	case keyHistoryIncludeLLMOutput:
		return strconv.FormatBool(cfg.History.IncludeLLMOutput), nil
	case keyHistoryIncludeDebug:
		return strconv.FormatBool(cfg.History.IncludeDebug), nil
	case keyHistoryAskOnError:
		return strconv.FormatBool(cfg.History.AskOnError), nil
	case keyHistoryAutoCheckOnError:
		return strconv.FormatBool(cfg.History.AutoCheckOnError), nil
	case keyHistoryRetryMaxAttempts:
		return strconv.Itoa(cfg.History.RetryMaxAttempts), nil
	case keyHistoryRetryContextDepth:
		return strconv.Itoa(cfg.History.RetryContextDepth), nil
	case keyDebugLogPayloads:
		return strconv.FormatBool(cfg.DebugLogPayloads), nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case keyProvider, keyDefault:
		if value != config.ProviderOpenAI && value != config.ProviderOpenRouter && value != config.ProviderLocal {
			return errors.New("provider must be 'openai', 'openrouter', or 'local'")
		}
		cfg.Provider.Default = value
	case keyModel:
		cfg.Provider.Model = value
	case keyProviderLight:
		if err := validateProvider(value); err != nil {
			return err
		}
		cfg.Provider.ProviderLight = value
	case keyModelLight:
		cfg.Provider.ModelLight = value
	case keyProviderHigh:
		if err := validateProvider(value); err != nil {
			return err
		}
		cfg.Provider.ProviderHigh = value
	case keyModelHigh:
		cfg.Provider.ModelHigh = value
	case keyModelParameters, keyParametersLight, keyParametersHigh:
		parameters, err := config.ParseModelParameters(value)
		if err != nil {
			return fmt.Errorf("%s %w", key, err)
		}
		switch key {
		case keyModelParameters:
			cfg.Provider.ModelParameters = parameters
		case keyParametersLight:
			cfg.Provider.ParametersLight = parameters
		case keyParametersHigh:
			cfg.Provider.ParametersHigh = parameters
		}
	case keyUpgradeModelOnFail:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s %w", keyUpgradeModelOnFail, err)
		}
		cfg.Provider.UpgradeModelOnFail = b
	case keyLLMKey:
		currentProviderDetail(cfg).APIKey = value
	case keyLLMURL:
		currentProviderDetail(cfg).BaseURL = value
	case keyAlwaysConfirm:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s %w", keyAlwaysConfirm, err)
		}
		cfg.Safety.AlwaysConfirm = b
	case keyToolCalling:
		if !config.ValidToolCallingMode(value) {
			return errors.New("tool_calling must be 'never', 'always_prompt', 'dangerous_prompt', or 'always_allow'")
		}
		cfg.Safety.ToolCalling = value
	case keyMinCertainty:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be a number: %w", keyMinCertainty, err)
		}
		if n < 0 || n > 100 {
			return fmt.Errorf("%s must be between 0 and 100", keyMinCertainty)
		}
		cfg.Safety.MinCertainty = n
	case keyDebug:
		if value != config.DebugNone && value != config.DebugScreen && value != config.DebugFile {
			return fmt.Errorf("debug must be '%s', '%s', or '%s'", config.DebugNone, config.DebugScreen, config.DebugFile)
		}
		cfg.Debug = value
	case keyHistoryIncludeLLMOutput:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s %w", keyHistoryIncludeLLMOutput, err)
		}
		cfg.History.IncludeLLMOutput = b
	case keyHistoryIncludeDebug:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("history_include_debug %w", err)
		}
		cfg.History.IncludeDebug = b
	case keyHistoryAskOnError:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("history_ask_on_error %w", err)
		}
		cfg.History.AskOnError = b
	case keyHistoryAutoCheckOnError:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("history_auto_check_on_error %w", err)
		}
		cfg.History.AutoCheckOnError = b
	case keyHistoryRetryMaxAttempts:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be a number: %w", keyHistoryRetryMaxAttempts, err)
		}
		if n < 0 {
			return fmt.Errorf("%s must be 0 or greater", keyHistoryRetryMaxAttempts)
		}
		cfg.History.RetryMaxAttempts = n
	case keyHistoryRetryContextDepth:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be a number: %w", keyHistoryRetryContextDepth, err)
		}
		if n <= 0 {
			return fmt.Errorf("%s must be greater than 0", keyHistoryRetryContextDepth)
		}
		cfg.History.RetryContextDepth = n
	case keyDebugLogPayloads:
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("debug_log_payloads %w", err)
		}
		cfg.DebugLogPayloads = b
	default:
		return fmt.Errorf("unknown config key: %s\nValid keys: %s", key, strings.Join(configKeys, ", "))
	}
	return nil
}

func validateProvider(value string) error {
	if value != config.ProviderOpenAI && value != config.ProviderOpenRouter && value != config.ProviderLocal {
		return errors.New("provider must be 'openai', 'openrouter', or 'local'")
	}
	return nil
}

func formatModelParameters(parameters map[string]any) (string, error) {
	if len(parameters) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(parameters)
	if err != nil {
		return "", fmt.Errorf("format model parameters: %w", err)
	}
	return string(data), nil
}

func maskKey(key string) string {
	return config.RedactSecret(key)
}

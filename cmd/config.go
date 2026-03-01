package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
)

// configKeys is the canonical list of settable/gettable config keys used for shell completion.
var configKeys = []string{
	"provider",
	"model",
	"openai_key",
	"openrouter_key",
	"openai_url",
	"openrouter_url",
	"llm_key",
	"llm_url",
	"always_confirm",
	"min_certainty",
	"debug",
}

// configKeyValues provides completion values for keys that have a fixed set of valid inputs.
var configKeyValues = map[string][]string{
	"provider":       {config.ProviderOpenAI, config.ProviderOpenRouter, config.ProviderLocal},
	"always_confirm": {"true", "false"},
	"debug":          {config.DebugNone, config.DebugScreen, config.DebugFile},
}

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage AI CLI configuration",
	}

	configCmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Show current configuration",
			RunE: func(_ *cobra.Command, _ []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				data, err := toml.Marshal(cfg)
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
	case "provider", "default":
		return cfg.Provider.Default, nil
	case "model":
		return cfg.Provider.Model, nil
	case "openai_key":
		return maskKey(cfg.Provider.OpenAI.APIKey), nil
	case "openrouter_key":
		return maskKey(cfg.Provider.OpenRouter.APIKey), nil
	case "openai_url":
		return cfg.Provider.OpenAI.BaseURL, nil
	case "openrouter_url":
		return cfg.Provider.OpenRouter.BaseURL, nil
	case "llm_key":
		return maskKey(currentProviderDetail(cfg).APIKey), nil
	case "llm_url":
		return currentProviderDetail(cfg).BaseURL, nil
	case "always_confirm":
		return strconv.FormatBool(cfg.Safety.AlwaysConfirm), nil
	case "min_certainty":
		return strconv.Itoa(cfg.Safety.MinCertainty), nil
	case "allowlist":
		return strings.Join(cfg.Safety.AllowlistPrefixes, ", "), nil
	case "debug":
		return cfg.Debug, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "provider", "default":
		if value != config.ProviderOpenAI && value != config.ProviderOpenRouter && value != config.ProviderLocal {
			return errors.New("provider must be 'openai', 'openrouter', or 'local'")
		}
		cfg.Provider.Default = value
	case "model":
		cfg.Provider.Model = value
	case "openai_key":
		cfg.Provider.OpenAI.APIKey = value
	case "openrouter_key":
		cfg.Provider.OpenRouter.APIKey = value
	case "openai_url":
		cfg.Provider.OpenAI.BaseURL = value
	case "openrouter_url":
		cfg.Provider.OpenRouter.BaseURL = value
	case "llm_key":
		currentProviderDetail(cfg).APIKey = value
	case "llm_url":
		currentProviderDetail(cfg).BaseURL = value
	case "always_confirm":
		b, err := config.ParseBool(value)
		if err != nil {
			return fmt.Errorf("always_confirm %w", err)
		}
		cfg.Safety.AlwaysConfirm = b
	case "min_certainty":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("min_certainty must be a number: %w", err)
		}
		if n < 0 || n > 100 {
			return errors.New("min_certainty must be between 0 and 100")
		}
		cfg.Safety.MinCertainty = n
	case "debug":
		if value != config.DebugNone && value != config.DebugScreen && value != config.DebugFile {
			return fmt.Errorf("debug must be '%s', '%s', or '%s'", config.DebugNone, config.DebugScreen, config.DebugFile)
		}
		cfg.Debug = value
	default:
		return fmt.Errorf("unknown config key: %s\nValid keys: provider, model, llm_key, llm_url, openai_key, openrouter_key, openai_url, openrouter_url, always_confirm, min_certainty, debug", key)
	}
	return nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

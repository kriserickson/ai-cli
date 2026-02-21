package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kriserickson/ai-cli/internal/config"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// configKeys is the canonical list of settable/gettable config keys used for shell completion.
var configKeys = []string{
	"provider",
	"model",
	"openai_key",
	"openrouter_key",
	"openai_url",
	"openrouter_url",
	"always_confirm",
	"min_certainty",
	"debug",
}

// configKeyValues provides completion values for keys that have a fixed set of valid inputs.
var configKeyValues = map[string][]string{
	"provider":       {"openai", "openrouter"},
	"always_confirm": {"true", "false"},
	"debug":          {"none", "screen", "file"},
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
			RunE: func(cmd *cobra.Command, args []string) error {
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
			ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				if len(args) == 0 {
					return configKeys, cobra.ShellCompDirectiveNoFileComp
				}
				return nil, cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(cmd *cobra.Command, args []string) error {
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
			ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
			RunE: func(cmd *cobra.Command, args []string) error {
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
	case "always_confirm":
		return fmt.Sprintf("%v", cfg.Safety.AlwaysConfirm), nil
	case "min_certainty":
		return fmt.Sprintf("%d", cfg.Safety.MinCertainty), nil
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
		if value != "openai" && value != "openrouter" {
			return fmt.Errorf("provider must be 'openai' or 'openrouter'")
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
	case "always_confirm":
		lower := strings.ToLower(strings.TrimSpace(value))
		switch lower {
		case "true":
			cfg.Safety.AlwaysConfirm = true
		case "false":
			cfg.Safety.AlwaysConfirm = false
		default:
			return fmt.Errorf("always_confirm must be 'true' or 'false'")
		}
	case "min_certainty":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("min_certainty must be a number: %w", err)
		}
		if n < 0 || n > 100 {
			return fmt.Errorf("min_certainty must be between 0 and 100")
		}
		cfg.Safety.MinCertainty = n
	case "debug":
		if value != "none" && value != "screen" && value != "file" {
			return fmt.Errorf("debug must be 'none', 'screen', or 'file'")
		}
		cfg.Debug = value
	default:
		return fmt.Errorf("unknown config key: %s\nValid keys: provider, model, openai_key, openrouter_key, openai_url, openrouter_url, always_confirm, min_certainty, debug", key)
	}
	return nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

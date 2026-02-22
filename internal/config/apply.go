package config

import (
	"errors"
	"fmt"
	"strconv"
)

// ApplyAction applies an LLM-initiated config change and saves the result.
// The action/key/value correspond to the structured response from the LLM
// (e.g. action="set_model", key="model", value="gpt-4o").
//
// This is the single source of truth for config mutations triggered by the LLM,
// used by both the single-shot CLI path and the interactive REPL.
func ApplyAction(cfg *Config, action, key, value string) error {
	switch action {
	case "set_model":
		cfg.Provider.Model = value
	case "set_provider":
		if value != ProviderOpenAI && value != ProviderOpenRouter {
			return fmt.Errorf("invalid provider: %s", value)
		}
		cfg.Provider.Default = value
	case "set_key":
		switch key {
		case "openai_key":
			cfg.Provider.OpenAI.APIKey = value
		case "openrouter_key":
			cfg.Provider.OpenRouter.APIKey = value
		default:
			return fmt.Errorf("unknown key: %s", key)
		}
	case "set_safety":
		switch key {
		case "always_confirm":
			b, err := ParseBool(value)
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
		default:
			return fmt.Errorf("unknown safety key: %s", key)
		}
	default:
		return fmt.Errorf("unknown config action: %s", action)
	}

	if err := Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

package config

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	keyLLMKey           = "llm_key"
	keyIncludeLLMOutput = "include_llm_output"
	keyIncludeDebug     = "include_debug"
	keyAskOnError       = "ask_on_error"
	keyAutoCheckOnError = "auto_check_on_error"
	keyRetryMaxAttempts = "retry_max_attempts"
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
		if value != ProviderOpenAI && value != ProviderOpenRouter && value != ProviderLocal {
			return fmt.Errorf("invalid provider: %s", value)
		}
		cfg.Provider.Default = value
	case "set_key":
		switch key {
		case keyLLMKey:
			switch cfg.Provider.Default {
			case ProviderOpenAI:
				cfg.Provider.OpenAI.APIKey = value
			case ProviderLocal:
				cfg.Provider.Local.APIKey = value
			default:
				cfg.Provider.OpenRouter.APIKey = value
			}
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
		case "tool_calling":
			if !ValidToolCallingMode(value) {
				return errors.New("tool_calling must be 'never', 'always_prompt', 'dangerous_prompt', or 'always_allow'")
			}
			cfg.Safety.ToolCalling = value
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
	case "set_history":
		switch key {
		case keyIncludeLLMOutput:
			b, err := ParseBool(value)
			if err != nil {
				return fmt.Errorf("history.include_llm_output %w", err)
			}
			cfg.History.IncludeLLMOutput = b
		case keyIncludeDebug:
			b, err := ParseBool(value)
			if err != nil {
				return fmt.Errorf("history.include_debug %w", err)
			}
			cfg.History.IncludeDebug = b
		case keyAskOnError:
			b, err := ParseBool(value)
			if err != nil {
				return fmt.Errorf("history.ask_on_error %w", err)
			}
			cfg.History.AskOnError = b
		case keyAutoCheckOnError:
			b, err := ParseBool(value)
			if err != nil {
				return fmt.Errorf("history.auto_check_on_error %w", err)
			}
			cfg.History.AutoCheckOnError = b
		case keyRetryMaxAttempts:
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("history.retry_max_attempts must be a number: %w", err)
			}
			if n < 0 {
				return errors.New("history.retry_max_attempts must be 0 or greater")
			}
			cfg.History.RetryMaxAttempts = n
		case "retry_context_depth":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("history.retry_context_depth must be a number: %w", err)
			}
			if n <= 0 {
				return errors.New("history.retry_context_depth must be greater than 0")
			}
			cfg.History.RetryContextDepth = n
		default:
			return fmt.Errorf("unknown history key: %s", key)
		}
	default:
		return fmt.Errorf("unknown config action: %s", action)
	}

	if err := Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

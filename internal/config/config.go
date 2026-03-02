package config

import (
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Provider name constants.
const (
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderLocal      = "local"

	DebugNone   = "none"
	DebugScreen = "screen"
	DebugFile   = "file"
)

type Config struct {
	Provider ProviderConfig `toml:"provider"`
	Safety   SafetyConfig   `toml:"safety"`
	History  HistoryConfig  `toml:"history"`
	Debug    string         `toml:"debug"` // "none", "screen", or "file"
}

type ProviderConfig struct {
	Default    string         `toml:"default"`
	Model      string         `toml:"model"`
	OpenAI     ProviderDetail `toml:"openai"`
	OpenRouter ProviderDetail `toml:"openrouter"`
	Local      ProviderDetail `toml:"local"`
}

type ProviderDetail struct {
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
}

type SafetyConfig struct {
	AlwaysConfirm     bool     `toml:"always_confirm"`
	MinCertainty      int      `toml:"min_certainty"`
	AllowlistPrefixes []string `toml:"allowlist_prefixes"`
	WhitelistPrefixes []string `toml:"whitelist_prefixes,omitempty"` // Deprecated: use allowlist_prefixes
}

type HistoryConfig struct {
	IncludeLLMOutput  bool `toml:"include_llm_output"`
	IncludeDebug      bool `toml:"include_debug"`
	AskOnError        bool `toml:"ask_on_error"`
	AutoCheckOnError  bool `toml:"auto_check_on_error"`
	RetryMaxAttempts  int  `toml:"retry_max_attempts"`
	RetryContextDepth int  `toml:"retry_context_depth"`
}

func DefaultConfig() *Config {
	return &Config{
		Provider: ProviderConfig{
			Default: ProviderOpenRouter,
			Model:   "anthropic/claude-3.5-sonnet",
			OpenAI: ProviderDetail{
				BaseURL: "https://api.openai.com/v1",
			},
			OpenRouter: ProviderDetail{
				BaseURL: "https://openrouter.ai/api/v1",
			},
			Local: ProviderDetail{
				BaseURL: "http://localhost:11434/api/generate",
			},
		},
		Safety: SafetyConfig{
			AlwaysConfirm:     false,
			MinCertainty:      80,
			AllowlistPrefixes: []string{"git", "ls", "cat", "echo", "pwd", "head", "tail", "wc", "grep", "find", "which", "man"},
		},
		History: HistoryConfig{
			IncludeLLMOutput:  true,
			IncludeDebug:      false,
			AskOnError:        true,
			AutoCheckOnError:  false,
			RetryMaxAttempts:  1,
			RetryContextDepth: 3,
		},
		Debug: DebugNone,
	}
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ai-cli"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := Save(cfg); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Migrate whitelist to allowlist if needed
	if len(cfg.Safety.WhitelistPrefixes) > 0 {
		cfg.Safety.AllowlistPrefixes = cfg.Safety.WhitelistPrefixes
		cfg.Safety.WhitelistPrefixes = nil
		// Save the migrated config
		_ = Save(cfg)
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}

	return nil
}

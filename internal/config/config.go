package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Provider name constants.
const (
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderLocal      = "local"

	ModelLevelLight   = "light"
	ModelLevelDefault = "default"
	ModelLevelHigh    = "high"

	DebugNone   = "none"
	DebugScreen = "screen"
	DebugFile   = "file"

	// ToolCalling modes control whether the AI can use read-only tools.
	ToolCallingNever           = "never"            // Tools disabled entirely
	ToolCallingAlwaysPrompt    = "always_prompt"    // Prompt the user before every tool call
	ToolCallingDangerousPrompt = "dangerous_prompt" // Only prompt when a tool hits a safety rule
	ToolCallingAlwaysAllow     = "always_allow"     // Execute all tools without prompting

	allowlistGit = "git"
)

type Config struct {
	Provider         ProviderConfig `toml:"provider"`
	Safety           SafetyConfig   `toml:"safety"`
	History          HistoryConfig  `toml:"history"`
	Debug            string         `toml:"debug"`              // "none", "screen", or "file"
	DebugLogPayloads bool           `toml:"debug_log_payloads"` // Only used for debug=file
	// ModelLevel is a runtime-only selection. It is set by --light/--high or
	// interactive mode and is intentionally not persisted.
	ModelLevel string `toml:"-"`
}

type ProviderConfig struct {
	Default            string         `toml:"default"`
	Model              string         `toml:"model"`
	ProviderLight      string         `toml:"provider_light,omitempty"`
	ModelLight         string         `toml:"model_light,omitempty"`
	ProviderHigh       string         `toml:"provider_high,omitempty"`
	ModelHigh          string         `toml:"model_high,omitempty"`
	ModelParameters    map[string]any `toml:"model_parameters,omitempty"`
	ParametersLight    map[string]any `toml:"parameters_light,omitempty"`
	ParametersHigh     map[string]any `toml:"parameters_high,omitempty"`
	UpgradeModelOnFail bool           `toml:"upgrade_model_on_fail"`
	OpenAI             ProviderDetail `toml:"openai"`
	OpenRouter         ProviderDetail `toml:"openrouter"`
	Local              ProviderDetail `toml:"local"`
}

// ModelSelection is the provider, model, and request parameters for a model tier.
type ModelSelection struct {
	Provider   string
	Model      string
	Parameters map[string]any
}

// ValidModelLevel reports whether level is light, default, or high.
func ValidModelLevel(level string) bool {
	switch level {
	case ModelLevelLight, ModelLevelDefault, ModelLevelHigh:
		return true
	}
	return false
}

// Selection returns the configured provider, model, and parameters for level.
// A blank light/high provider inherits the default provider.
func (p ProviderConfig) Selection(level string) (ModelSelection, error) {
	selection := ModelSelection{Provider: p.Default}
	switch level {
	case ModelLevelDefault, "":
		selection.Model = p.Model
		selection.Parameters = p.ModelParameters
	case ModelLevelLight:
		if p.ProviderLight != "" {
			selection.Provider = p.ProviderLight
		}
		selection.Model = p.ModelLight
		selection.Parameters = p.ParametersLight
	case ModelLevelHigh:
		if p.ProviderHigh != "" {
			selection.Provider = p.ProviderHigh
		}
		selection.Model = p.ModelHigh
		selection.Parameters = p.ParametersHigh
	default:
		return ModelSelection{}, fmt.Errorf("invalid model level %q: must be light, default, or high", level)
	}
	if selection.Model == "" {
		return ModelSelection{}, fmt.Errorf("no %s model configured; run: ai set-model %s", level, level)
	}
	return selection, nil
}

// ActiveSelection returns the runtime-selected model tier.
func (c *Config) ActiveSelection() (ModelSelection, error) {
	level := c.ModelLevel
	if level == "" {
		level = ModelLevelDefault
	}
	return c.Provider.Selection(level)
}

type ProviderDetail struct {
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
}

type SafetyConfig struct {
	AlwaysConfirm     bool     `toml:"always_confirm"`
	ToolCalling       string   `toml:"tool_calling"`
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

// ValidToolCallingMode returns true if the given mode is a valid tool_calling value.
func ValidToolCallingMode(mode string) bool {
	switch mode {
	case ToolCallingNever, ToolCallingAlwaysPrompt, ToolCallingDangerousPrompt, ToolCallingAlwaysAllow:
		return true
	}
	return false
}

func DefaultConfig() *Config {
	return &Config{
		Provider: ProviderConfig{
			Default:            ProviderOpenRouter,
			Model:              "anthropic/claude-3.5-sonnet",
			UpgradeModelOnFail: false,
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
			ToolCalling:       ToolCallingNever,
			MinCertainty:      80,
			AllowlistPrefixes: []string{allowlistGit, "ls", "cat", "echo", "pwd", "head", "tail", "wc", "grep", "find", "which", "man"},
		},
		History: HistoryConfig{
			IncludeLLMOutput:  true,
			IncludeDebug:      false,
			AskOnError:        true,
			AutoCheckOnError:  false,
			RetryMaxAttempts:  1,
			RetryContextDepth: 3,
		},
		Debug:            DebugNone,
		DebugLogPayloads: false,
		ModelLevel:       ModelLevelDefault,
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

package config

// RedactSecret masks a secret for display purposes.
func RedactSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

// RedactedCopy returns a copy of cfg with provider API keys masked for display.
func RedactedCopy(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	copyCfg := *cfg
	copyCfg.Provider = cfg.Provider
	copyCfg.Safety = cfg.Safety

	copyCfg.Provider.OpenAI = cfg.Provider.OpenAI
	copyCfg.Provider.OpenRouter = cfg.Provider.OpenRouter
	copyCfg.Provider.Local = cfg.Provider.Local

	copyCfg.Provider.OpenAI.APIKey = RedactSecret(copyCfg.Provider.OpenAI.APIKey)
	copyCfg.Provider.OpenRouter.APIKey = RedactSecret(copyCfg.Provider.OpenRouter.APIKey)
	copyCfg.Provider.Local.APIKey = RedactSecret(copyCfg.Provider.Local.APIKey)

	return &copyCfg
}

// DisplayValue masks sensitive config values before printing them to the user.
func DisplayValue(action, key, value string) string {
	if action == "set_key" || key == "llm_key" {
		return RedactSecret(value)
	}
	return value
}

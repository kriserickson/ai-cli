package llm

import (
	"fmt"
	"strings"

	"github.com/kriserickson/ai-cli/internal/config"
)

// ModelParameterHelp returns concise setup guidance for the major model
// families supported directly by OpenAI or through OpenRouter.
func ModelParameterHelp(provider, model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "gpt-5") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4") ||
		strings.Contains(lower, "/o1") || strings.Contains(lower, "/o3") || strings.Contains(lower, "/o4"):
		return "OpenAI reasoning model: use reasoning_effort (for example low, medium, or high). " +
			"GPT-5-family support varies by version; avoid temperature unless that model explicitly supports it."
	case strings.Contains(lower, "gpt-4"):
		return "OpenAI GPT-4-family model: temperature (0-2) or top_p (0-1) controls sampling; change one, not both."
	case strings.Contains(lower, "anthropic/") || strings.Contains(lower, "claude"):
		if provider == "openrouter" {
			return "Anthropic Claude through OpenRouter: temperature, top_p, and top_k are available; " +
				"current reasoning-capable Claude models can use reasoning_effort (low, medium, high, and model-dependent higher levels)."
		}
		return "Anthropic Claude: temperature, top_p, and top_k control sampling; current Claude models also support effort/thinking controls."
	case strings.Contains(lower, "google/") || strings.Contains(lower, "gemini"):
		if strings.Contains(lower, "gemini-3") || strings.Contains(lower, "gemini-4") {
			return "Google Gemini 3+: keep temperature/top_p/top_k at model defaults; use reasoning_effort through OpenRouter to control thinking."
		}
		return "Google Gemini: temperature, top_p, and top_k are supported; Gemini 2.5 reasoning can be controlled with reasoning_effort through OpenRouter."
	case provider == config.ProviderLocal:
		return "Ollama-compatible model: parameters are sent in options; common keys include temperature, top_p, top_k, and num_predict."
	default:
		return fmt.Sprintf("%s model: common OpenAI-compatible parameters include temperature, top_p, max_tokens, and reasoning_effort; verify model support.", provider)
	}
}

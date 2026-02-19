package executor

import (
	"strings"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

// ShouldConfirm determines whether a command needs user confirmation before execution.
func ShouldConfirm(cmd llm.Command, cfg *config.Config) bool {
	if cfg.Safety.AlwaysConfirm {
		return true
	}

	whitelisted := isWhitelisted(cmd.Command, cfg.Safety.WhitelistPrefixes)
	highCertainty := cmd.Certainty >= cfg.Safety.MinCertainty

	switch {
	case cmd.Risk == "safe" && highCertainty:
		return false
	case cmd.Risk == "safe" && !highCertainty:
		return true
	case cmd.Risk == "risky" && whitelisted && highCertainty:
		return false
	case cmd.Risk == "risky" && whitelisted && !highCertainty:
		return true
	case cmd.Risk == "risky" && !whitelisted:
		return true
	default:
		return true
	}
}

func isWhitelisted(command string, prefixes []string) bool {
	cmd := strings.TrimSpace(command)
	for _, prefix := range prefixes {
		if strings.HasPrefix(cmd, prefix+" ") || cmd == prefix {
			return true
		}
	}
	return false
}

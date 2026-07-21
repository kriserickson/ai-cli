package executor

import (
	"strings"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

// ShouldConfirm determines whether a command needs user confirmation before execution.
//
// Decision matrix:
//
//	always_confirm=true        → confirm
//	risky                      → confirm
//	safe + allowlisted         → auto-execute
//	safe + certainty ≥ min     → auto-execute
//	safe + certainty < min     → confirm
func ShouldConfirm(cmd llm.Command, cfg *config.Config) bool {
	if cfg.Safety.AlwaysConfirm {
		return true
	}

	if cmd.Risk == riskRisky {
		return true
	}

	// Safe commands that match the allowlist always auto-execute.
	if isAllowlisted(cmd.Command, cfg.Safety.AllowlistPrefixes) {
		return false
	}

	return cmd.Certainty < cfg.Safety.MinCertainty
}

func isAllowlisted(command string, prefixes []string) bool {
	cmd := strings.TrimSpace(command)
	for _, prefix := range prefixes {
		if strings.HasPrefix(cmd, prefix+" ") || cmd == prefix {
			return true
		}
	}
	return false
}

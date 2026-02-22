package executor

import (
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
)

func defaultSafetyCfg() *config.Config {
	return &config.Config{
		Safety: config.SafetyConfig{
			AlwaysConfirm:     false,
			MinCertainty:      80,
			AllowlistPrefixes: []string{"git", "ls", "cat", "echo", "grep", "find"},
		},
	}
}

func TestShouldConfirm(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		risk        string
		certainty   int
		cfgOverride func(*config.Config)
		wantConfirm bool
	}{
		// safe, NOT allowlisted — certainty decides
		{
			name:        "safe + high certainty → auto-execute",
			command:     "date",
			risk:        "safe",
			certainty:   95,
			wantConfirm: false,
		},
		{
			name:        "safe + low certainty → confirm",
			command:     "date",
			risk:        "safe",
			certainty:   50,
			wantConfirm: true,
		},
		{
			name:        "safe + exact threshold → auto-execute (>= not >)",
			command:     "date",
			risk:        "safe",
			certainty:   80,
			wantConfirm: false,
		},

		// safe + allowlisted — bypasses certainty check
		{
			name:        "safe + allowlisted + low certainty → auto-execute",
			command:     "ls -la",
			risk:        "safe",
			certainty:   50,
			wantConfirm: false,
		},

		// risky — always confirm regardless of allowlist or certainty
		{
			name:        "risky + allowlisted + high certainty → confirm",
			command:     "git push origin main",
			risk:        "risky",
			certainty:   90,
			wantConfirm: true,
		},
		{
			name:        "risky + not allowlisted → confirm",
			command:     "rm -rf /tmp/stuff",
			risk:        "risky",
			certainty:   99,
			wantConfirm: true,
		},

		// always_confirm overrides everything
		{
			name:      "always_confirm overrides safe + allowlisted + 100%",
			command:   "ls",
			risk:      "safe",
			certainty: 100,
			cfgOverride: func(c *config.Config) {
				c.Safety.AlwaysConfirm = true
			},
			wantConfirm: true,
		},

		// empty allowlist — falls back to certainty check
		{
			name:      "empty allowlist + safe + low certainty → confirm",
			command:   "ls -la",
			risk:      "safe",
			certainty: 50,
			cfgOverride: func(c *config.Config) {
				c.Safety.AllowlistPrefixes = []string{}
			},
			wantConfirm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultSafetyCfg()
			if tt.cfgOverride != nil {
				tt.cfgOverride(cfg)
			}
			cmd := llm.Command{Command: tt.command, Risk: tt.risk, Certainty: tt.certainty}
			if got := ShouldConfirm(cmd, cfg); got != tt.wantConfirm {
				t.Errorf("ShouldConfirm() = %v, want %v", got, tt.wantConfirm)
			}
		})
	}
}

func TestIsAllowlisted(t *testing.T) {
	prefixes := []string{"git", "ls", "cat", "echo"}

	tests := []struct {
		command string
		want    bool
	}{
		{"ls -la", true},
		{"ls", true},
		{"git status", true},
		{"git", true},
		{"cat /etc/hosts", true},
		{"echo hello", true},
		{"rm -rf /", false},
		{"lsof -i :8080", false}, // "ls" prefix shouldn't match "lsof"
		{"gitsomething", false},  // "git" prefix shouldn't match "gitsomething"
		{"  ls -la", true},       // leading whitespace
		{"", false},
	}

	for _, tt := range tests {
		got := isAllowlisted(tt.command, prefixes)
		if got != tt.want {
			t.Errorf("isAllowlisted(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

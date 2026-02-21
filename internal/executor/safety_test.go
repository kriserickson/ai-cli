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

func TestShouldConfirm_SafeHighCertainty(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "date", Risk: "safe", Certainty: 95}
	if ShouldConfirm(cmd, cfg) {
		t.Error("safe command with high certainty should auto-execute")
	}
}

func TestShouldConfirm_SafeLowCertainty(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "date", Risk: "safe", Certainty: 50}
	if !ShouldConfirm(cmd, cfg) {
		t.Error("safe command with low certainty should require confirmation")
	}
}

func TestShouldConfirm_SafeExactThreshold(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "date", Risk: "safe", Certainty: 80}
	if ShouldConfirm(cmd, cfg) {
		t.Error("safe command at exactly min_certainty should auto-execute")
	}
}

func TestShouldConfirm_RiskyAllowlistedHighCertainty(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "git push origin main", Risk: "risky", Certainty: 90}
	if !ShouldConfirm(cmd, cfg) {
		t.Error("risky Allowlisted command with high certainty should require confirmation")
	}
}

func TestShouldConfirm_RiskyAllowlistedLowCertainty(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "git push --force", Risk: "risky", Certainty: 60}
	if !ShouldConfirm(cmd, cfg) {
		t.Error("risky Allowlisted command with low certainty should require confirmation")
	}
}

func TestShouldConfirm_RiskyAllowlistedExactThreshold(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "git checkout .", Risk: "risky", Certainty: 80}
	if !ShouldConfirm(cmd, cfg) {
		t.Error("risky command at min_certainty should require confirmation")
	}
}

func TestShouldConfirm_RiskyNotAllowlisted(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmd := llm.Command{Command: "rm -rf /tmp/stuff", Risk: "risky", Certainty: 99}
	if !ShouldConfirm(cmd, cfg) {
		t.Error("risky non-Allowlisted command should always require confirmation")
	}
}

func TestShouldConfirm_AlwaysConfirm(t *testing.T) {
	cfg := defaultSafetyCfg()
	cfg.Safety.AlwaysConfirm = true
	cmd := llm.Command{Command: "ls", Risk: "safe", Certainty: 100}
	if !ShouldConfirm(cmd, cfg) {
		t.Error("always_confirm should require confirmation for any command")
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
		{"lsof -i :8080", false},  // "ls" prefix shouldn't match "lsof"
		{"gitsomething", false},   // "git" prefix shouldn't match "gitsomething"
		{"  ls -la", true},        // leading whitespace
		{"", false},
	}

	for _, tt := range tests {
		got := isAllowlisted(tt.command, prefixes)
		if got != tt.want {
			t.Errorf("isAllowlisted(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

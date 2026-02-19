package llm

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_ContainsEnvironment(t *testing.T) {
	prompt := BuildSystemPrompt("darwin/arm64", "/bin/zsh", "zsh 5.9", "/home/user")

	checks := []string{
		"darwin/arm64",
		"/bin/zsh",
		"zsh 5.9",
		"/home/user",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestBuildSystemPrompt_ContainsJSONInstructions(t *testing.T) {
	prompt := BuildSystemPrompt("linux/amd64", "/bin/bash", "5.1", "/tmp")

	checks := []string{
		`"type": "commands"`,
		`"risk"`,
		`"certainty"`,
		`"type": "config"`,
		"valid JSON",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

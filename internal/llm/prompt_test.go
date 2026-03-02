package llm

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_ContainsEnvironment(t *testing.T) {
	prompt := BuildSystemPrompt("darwin/arm64", "/bin/zsh", "zsh 5.9", "/home/user", false)

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
	prompt := BuildSystemPrompt("linux/amd64", "/bin/bash", "5.1", "/tmp", false)

	checks := []string{
		`"type": "commands"`,
		`"risk"`,
		`"certainty"`,
		`"explanation"`,
		`"type": "config"`,
		"valid JSON",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestBuildSystemPrompt_DarwinPlatformHints(t *testing.T) {
	prompt := BuildSystemPrompt("darwin/arm64", "/bin/zsh", "zsh 5.9", "/Users/test", false)

	mustContain := []string{
		"BSD userland",
		`ps aux -r`,
		`Do NOT use GNU "--sort" flag`,
		`sed -i ''`,
		`grep -E`,
		`stat -f`,
		`du -d`,
	}
	for _, want := range mustContain {
		if !strings.Contains(prompt, want) {
			t.Errorf("darwin prompt missing %q", want)
		}
	}

	mustNotContain := []string{
		"uses GNU coreutils",
		"PowerShell cmdlets",
	}
	for _, bad := range mustNotContain {
		if strings.Contains(prompt, bad) {
			t.Errorf("darwin prompt should not contain %q", bad)
		}
	}
}

func TestBuildSystemPrompt_LinuxPlatformHints(t *testing.T) {
	prompt := BuildSystemPrompt("linux/amd64", "/bin/bash", "5.1", "/home/test", false)

	mustContain := []string{
		"uses GNU coreutils",
		`ps aux --sort=-%cpu`,
		`sed -i`,
		`date -d`,
		`grep -P`,
	}
	for _, want := range mustContain {
		if !strings.Contains(prompt, want) {
			t.Errorf("linux prompt missing %q", want)
		}
	}

	mustNotContain := []string{
		"BSD userland",
		"PowerShell cmdlets",
	}
	for _, bad := range mustNotContain {
		if strings.Contains(prompt, bad) {
			t.Errorf("linux prompt should not contain %q", bad)
		}
	}
}

func TestBuildSystemPrompt_WindowsPlatformHints(t *testing.T) {
	prompt := BuildSystemPrompt("windows/amd64", "powershell", "5.1", "C:\\Users\\test", false)

	mustContain := []string{
		"PowerShell cmdlets",
		"cmd.exe",
		"Do NOT use Unix commands",
	}
	for _, want := range mustContain {
		if !strings.Contains(prompt, want) {
			t.Errorf("windows prompt missing %q", want)
		}
	}

	mustNotContain := []string{
		"BSD userland",
		"uses GNU coreutils",
	}
	for _, bad := range mustNotContain {
		if strings.Contains(prompt, bad) {
			t.Errorf("windows prompt should not contain %q", bad)
		}
	}
}

// --- AppendMemories ---

func TestAppendMemories_Empty(t *testing.T) {
	prompt := "base prompt"
	result := AppendMemories(prompt, nil)
	if result != prompt {
		t.Errorf("AppendMemories with nil = %q, want original prompt", result)
	}

	result = AppendMemories(prompt, []MemoryContext{})
	if result != prompt {
		t.Errorf("AppendMemories with empty = %q, want original prompt", result)
	}
}

func TestAppendMemories_SingleMemory(t *testing.T) {
	prompt := "system prompt"
	memories := []MemoryContext{
		{Keyword: "docker", Content: "use docker compose v2"},
	}
	result := AppendMemories(prompt, memories)
	if !strings.Contains(result, "system prompt") {
		t.Error("result missing original prompt")
	}
	if !strings.Contains(result, "User-defined context") {
		t.Error("result missing context header")
	}
	if !strings.Contains(result, `"docker": use docker compose v2`) {
		t.Error("result missing memory content")
	}
}

func TestAppendMemories_MultipleMemories(t *testing.T) {
	prompt := "base"
	memories := []MemoryContext{
		{Keyword: "git", Content: "always use --no-ff for merges"},
		{Keyword: "python", Content: "prefer python3"},
	}
	result := AppendMemories(prompt, memories)
	if !strings.Contains(result, `"git"`) {
		t.Error("result missing git memory")
	}
	if !strings.Contains(result, `"python"`) {
		t.Error("result missing python memory")
	}
}

func TestBuildSystemPrompt_UnknownOSNoPlatformHints(t *testing.T) {
	prompt := BuildSystemPrompt("freebsd/amd64", "/bin/sh", "sh 1.0", "/home/test", false)

	platformSections := []string{
		"BSD userland",
		"uses GNU coreutils",
		"PowerShell cmdlets",
	}
	for _, bad := range platformSections {
		if strings.Contains(prompt, bad) {
			t.Errorf("unknown OS prompt should not contain %q", bad)
		}
	}

	// Verify the prompt is still well-formed with no platform hints
	if !strings.Contains(prompt, "freebsd/amd64") {
		t.Error("prompt missing OS info")
	}
	if !strings.Contains(prompt, `"type": "commands"`) {
		t.Error("prompt missing JSON instructions")
	}
}

func TestBuildSystemPrompt_ExplainTrue(t *testing.T) {
	prompt := BuildSystemPrompt("linux/amd64", "/bin/bash", "5.1", "/tmp", true)

	mustContain := []string{
		"explanation",
		"detailed explanation",
		"find . -name",
		"tar -czf",
		"grep -rn",
	}
	for _, want := range mustContain {
		if !strings.Contains(prompt, want) {
			t.Errorf("explain=true prompt missing %q", want)
		}
	}
}

func TestBuildSystemPrompt_ExplainFalse(t *testing.T) {
	prompt := BuildSystemPrompt("linux/amd64", "/bin/bash", "5.1", "/tmp", false)

	if strings.Contains(prompt, "detailed explanation") {
		t.Error("explain=false prompt should not contain explanation instructions")
	}
}

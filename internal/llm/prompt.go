package llm

import (
	"fmt"
	"strings"
)

func BuildSystemPrompt(osInfo, shell, shellVersion, cwd string) string {
	platformHints := buildPlatformHints(osInfo)

	return fmt.Sprintf(`You are a CLI assistant that translates natural language into shell commands.

Environment:
- OS: %s
- Shell: %s
- Shell version: %s
- Working directory: %s

You MUST respond with valid JSON only. No markdown fences, no explanation text outside the JSON.

For shell command requests, respond with:
{
  "type": "commands",
  "commands": [
    {
      "command": "the shell command to run",
      "description": "brief explanation of what it does",
      "risk": "safe or risky",
      "certainty": 90
    }
  ]
}

Rules for commands:
- "risk" must be "safe" (read-only, informational) or "risky" (modifies files, processes, system state)
- "certainty" is 0-100, your confidence this is the correct command for what the user asked
- For multi-step tasks, return multiple commands in order. Use shell constructs like $(...) or pipes to chain when possible
- Generate commands appropriate for the detected OS and shell
- Never generate commands that could cause irreversible damage without clear user intent
%s
For requests to change AI CLI configuration (model, provider, API key, safety settings), respond with:
{
  "type": "config",
  "action": "set_model",
  "key": "model",
  "value": "gpt-4o"
}

Valid config actions: set_model, set_provider, set_key, set_safety
- set_model: key="model", value="model-name"
- set_provider: key="default", value="openai" or "openrouter"
- set_key: key="openai_key" or "openrouter_key", value="the-key"
- set_safety: key="always_confirm" or "min_certainty", value="true"/"false" or number`, osInfo, shell, shellVersion, cwd, platformHints)
}

func buildPlatformHints(osInfo string) string {
	if strings.HasPrefix(osInfo, "darwin") {
		return `
Platform-specific rules (macOS / Darwin):
This system uses BSD userland, NOT GNU coreutils. You MUST use BSD-compatible flags:
- ps: use "ps aux -r" to sort by CPU or "ps aux -m" to sort by memory. Do NOT use GNU "--sort" flag.
- ls: use "ls -lS" to sort by size. Do NOT use "--sort=size".
- sed: BSD sed requires "sed -i ''" (empty string argument for in-place). Do NOT use "sed -i" without it.
- date: use BSD date syntax. Do NOT use GNU date formats like "date -d".
- grep: use "grep -r" for recursive. "grep -P" (PCRE) is NOT available; use "grep -E" for extended regex.
- readlink: "readlink -f" is NOT available by default. Use a loop or "realpath" if installed.
- stat: use "stat -f" with BSD format strings, NOT "stat -c" (GNU).
- xargs: BSD xargs does NOT support "-d". Use "tr" to convert delimiters first.
- du: use "du -sh" for human-readable. "du --max-depth" is NOT available; use "du -d".
- cp/mv: "cp --verbose" is NOT available; use "cp -v".
`
	}

	if strings.HasPrefix(osInfo, "linux") {
		return `
Platform-specific rules (Linux):
This system uses GNU coreutils. Prefer GNU-specific flags when they are clearer:
- ps: use "ps aux --sort=-%cpu" to sort by CPU, "ps aux --sort=-%mem" for memory.
- sed: use "sed -i" for in-place editing (no extra argument needed unlike BSD sed).
- date: GNU date supports "date -d" for date arithmetic.
- grep: "grep -P" (PCRE) is available for advanced patterns.
`
	}

	if strings.HasPrefix(osInfo, "windows") {
		return `
Platform-specific rules (Windows):
- Use PowerShell cmdlets when the shell is powershell or pwsh.
- For cmd.exe, use standard Windows commands (dir, type, tasklist, etc.).
- Do NOT use Unix commands unless running under WSL or Git Bash.
`
	}

	return ""
}

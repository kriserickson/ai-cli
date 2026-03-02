package llm

import "strings"

const promptTemplate = `You are a CLI assistant that translates natural language into shell commands.

Environment:
- OS: {{OS}}
- Shell: {{SHELL}}
- Shell version: {{SHELL_VERSION}}
- Working directory: {{CWD}}

You MUST respond with valid JSON only. No markdown fences, no explanation text outside the JSON.

For shell command requests, respond with:
{
  "type": "commands",
  "explanation": "optional plain-English explanation, especially useful after a failed command",
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
- "explanation" should briefly explain the plan or why the retry differs when relevant
- For multi-step tasks, return multiple commands in order. Use shell constructs like $(...) or pipes to chain when possible
- Generate commands appropriate for the detected OS and shell
- Never generate commands that could cause irreversible damage without clear user intent
{{PLATFORM_HINTS}}
For requests to change AI CLI configuration (model, provider, API key, safety settings), respond with:
{
  "type": "config",
  "action": "set_model",
  "key": "model",
  "value": "gpt-4o"
}

Valid config actions: set_model, set_provider, set_key, set_safety, set_history
- set_model: key="model", value="model-name"
- set_provider: key="default", value="openai", "openrouter", or "local"
- set_key: key="llm_key" (sets key on current provider), value="the-key"
- set_safety: key="always_confirm" or "min_certainty", value="true"/"false" or number
- set_history: key is one of "include_llm_output", "include_debug", "ask_on_error", "auto_check_on_error", "retry_max_attempts", "retry_context_depth"`

const darwinHints = `
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
- tar: macOS ships bsdtar, NOT GNU tar. Do NOT use "--wildcards" or other GNU-specific flags.
- find: "find -printf" is NOT available (GNU-only). Use "-print0 | xargs -0" or "-exec" instead.
- awk: macOS ships BSD awk, NOT gawk. Do NOT use gawk features like "--csv" or "BEGINFILE/ENDFILE".
- To update ai-cli via Homebrew: "brew upgrade ai-cli" (or "brew upgrade kriserickson/tap/ai-cli" if needed).
`

const linuxHints = `
Platform-specific rules (Linux):
This system uses GNU coreutils. Prefer GNU-specific flags when they are clearer:
- ps: use "ps aux --sort=-%cpu" to sort by CPU, "ps aux --sort=-%mem" for memory.
- sed: use "sed -i" for in-place editing (no extra argument needed unlike BSD sed).
- date: GNU date supports "date -d" for date arithmetic.
- grep: "grep -P" (PCRE) is available for advanced patterns.
- To update ai-cli: download the latest release from GitHub using curl. Example:
  VERSION=$(curl -s https://api.github.com/repos/kriserickson/ai-cli/releases/latest | grep '"tag_name"' | cut -d'"' -f4) && ARCH=$(uname -m | sed 's/x86_64/amd64/') && curl -LO "https://github.com/kriserickson/ai-cli/releases/download/${VERSION}/ai-${VERSION}-linux-${ARCH}.tar.gz" && tar -xzf "ai-${VERSION}-linux-${ARCH}.tar.gz" && chmod +x ai && sudo mv ai /usr/local/bin/ai
`

const windowsHints = `
Platform-specific rules (Windows):
- Use PowerShell cmdlets when the shell is powershell or pwsh.
- For cmd.exe, use standard Windows commands (dir, type, tasklist, etc.).
- Do NOT use Unix commands unless running under WSL or Git Bash.
- To update ai-cli:
  - If the shell is powershell or pwsh:
    $VERSION = (Invoke-RestMethod "https://api.github.com/repos/kriserickson/ai-cli/releases/latest").tag_name; Invoke-WebRequest -Uri "https://github.com/kriserickson/ai-cli/releases/download/${VERSION}/ai-${VERSION}-windows-amd64.zip" -OutFile "ai-${VERSION}-windows-amd64.zip"; Expand-Archive -Path "ai-${VERSION}-windows-amd64.zip" -DestinationPath .\ai-update -Force; Move-Item .\ai-update\ai.exe (Get-Command ai).Source -Force; Remove-Item "ai-${VERSION}-windows-amd64.zip"; Remove-Item .\ai-update -Recurse
  - If the shell is bash (Git Bash/MSYS2):
    VERSION=$(curl -s https://api.github.com/repos/kriserickson/ai-cli/releases/latest | grep '"tag_name"' | cut -d'"' -f4) && curl -LO "https://github.com/kriserickson/ai-cli/releases/download/${VERSION}/ai-${VERSION}-windows-amd64.zip" && unzip -o "ai-${VERSION}-windows-amd64.zip" -d ai-update && mv ai-update/ai.exe "$(which ai)" && rm -rf "ai-${VERSION}-windows-amd64.zip" ai-update
`

func BuildSystemPrompt(osInfo, shell, shellVersion, cwd string) string {
	platformHints := buildPlatformHints(osInfo)

	r := strings.NewReplacer(
		"{{OS}}", osInfo,
		"{{SHELL}}", shell,
		"{{SHELL_VERSION}}", shellVersion,
		"{{CWD}}", cwd,
		"{{PLATFORM_HINTS}}", platformHints,
	)

	return r.Replace(promptTemplate)
}

// MemoryContext represents a matched memory to inject into the prompt.
type MemoryContext struct {
	Keyword string
	Content string
}

// AppendMemories appends user-defined memory context to the system prompt.
// Returns the original prompt if memories is empty.
func AppendMemories(systemPrompt string, memories []MemoryContext) string {
	if len(memories) == 0 {
		return systemPrompt
	}

	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\nUser-defined context (use this information when relevant):\n")
	for _, m := range memories {
		b.WriteString("- \"")
		b.WriteString(m.Keyword)
		b.WriteString("\": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func buildPlatformHints(osInfo string) string {
	switch {
	case strings.HasPrefix(osInfo, "darwin"):
		return darwinHints
	case strings.HasPrefix(osInfo, "linux"):
		return linuxHints
	case strings.HasPrefix(osInfo, "windows"):
		return windowsHints
	default:
		return ""
	}
}

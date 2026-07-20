# AI CLI

AI CLI translates natural language into shell commands using LLMs (OpenAI, OpenRouter, or a local server), then applies a safety policy before execution.

![AI CLI banner](images/ai-cli-banner.png)

## Important Safety Warning

LLMs are non-deterministic. The same prompt can produce different commands across runs.

Always review and understand every command before you approve or run it. Treat generated commands as untrusted suggestions, especially for file operations, process control, networking, and anything requiring elevated permissions.

## Quick Start (Users)

### 1) Install

#### Homebrew (macOS and Linux)

```sh
brew install kriserickson/tap/ai-cli
```

#### Go

Requires Go 1.25+.

```sh
go install github.com/kriserickson/ai-cli@latest
```

#### Binary releases

Download prebuilt binaries for macOS (signed and notarized), Linux, and Windows from the [latest release](https://github.com/kriserickson/ai-cli/releases/latest).

On macOS or Linux, extract the archive, make it executable, and move it onto your `PATH`:

```sh
VERSION=$(curl -s https://api.github.com/repos/kriserickson/ai-cli/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/')
curl -LO "https://github.com/kriserickson/ai-cli/releases/download/${VERSION}/ai-${VERSION}-${OS}-${ARCH}.tar.gz"
tar -xzf "ai-${VERSION}-${OS}-${ARCH}.tar.gz"
chmod +x ai
sudo mv ai /usr/local/bin/ai
ai version
```

On Windows (PowerShell), extract the zip and move `ai.exe` onto your `PATH`:

```powershell
$VERSION = (Invoke-RestMethod "https://api.github.com/repos/kriserickson/ai-cli/releases/latest").tag_name
Invoke-WebRequest -Uri "https://github.com/kriserickson/ai-cli/releases/download/${VERSION}/ai-${VERSION}-windows-amd64.zip" -OutFile "ai-${VERSION}-windows-amd64.zip"
Expand-Archive -Path "ai-${VERSION}-windows-amd64.zip" -DestinationPath .\ai
Move-Item .\ai\ai.exe "$env:USERPROFILE\go\bin\ai.exe" -Force
ai.exe version
```

### 2) Run Setup and Health Checks First

Use `ai doctor` as the first command. It verifies config and launches setup if your API key is missing.

```sh
ai doctor
```

Typical flow:

- validates config file location
- checks selected provider and model
- checks API key presence
- starts setup wizard if needed

### Recommended: Shell Alias for Special Characters

Characters like `?`, `*`, and `#` have special meaning in most shells. Without protection, a command like `ai what is using all my cpu?` will fail because zsh tries to glob-expand `cpu?` before `ai` ever sees it.

Add a `noglob` alias to your shell config so you can type naturally:

**zsh** (`~/.zshrc`):

```sh
alias ai='noglob ai'
```

**bash** (`~/.bashrc`):

```sh
# Only needed if you have failglob or nullglob enabled;
# noglob is zsh-only, so bash needs a wrapper function instead.
ai() { set -f; command ai "$@"; set +f; }
```

Then reload your shell:

```sh
source ~/.zshrc   # or source ~/.bashrc
```

After this, `ai what is using all my cpu?` will work as expected.

### 3) Run Your First Commands

Single-shot examples:

```sh
ai list files in current directory sorted by size
ai find all files larger than 10MB
ai show what process is using port 8080

# Use a configured model tier for one command
ai --light summarize these logs
ai --high diagnose this difficult build failure
```

Interactive mode:

```sh
ai
```

Then type requests one by one:

```text
ai> what ports are listening
ai> compress all log files in /var/log older than 7 days
ai> show biggest folders in my home directory
ai> model-level high
```

## How Command Execution Safety Works

Every generated command has:

- `risk`: `safe` or `risky`
- `certainty`: 0-100

Decision matrix:

| Risk | Allowlisted | Certainty >= threshold | Action |
|------|-------------|------------------------|--------|
| safe | any | yes | Auto-execute |
| safe | any | no | Ask confirmation |
| risky | any | any | Ask confirmation |

When `always_confirm=true`, every command asks first.

Default allowlist prefixes:

- `git`
- `ls`
- `cat`
- `echo`
- `pwd`
- `head`
- `tail`
- `wc`
- `grep`
- `find`
- `which`
- `man`

## Restrict or Expand Auto-Run Behavior

### Restrict auto-run (safer)

Force confirmation for everything:

```sh
ai config set always_confirm true
```

Increase certainty required for auto-run:

```sh
ai config set min_certainty 95
```

### Turn auto-run on more aggressively

Allow auto-run decisions from the safety matrix:

```sh
ai config set always_confirm false
```

Lower certainty threshold:

```sh
ai config set min_certainty 60
```

Important:

- all risky commands prompt for confirmation, regardless of threshold or allowlist
- `min_certainty` only affects whether safe commands auto-run

### Allowlist control

Current CLI supports reading allowlist via config output, but not setting it directly with `ai config set`.

To customize allowlist prefixes, edit `~/.ai-cli/config.toml`:

```toml
[safety]
always_confirm = false
tool_calling = "never"
min_certainty = 80
allowlist_prefixes = ["git", "ls", "cat", "echo", "pwd", "head", "tail", "wc", "grep", "find", "which", "man"]
```

After editing, run:

```sh
ai status
```

## Tool Calling

AI CLI can optionally let the model use built-in read-only tools before it generates shell commands. This is controlled by `safety.tool_calling`.

| Mode | Description |
|------|-------------|
| `never` | Tools are fully disabled. No tool instructions are added to the prompt and no tool loop runs. Default. |
| `always_prompt` | Prompt before every tool call. |
| `dangerous_prompt` | Auto-approve safe tool calls, but prompt when a tool triggers a safety rule. |
| `always_allow` | Execute all tool calls without prompting. |

Set it with:

```sh
ai config set tool_calling never
ai config set tool_calling always_prompt
ai config set tool_calling dangerous_prompt
ai config set tool_calling always_allow
```

Notes:

- `tool_calling` only controls AI tool usage
- generated shell commands still follow `always_confirm`, `min_certainty`, risk classification, and the allowlist
- `dangerous_prompt` prompts when a tool call hits a safety rule, including restricted file access and sensitive tools like `environment` or `list_memories`
- `never` makes AI CLI skip the tool loop entirely and respond directly

## Memories

Memories let you store named context (like server addresses, port mappings, or project conventions) that automatically gets injected into the AI prompt when the keyword appears in your input.

### Managing memories

```sh
ai memory add my-server "user@10.1.1.103 -L 9229:localhost:2229"
ai memory add staging-db "postgres://app:secret@staging.example.com:5432/mydb"
ai memory list
ai memory remove my-server
```

### Using memories

Once stored, just use the keyword naturally:

```sh
ai connect to my-server
# → ssh user@10.1.1.103 -L 9229:localhost:2229

ai dump the users table from staging-db
# → pg_dump -t users "postgres://app:secret@staging.example.com:5432/mydb"
```

Keyword matching is case-insensitive. Multiple memories can match in a single request. Memories are stored in `~/.ai-cli/memory.json`.

In interactive mode, the same commands are available:

```text
ai> memory add my-server "user@10.1.1.103 -L 9229:localhost:2229"
ai> memory list
ai> memory remove my-server
```

## Common User Commands

```sh
ai status
ai doctor
ai set-model
ai set-model light
ai set-model high
ai version
ai config show
ai config get provider
ai config set provider openai
ai config set llm_key sk-your-key-here
```

## Shell Completion

`ai completion` generates an autocompletion script for your shell so you can tab-complete subcommands and flags.

### zsh

Enable completion in your environment (once, if not already done):

```sh
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

Load for the current session:

```sh
source <(ai completion zsh)
```

Load permanently (macOS with Homebrew):

```sh
ai completion zsh > $(brew --prefix)/share/zsh/site-functions/_ai
```

Load permanently (Linux):

```sh
ai completion zsh > "${fpath[1]}/_ai"
```

### bash

Requires the `bash-completion` package (`brew install bash-completion` on macOS, or your distro's package manager on Linux).

Load for the current session:

```sh
source <(ai completion bash)
```

Load permanently (macOS):

```sh
ai completion bash > $(brew --prefix)/etc/bash_completion.d/ai
```

Load permanently (Linux):

```sh
ai completion bash > /etc/bash_completion.d/ai
```

### fish

Load for the current session:

```sh
ai completion fish | source
```

Load permanently:

```sh
ai completion fish > ~/.config/fish/completions/ai.fish
```

### PowerShell

Load for the current session:

```powershell
ai completion powershell | Out-String | Invoke-Expression
```

Load permanently — add the above line to your PowerShell profile (`$PROFILE`).

---

Start a new shell after any permanent installation for changes to take effect.

## Configuration

Configuration file location:

- `~/.ai-cli/config.toml`

Available `ai config get/set` keys:

| Key | Description | Default |
|-----|-------------|---------|
| `provider` | Active provider (`openai`, `openrouter`, or `local`) | `openrouter` |
| `model` | Model identifier | `anthropic/claude-3.5-sonnet` |
| `provider_light` | Provider for the light tier; blank inherits `provider` | (empty) |
| `model_light` | Model identifier for `--light` | (empty) |
| `provider_high` | Provider for the high tier; blank inherits `provider` | (empty) |
| `model_high` | Model identifier for `--high` | (empty) |
| `model_parameters` | Default-tier request parameters as a JSON object | `{}` |
| `parameters_light` | Light-tier request parameters as a JSON object | `{}` |
| `parameters_high` | High-tier request parameters as a JSON object | `{}` |
| `upgrade_model_on_fail` | Retry a failed command with the next model tier | `false` |
| `llm_key` | API key for the current provider | (empty) |
| `llm_url` | Base URL for the current provider | (provider default) |
| `always_confirm` | Always prompt before execution (`true`/`false`) | `false` |
| `tool_calling` | Tool usage mode (`never`, `always_prompt`, `dangerous_prompt`, `always_allow`) | `never` |
| `min_certainty` | Auto-execute threshold (0-100) | `80` |
| `debug` | Debug mode (`none`, `screen`, `file`) | `none` |
| `history_include_llm_output` | Store structured LLM responses in history (`true`/`false`) | `true` |
| `history_include_debug` | Store raw request/response debug payloads in history (`true`/`false`) | `false` |
| `history_ask_on_error` | Prompt to send failures back to the AI (`true`/`false`) | `true` |
| `history_auto_check_on_error` | Automatically ask the AI to retry on failure (`true`/`false`) | `false` |
| `history_retry_max_attempts` | Automatic AI retries after a failed command | `1` |
| `history_retry_context_depth` | Number of recent command results to send back on retry | `3` |
| `debug_log_payloads` | When `debug=file`, include full prompts and responses in `llm.log` (`true`/`false`) | `false` |

Notes:

- `llm_key` and `llm_url` read and write the key/URL for whichever provider is currently selected
- `ai config get llm_key` returns a masked value
- `allowlist_prefixes` exists in config but is currently edited directly in `config.toml`

### Model tiers and parameters

The existing `provider` and `model` settings are the default (medium) tier. Configure the optional tiers with the setup wizard:

```sh
ai set-model light
ai set-model default
ai set-model high
```

The wizard selects a provider and model, displays parameter guidance for OpenAI, Anthropic, Google, or local model families, and accepts a JSON object of model parameters. You can also set parameters directly:

```sh
ai config set parameters_light '{"temperature":0.2}'
ai config set model_parameters '{"reasoning_effort":"medium"}'
ai config set parameters_high '{"reasoning_effort":"high"}'
```

For OpenAI-compatible endpoints, parameters are merged into the Chat Completions request. For an Ollama-compatible local provider, they are sent in `options`. Model support varies: GPT-4-family models commonly use `temperature`; GPT-5-family reasoning models use `reasoning_effort`. The wizard also explains the normalized OpenRouter controls for current Claude and Gemini models.

Choose a tier for a single-shot command or interactive session:

```sh
ai --light list files
ai --high diagnose the failed deployment

ai
ai> model-level
ai> model-level light
ai> model-level default
ai> model-level high
```

`--light` and `--high` are mutually exclusive. In interactive mode, `model-level` changes only the current session. If `upgrade_model_on_fail=true`, a retry after a command failure uses one higher tier (`light` to `default`, or `default` to `high`) and then restores the original tier. Automatic retries still require `history_auto_check_on_error=true`; prompted or manual retries also use the upgrade.

## Debugging

```sh
# Persisted config
ai config set debug screen
ai config set debug file
ai config set debug_log_payloads true   # opt in to full prompt/response capture for debug=file

# Per-command override
ai --debug list files
ai --debug=screen list files
ai --debug=file list files
```

Debug output still goes to the configured debug sink (`screen` or `~/.ai-cli/llm.log`). History storage is separate and controlled by the `history_*` settings above.

## History And Retry

Each top-level AI instruction now writes to a rotating history log under `~/.ai-cli/history.log` with rotated backups like `history.log.1`.

- New instructions start a fresh session.
- If a generated command fails, AI CLI can prompt to send the failure back to the model with the original request, prior AI response, recent command results, exit code, and captured stdout/stderr.
- In interactive mode, `retry` replays the last failed AI session, and `retry <depth>` overrides how many recent command results get included.

Example:

```sh
ai config set history_auto_check_on_error true
ai config set history_retry_context_depth 5

# Per-command override
ai --retry-on-error --retry-depth 5 fix the broken command

# Inspect stored sessions
ai history list
ai history show 1741012345678901
```

Notes:

- `debug=screen` prints request and response payloads to the terminal for the current session
- `debug=file` writes to `~/.ai-cli/llm.log`
- `debug=file` logs metadata only by default; full prompt and response bodies require `debug_log_payloads=true`
- `llm.log` is created with user-only permissions

## Security Notes

- `ai config show` and `ai config get llm_key` mask stored API keys
- tool output is treated as untrusted data before it is sent back to the model
- the `environment` tool shows all variable names, but only a small allowlisted subset of values; all other values are hidden or redacted
- memories and environment data can still be sensitive, so `dangerous_prompt` asks before using those tools
- enabling `debug=file` can still store sensitive data if you also set `debug_log_payloads=true`

## Multi-Step Commands

For complex requests, AI CLI may return multiple commands. They run sequentially and stop on first failure unless an AI retry is triggered.

```sh
$ ai kill the process on port 8080

[1/2] Find PID on port 8080
  $ lsof -ti :8080  [safe] 95% certainty
[2/2] Kill the process
  $ kill -9 $(lsof -ti :8080)  [risky] 90% certainty
Execute? [Y/n]
```

## Developer Guide

### Build, Test, and Install (go-task)

Requires Go 1.25+ and `go-task`.

Install `task` once:

```sh
go install github.com/go-task/task/v3/cmd/task@latest
```

Ensure your Go bin directory is in `PATH`:

- Windows: `%USERPROFILE%\\go\\bin`
- macOS/Linux: `$HOME/go/bin`

Example for zsh:

```sh
echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.zshrc
source ~/.zshrc
```

Common commands:

```sh
task             # build + test
task build       # dist/<os>/ai
task test
task test:verbose
task test:pkg PKG=./internal/llm/...
task install
```

`task install` copies from `dist/<os>/` into:

- Windows: `%USERPROFILE%\\go\\bin`
- macOS/Linux: `/usr/local/bin`

Override install target:

```sh
task install INSTALL_DIR="$HOME/.local/bin"
```

Windows (PowerShell):

```powershell
task install INSTALL_DIR="$env:USERPROFILE\\tools\\bin"
```

### Versioning and Releases

Version is injected at build time using ldflags. Default value in source is `dev`.

Create a release:

```sh
git tag v0.2.0
git push origin v0.2.0
```

Tag push triggers GitHub Actions to:

- run tests
- build cross-platform artifacts
- create a GitHub Release
- upload artifacts to the release

### Testing

```sh
task test
task test:verbose
task test:pkg PKG=./internal/executor/...
task test:pkg PKG=./internal/llm/...
task test:coverage
```

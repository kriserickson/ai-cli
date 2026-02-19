# AI CLI

AI CLI translates natural language into shell commands using LLMs (OpenAI or OpenRouter), then applies a safety policy before execution.

## Important Safety Warning

LLMs are non-deterministic. The same prompt can produce different commands across runs.

Always review and understand every command before you approve or run it. Treat generated commands as untrusted suggestions, especially for file operations, process control, networking, and anything requiring elevated permissions.

## Quick Start (Users)

### 1) Install from GitHub Release Artifacts

Each release publishes prebuilt artifacts for macOS, Linux, and Windows.

Artifact names:

- `ai-vX.Y.Z-darwin-amd64.tar.gz`
- `ai-vX.Y.Z-linux-amd64.tar.gz`
- `ai-vX.Y.Z-windows-amd64.zip`

Download from your release page:

- `https://github.com/kriserickson/ai-cli/releases`

Install on macOS/Linux:

```sh
# Example for Linux v0.2.0
curl -LO https://github.com/kriserickson/ai-cli/releases/download/v0.1.0/ai-v0.1.0-linux-amd64.tar.gz
tar -xzf ai-v0.1.0-linux-amd64.tar.gz
chmod +x ai
sudo mv ai /usr/local/bin/ai
ai version
```

Install on Windows (PowerShell):

```powershell
# Example for Windows v0.2.0
Invoke-WebRequest -Uri "https://github.com/kriserickson/ai-cli/releases/download/v0.2.0/ai-v0.2.0-windows-amd64.zip" -OutFile "ai-v0.2.0-windows-amd64.zip"
Expand-Archive -Path "ai-v0.2.0-windows-amd64.zip" -DestinationPath ".\\ai"
Move-Item ".\\ai\\ai.exe" "$env:USERPROFILE\\go\\bin\\ai.exe" -Force
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

### 3) Run Your First Commands

Single-shot examples:

```sh
ai list files in current directory sorted by size
ai find all files larger than 10MB
ai show what process is using port 8080
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
```

## How Command Execution Safety Works

Every generated command has:

- `risk`: `safe` or `risky`
- `certainty`: 0-100

Decision matrix:

| Risk | Whitelisted | Certainty >= threshold | Action |
|------|-------------|------------------------|--------|
| safe | any | yes | Auto-execute |
| safe | any | no | Ask confirmation |
| risky | any | any | Ask confirmation |

When `always_confirm=true`, every command asks first.

Default whitelist prefixes:

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

- all risky commands prompt for confirmation, regardless of threshold or whitelist
- `min_certainty` only affects whether safe commands auto-run

### Whitelist control

Current CLI supports reading whitelist via config output, but not setting it directly with `ai config set`.

To customize whitelist prefixes, edit `~/.ai-cli/config.toml`:

```toml
[safety]
always_confirm = false
min_certainty = 80
whitelist_prefixes = ["git", "ls", "cat", "echo", "pwd", "head", "tail", "wc", "grep", "find", "which", "man"]
```

After editing, run:

```sh
ai status
```

## Common User Commands

```sh
ai status
ai doctor
ai set-model
ai version
ai config show
ai config get provider
ai config set provider openai
ai config set openai_key sk-your-key-here
```

## Configuration

Configuration file location:

- `~/.ai-cli/config.toml`

Available `ai config get/set` keys:

| Key | Description | Default |
|-----|-------------|---------|
| `provider` | Active provider (`openai` or `openrouter`) | `openrouter` |
| `model` | Model identifier | `anthropic/claude-3.5-sonnet` |
| `openai_key` | OpenAI API key | (empty) |
| `openrouter_key` | OpenRouter API key | (empty) |
| `openai_url` | OpenAI base URL | `https://api.openai.com/v1` |
| `openrouter_url` | OpenRouter base URL | `https://openrouter.ai/api/v1` |
| `always_confirm` | Always prompt before execution (`true`/`false`) | `false` |
| `min_certainty` | Auto-execute threshold (0-100) | `80` |
| `debug` | Debug mode (`none`, `screen`, `file`) | `none` |

Notes:

- `whitelist_prefixes` exists in config but is currently edited directly in `config.toml`
- `ai config get openai_key` and `ai config get openrouter_key` return masked values

## Debugging

```sh
# Persisted config
ai config set debug screen
ai config set debug file

# Per-command override
ai --debug list files
ai --debug=screen list files
ai --debug=file list files
```

## Multi-Step Commands

For complex requests, AI CLI may return multiple commands. They run sequentially and stop on first failure.

```text
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
task test:pkg PKG=./internal/config/...
task test:pkg PKG=./internal/shell/...
```

### Project Structure

```text
ai-cli/
├── main.go
├── go.mod
├── cmd/
│   ├── root.go
│   ├── config.go
│   ├── version.go
│   ├── status.go
│   ├── doctor.go
│   ├── setmodel.go
│   └── wizard.go
└── internal/
    ├── config/
    │   ├── config.go            # TOML config load/save/defaults (~/.ai-cli/config.toml)
    │   └── config_test.go
    ├── llm/
    │   ├── client.go            # LLM HTTP client (OpenAI-compatible), debug logging
    │   ├── client_test.go
    │   ├── models.go            # Model-list fetching (OpenRouter + OpenAI), GroupByCompany
    │   ├── models_test.go
    │   ├── prompt.go            # System prompt template with OS/shell/cwd context
    │   ├── prompt_test.go
    │   ├── parse_test.go
    │   └── types.go             # JSON request/response structs
    ├── executor/
    │   ├── executor.go          # Sequential command execution with colored output
    │   ├── safety.go            # Whitelist check + risk/certainty safety matrix
    │   └── safety_test.go
    ├── shell/
    │   ├── detect.go            # OS, shell, version detection
    │   └── detect_test.go
    └── interactive/
```

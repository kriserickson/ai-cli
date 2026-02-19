# AI CLI

A Go command-line tool that translates natural language into shell commands using LLMs (OpenAI and OpenRouter). It detects your OS and shell, generates appropriate commands, assesses risk, and optionally auto-executes safe commands.

## Build, Test, and Install (go-task)

Requires Go 1.25+ and `go-task`.

Install `task` once:

```sh
go install github.com/go-task/task/v3/cmd/task@latest
```

Then make sure your Go bin directory is on `PATH`:

- Windows: `%USERPROFILE%\go\bin`
- macOS / Linux: `$HOME/go/bin`

Common project commands:

```sh
task             # build + test
task build       # dist/<os>/ai
task test
task test:verbose
task test:pkg PKG=./internal/llm/...
task install
```

`task install` copies the built binary from `dist/<os>/` into an OS-specific PATH directory.

Default install targets:

- Windows: `%USERPROFILE%\go\bin`
- macOS / Linux: `/usr/local/bin`

Override the install target:

**macOS / Linux:**
```sh
task install INSTALL_DIR="$HOME/.local/bin"
```

**Windows (PowerShell):**
```powershell
task install INSTALL_DIR="$env:USERPROFILE\tools\bin"
```

## Quick Start

The easiest way to get started is `ai doctor` — it checks your configuration and runs the interactive setup wizard if no API key is found:

```sh
task install
ai doctor
```

Or set everything up manually:

**macOS / Linux:**
```sh
# Set your API key
./ai config set openai_key sk-your-key-here
./ai config set provider openai

# Or use OpenRouter
./ai config set openrouter_key sk-or-your-key-here
./ai config set provider openrouter
./ai config set model anthropic/claude-3.5-sonnet

# Single-shot mode
./ai list files in current directory
./ai find all go files larger than 1MB
./ai kill the process on port 8080

# Interactive mode (no arguments)
./ai
```

**Windows (PowerShell or cmd):**
```sh
# Set your API key
ai.exe config set openai_key sk-your-key-here
ai.exe config set provider openai

# Or use OpenRouter
ai.exe config set openrouter_key sk-or-your-key-here
ai.exe config set provider openrouter
ai.exe config set model anthropic/claude-3.5-sonnet

# Single-shot mode
ai.exe list files in current directory
ai.exe find all go files larger than 1MB

# Interactive mode (no arguments)
ai.exe
```

## Usage

### Single-Shot Mode

Pass your instruction as arguments:

```sh
./ai <natural language instruction>
```

The tool sends your instruction to the configured LLM along with your OS, shell, and working directory context. The LLM returns one or more shell commands which are displayed with a risk assessment and certainty score before execution.

```
$ ./ai show disk usage sorted by size

Show disk usage of current directory sorted by size
  $ du -sh * | sort -rh  [safe] 92% certainty
```

### Interactive Mode

Run with no arguments to enter a readline-powered REPL with command history:

```
$ ./ai
AI CLI interactive mode. Type 'exit' or 'quit' to leave.
ai> what ports are listening
ai> compress all log files in /var/log
ai> exit
Bye!
```

History is saved to `~/.ai-cli/history`.

### Configuration

Configuration is stored in `~/.ai-cli/config.toml` and created with defaults on first run.

```sh
./ai config show            # Show full config
./ai config get <key>       # Get a single value
./ai config set <key> <val> # Set a value
```

Available config keys:

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

### Status, Doctor, and Model Selection

#### `ai status`

Prints a one-line snapshot of the current installation:

```
$ ./ai status
ai-cli v0.1.0
Config:   /home/user/.ai-cli/config.toml  ✓ exists
Log:      /home/user/.ai-cli/llm.log      — not created yet
Provider: openrouter
Model:    anthropic/claude-3.5-sonnet
API Key:  not set
```

The API key is shown masked (e.g. `sk-or-****...1234`) if set, or highlighted in red if missing.

#### `ai doctor`

Runs a health check and automatically launches the setup wizard if no API key is configured:

```
$ ./ai doctor
Checking configuration...
  ✓ Config file: /home/user/.ai-cli/config.toml
  ✗ API key: No API key configured for openrouter
    Running setup wizard...
    ...
  ✓ Model: anthropic/claude-3.5-sonnet
All checks passed!
```

If the API key is already set, doctor simply confirms everything is in order without prompting.

#### `ai set-model`

Opens the interactive wizard to pick a new provider and model at any time — useful when you want to switch between OpenAI and OpenRouter or try a different model:

```sh
./ai set-model
```

The wizard always asks for the provider first, then prompts for an API key only if one isn't already saved, then shows an arrow-key selector to choose the model. For OpenRouter the model list is grouped by company; for OpenAI it shows GPT models sorted newest-first. The selection is saved to config when you confirm.

### Self-Configuration via Natural Language

You can change settings by asking in natural language:

```sh
./ai change model to gpt-4o
./ai switch to openai provider
```

The tool will show the proposed config change and ask for confirmation before applying.

### Debugging

Debug mode logs the full JSON request sent to the LLM API and the raw response.

```sh
# Set persistently
./ai config set debug screen   # Print to stderr
./ai config set debug file     # Append to ~/.ai-cli/llm.log

# Or override per-invocation
./ai --debug list files          # prints to stderr (default)
./ai --debug=screen list files   # same as above
./ai --debug=file list files     # appends to ~/.ai-cli/llm.log
```

## Safety System

Every command returned by the LLM includes a **risk** level (`safe` or `risky`) and a **certainty** percentage. These determine whether the command auto-executes or requires confirmation:

| Risk | Whitelisted | Certainty >= threshold | Action |
|------|-------------|------------------------|--------|
| safe | any | yes | Auto-execute |
| safe | any | no | Ask confirmation |
| risky | yes | yes | Auto-execute |
| risky | yes | no | Ask confirmation |
| risky | no | any | Ask confirmation |

When `always_confirm` is `true`, every command prompts regardless.

The default whitelist includes: `git`, `ls`, `cat`, `echo`, `pwd`, `head`, `tail`, `wc`, `grep`, `find`, `which`, `man`.

### Multi-Step Commands

For complex requests, the LLM may return multiple commands that execute sequentially. Execution stops on the first failure.

```
$ ./ai kill the process on port 8080

[1/2] Find PID on port 8080
  $ lsof -ti :8080  [safe] 95% certainty
[2/2] Kill the process
  $ kill -9 $(lsof -ti :8080)  [risky] 90% certainty
Execute? [Y/n]
```

## Testing

Use go-task:

```sh
task test
task test:verbose
task test:pkg PKG=./internal/executor/...
task test:pkg PKG=./internal/llm/...
task test:pkg PKG=./internal/config/...
task test:pkg PKG=./internal/shell/...
```

### Test Coverage

| Package | Test File | What's Tested |
|---------|-----------|---------------|
| `internal/executor` | `safety_test.go` | Safety matrix (all risk/certainty/whitelist combinations), `always_confirm` override, whitelist prefix matching including edge cases (partial prefix matches, leading whitespace, empty input) |
| `internal/llm` | `parse_test.go` | JSON response parsing: plain JSON, markdown-fenced JSON (with and without language tag), whitespace handling, config-type responses, multi-command responses, invalid JSON, empty input |
| `internal/llm` | `client_test.go` | Client creation (missing API key, unknown provider), HTTP integration via httptest (successful chat, API errors, empty choices), debug output capture |
| `internal/llm` | `models_test.go` | `FetchOpenRouterModels` (company extraction, "Other" fallback, HTTP errors), `FetchOpenAIModels` (gpt-* filter, created-desc sort, Bearer auth, HTTP errors), `GroupByCompany` (alphabetical groups, model order preserved, nil input) |
| `internal/llm` | `prompt_test.go` | System prompt contains environment info (OS, shell, version, cwd) and required JSON structure instructions |
| `internal/config` | `config_test.go` | Default config values, save/load round-trip using temp dir, auto-creation of default config on first load, TOML marshal/unmarshal round-trip |
| `internal/shell` | `detect_test.go` | OS detection matches runtime, shell/version detection returns non-empty values, `shellBaseName` with Unix and Windows paths, `ShellCommand` args for zsh/bash/powershell/cmd |

The LLM client tests use `net/http/httptest` to run a local HTTP server, so they don't require an API key or network access.

## Project Structure

```
ai-cli/
├── main.go                      # Entry point
├── go.mod                       # Go module (cobra, go-toml, readline, color, survey)
├── cmd/
│   ├── root.go                  # Root command: single-shot and interactive mode routing
│   ├── config.go                # `ai config show/get/set` subcommands
│   ├── version.go               # `ai version` subcommand
│   ├── status.go                # `ai status` — configuration snapshot
│   ├── doctor.go                # `ai doctor` — health check + setup wizard trigger
│   ├── setmodel.go              # `ai set-model` — interactive provider/model picker
│   └── wizard.go                # Shared interactive wizard (survey-based selectors)
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
        └── repl.go              # Readline REPL with history
```

### Key Design Decisions

- **No OpenAI SDK** — uses raw `net/http` since the chat completions API is simple and OpenRouter is wire-compatible.
- **Strict JSON responses** — the system prompt instructs the LLM to return structured JSON. A fallback strips markdown code fences if the LLM wraps the response.
- **Shell-aware execution** — commands run via the detected shell (`zsh -c`, `bash -c`, `powershell -Command`, `cmd /c`) so shell features like pipes, globs, and `$(...)` work correctly.

## Dependencies

| Package | Purpose |
|---------|---------|
| [cobra](https://github.com/spf13/cobra) | CLI framework and subcommand routing |
| [go-toml/v2](https://github.com/pelletier/go-toml) | TOML config file parsing |
| [readline](https://github.com/chzyer/readline) | Interactive mode line editing and history |
| [color](https://github.com/fatih/color) | Colored terminal output |
| [survey/v2](https://github.com/AlecAivazis/survey) | Interactive arrow-key selectors and password prompts for the setup wizard |

# AI CLI

A Go command-line tool that translates natural language into shell commands using LLMs (OpenAI and OpenRouter). It detects your OS and shell, generates appropriate commands, assesses risk, and optionally auto-executes safe commands.

## Building

Requires Go 1.25+.

```sh
go build -o ai .
```

## Quick Start

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
./ai --debug screen list files
./ai --debug file list files
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

## Project Structure

```
ai-cli/
├── main.go                      # Entry point
├── go.mod                       # Go module (cobra, go-toml, readline, color)
├── cmd/
│   ├── root.go                  # Root command: single-shot and interactive mode routing
│   ├── config.go                # `ai config show/get/set` subcommands
│   └── version.go               # `ai version` subcommand
└── internal/
    ├── config/
    │   └── config.go            # TOML config load/save/defaults (~/.ai-cli/config.toml)
    ├── llm/
    │   ├── client.go            # LLM HTTP client (OpenAI-compatible), debug logging
    │   ├── prompt.go            # System prompt template with OS/shell/cwd context
    │   └── types.go             # JSON request/response structs
    ├── executor/
    │   ├── executor.go          # Sequential command execution with colored output
    │   └── safety.go            # Whitelist check + risk/certainty safety matrix
    ├── shell/
    │   └── detect.go            # OS, shell, version detection
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

package interactive

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
)

type replLineReader interface {
	Readline() (string, error)
	Close() error
}

var (
	replConfigDir         = config.Dir
	replNewReadline       = func(cfg *readline.Config) (replLineReader, error) { return readline.NewEx(cfg) }
	replBuildSystemPrompt = func(osName, shellName, shellVersion, cwd string) string {
		return llm.BuildSystemPrompt(osName, shellName, shellVersion, cwd, false)
	}
	replHandleResponse = handleResponse
)

// BuiltinCommands holds handlers for built-in REPL commands so the interactive
// package doesn't need to import the cmd package (which would be circular).
type BuiltinCommands struct {
	Status   func() error
	Doctor   func() error
	SetModel func() error
	// ConfigRun handles "config show", "config get <key>", "config set <key> <val>".
	// args is the slice after "config", e.g. ["show"] or ["get", "model"].
	ConfigRun func(args []string) error
	// MemoryRun handles "memory list", "memory add <keyword> <content...>", "memory remove <keyword>".
	MemoryRun func(args []string) error
}

func Run(version string, cmds BuiltinCommands, cfg *config.Config, client llm.Client, shellInfo shell.Info) error {
	configDir, err := replConfigDir()
	if err != nil {
		return err
	}

	rl, err := replNewReadline(&readline.Config{
		Prompt:      color.New(color.FgCyan, color.Bold).Sprint("ai> "),
		HistoryFile: filepath.Join(configDir, "history"),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	fmt.Printf("AI CLI %s — interactive mode. Type 'help' for commands or 'exit' to quit.\n", version)

	systemPrompt := replBuildSystemPrompt(shellInfo.OS, shellInfo.Shell, shellInfo.Version, "")

	for {
		line, err := rl.Readline()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, readline.ErrInterrupt) {
				fmt.Println("Bye!")
				return nil
			}
			return err
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Built-in commands — handle without hitting the LLM
		switch {
		case input == "exit" || input == "quit":
			fmt.Println("Bye!")
			return nil

		case input == "help":
			printHelp()

		case input == "version":
			fmt.Printf("ai %s\n", version)

		case input == "status":
			if err := cmds.Status(); err != nil {
				color.Red("Error: %v", err)
			}

		case input == "doctor":
			if err := cmds.Doctor(); err != nil {
				color.Red("Error: %v", err)
			}

		case input == "set-model":
			if err := cmds.SetModel(); err != nil {
				color.Red("Error: %v", err)
			}

		case input == "config" || strings.HasPrefix(input, "config "):
			parts := strings.Fields(input)
			if err := cmds.ConfigRun(parts[1:]); err != nil {
				color.Red("Error: %v", err)
			}

		case input == "memory" || strings.HasPrefix(input, "memory "):
			parts := strings.Fields(input)
			if err := cmds.MemoryRun(parts[1:]); err != nil {
				color.Red("Error: %v", err)
			}

		default:
			// Inject matching memories into the prompt
			prompt := systemPrompt
			entries, memErr := memory.Load()
			if memErr == nil {
				matches := memory.FindMatching(input, entries)
				if len(matches) > 0 {
					contexts := make([]llm.MemoryContext, len(matches))
					for i, m := range matches {
						contexts[i] = llm.MemoryContext{Keyword: m.Keyword, Content: m.Content}
					}
					prompt = llm.AppendMemories(prompt, contexts)
				}
			}

			// Send to LLM
			resp, err := client.Chat(prompt, input)
			if err != nil {
				color.Red("Error: %v", err)
				continue
			}
			if err := replHandleResponse(resp, cfg, shellInfo); err != nil {
				color.Red("Error: %v", err)
			}
		}
	}
}

func printHelp() {
	fmt.Println()
	fmt.Println("Built-in commands:")
	fmt.Printf("  %-30s %s\n", "help", "Show this help message")
	fmt.Printf("  %-30s %s\n", "version", "Print the current version")
	fmt.Printf("  %-30s %s\n", "status", "Show current configuration status")
	fmt.Printf("  %-30s %s\n", "doctor", "Check and repair configuration")
	fmt.Printf("  %-30s %s\n", "set-model", "Interactively select a provider and model")
	fmt.Printf("  %-30s %s\n", "config show", "Show current configuration")
	fmt.Printf("  %-30s %s\n", "config get <key>", "Get a config value")
	fmt.Printf("  %-30s %s\n", "config set <key> <value>", "Set a config value")
	fmt.Printf("  %-30s %s\n", "memory list", "List all memories")
	fmt.Printf("  %-30s %s\n", "memory add <keyword> <content>", "Add a memory")
	fmt.Printf("  %-30s %s\n", "memory remove <keyword>", "Remove a memory")
	fmt.Printf("  %-30s %s\n", "exit / quit", "Exit interactive mode")
	fmt.Println()
	fmt.Println("Any other input is translated into shell commands by the AI.")
	fmt.Println()
}

func handleResponse(resp *llm.Response, cfg *config.Config, shellInfo shell.Info) error {
	switch resp.Type {
	case "commands":
		return executor.Run(resp.Commands, cfg, shellInfo, cfg.Explain)
	case "config":
		return applyConfig(resp, cfg)
	default:
		return fmt.Errorf("unknown response type: %s", resp.Type)
	}
}

func applyConfig(resp *llm.Response, cfg *config.Config) error {
	fmt.Printf("Config change: %s %s = %s\n", resp.Action, resp.Key, resp.Value)
	fmt.Print("Apply? [Y/n] ")
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "" && input != "y" && input != "yes" {
		fmt.Println("Skipped.")
		return nil
	}

	if err := config.ApplyAction(cfg, resp.Action, resp.Key, resp.Value); err != nil {
		return err
	}
	color.Green("Config updated successfully.")
	return nil
}

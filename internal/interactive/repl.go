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
	"github.com/kriserickson/ai-cli/internal/runner"
)

type replLineReader interface {
	Readline() (string, error)
	Close() error
}

var (
	replConfigDir   = config.Dir
	replNewReadline = func(cfg *readline.Config) (replLineReader, error) { return readline.NewEx(cfg) }
)

// BuiltinCommands holds handlers for built-in REPL commands so the interactive
// package doesn't need to import the cmd package (which would be circular).
type BuiltinCommands struct {
	Status   func() error
	Doctor   func() error
	SetModel func(level string) error
	// ModelLevel gets the current model level when level is blank, or switches
	// the current interactive session when level is provided.
	ModelLevel func(level string) (string, error)
	// ConfigRun handles "config show", "config get <key>", "config set <key> <val>".
	// args is the slice after "config", e.g. ["show"] or ["get", "model"].
	ConfigRun func(args []string) error
	// MemoryRun handles "memory list", "memory add <keyword> <content...>", "memory remove <keyword>".
	MemoryRun func(args []string) error
	// HistoryRun handles "history list", "history show <id>", "history remove <id>", "history clear".
	HistoryRun func(args []string) error
}

func Run(version string, cmds BuiltinCommands, instructionRunner runner.Interface) error {
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

		case input == "set-model" || strings.HasPrefix(input, "set-model "):
			parts := strings.Fields(input)
			if len(parts) > 2 {
				color.Red("Error: usage: set-model [light|default|high]")
				continue
			}
			level := config.ModelLevelDefault
			if len(parts) == 2 {
				level = parts[1]
			}
			if !config.ValidModelLevel(level) {
				color.Red("Error: model level must be light, default, or high")
				continue
			}
			if err := cmds.SetModel(level); err != nil {
				color.Red("Error: %v", err)
			}

		case input == "model-level" || strings.HasPrefix(input, "model-level "):
			parts := strings.Fields(input)
			if len(parts) > 2 {
				color.Red("Error: usage: model-level [light|default|high]")
				continue
			}
			level := ""
			if len(parts) == 2 {
				level = parts[1]
				if !config.ValidModelLevel(level) {
					color.Red("Error: model level must be light, default, or high")
					continue
				}
			}
			current, err := cmds.ModelLevel(level)
			if err != nil {
				color.Red("Error: %v", err)
				continue
			}
			fmt.Printf("Model level: %s\n", current)

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

		case input == "history" || strings.HasPrefix(input, "history "):
			parts := strings.Fields(input)
			if err := cmds.HistoryRun(parts[1:]); err != nil {
				color.Red("Error: %v", err)
			}

		case input == "retry" || strings.HasPrefix(input, "retry "):
			depth, err := runner.ParseRetryDepth(input)
			if err != nil {
				color.Red("Error: %v", err)
				continue
			}
			if err := instructionRunner.RetryLastFailed(depth); err != nil {
				color.Red("Error: %v", err)
			}

		default:
			if err := instructionRunner.RunInstruction(input); err != nil {
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
	fmt.Printf("  %-30s %s\n", "set-model [level]", "Configure the light, default, or high model")
	fmt.Printf("  %-30s %s\n", "model-level [level]", "Show or switch the current model level")
	fmt.Printf("  %-30s %s\n", "config show", "Show current configuration")
	fmt.Printf("  %-30s %s\n", "config get <key>", "Get a config value")
	fmt.Printf("  %-30s %s\n", "config set <key> <value>", "Set a config value")
	fmt.Printf("  %-30s %s\n", "memory list", "List all memories")
	fmt.Printf("  %-30s %s\n", "memory add <keyword> <content>", "Add a memory")
	fmt.Printf("  %-30s %s\n", "memory remove <keyword>", "Remove a memory")
	fmt.Printf("  %-30s %s\n", "history [list|show|remove|clear]", "Inspect saved AI sessions")
	fmt.Printf("  %-30s %s\n", "retry [depth]", "Retry the last failed AI command with optional history depth")
	fmt.Printf("  %-30s %s\n", "exit / quit", "Exit interactive mode")
	fmt.Println()
	fmt.Println("Any other input is translated into shell commands by the AI.")
	fmt.Println()
}

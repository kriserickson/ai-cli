package interactive

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
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
}

func Run(version string, cmds BuiltinCommands, cfg *config.Config, client llm.Client, shellInfo shell.Info) error {
	configDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:      color.New(color.FgCyan, color.Bold).Sprint("ai> "),
		HistoryFile: filepath.Join(configDir, "history"),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	fmt.Printf("AI CLI %s — interactive mode. Type 'help' for commands or 'exit' to quit.\n", version)

	systemPrompt := llm.BuildSystemPrompt(shellInfo.OS, shellInfo.Shell, shellInfo.Version, "")

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF || err == readline.ErrInterrupt {
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

		default:
			// Send to LLM
			resp, err := client.Chat(systemPrompt, input)
			if err != nil {
				color.Red("Error: %v", err)
				continue
			}
			if err := handleResponse(resp, cfg, shellInfo); err != nil {
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
	fmt.Printf("  %-30s %s\n", "exit / quit", "Exit interactive mode")
	fmt.Println()
	fmt.Println("Any other input is translated into shell commands by the AI.")
	fmt.Println()
}

func handleResponse(resp *llm.Response, cfg *config.Config, shellInfo shell.Info) error {
	switch resp.Type {
	case "commands":
		return executor.Run(resp.Commands, cfg, shellInfo)
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
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "" && input != "y" && input != "yes" {
		fmt.Println("Skipped.")
		return nil
	}

	return applyConfigChange(resp.Action, resp.Key, resp.Value, cfg)
}

func applyConfigChange(action, key, value string, cfg *config.Config) error {
	switch action {
	case "set_model":
		cfg.Provider.Model = value
	case "set_provider":
		cfg.Provider.Default = value
	case "set_key":
		switch key {
		case "openai_key":
			cfg.Provider.OpenAI.APIKey = value
		case "openrouter_key":
			cfg.Provider.OpenRouter.APIKey = value
		default:
			return fmt.Errorf("unknown key: %s", key)
		}
	case "set_safety":
		switch key {
		case "always_confirm":
			cfg.Safety.AlwaysConfirm = value == "true"
		case "min_certainty":
			var n int
			fmt.Sscanf(value, "%d", &n)
			cfg.Safety.MinCertainty = n
		default:
			return fmt.Errorf("unknown safety key: %s", key)
		}
	default:
		return fmt.Errorf("unknown config action: %s", action)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	color.Green("Config updated successfully.")
	return nil
}

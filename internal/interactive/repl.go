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

func Run(cfg *config.Config, client llm.Client, shellInfo shell.Info) error {
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

	fmt.Println("AI CLI interactive mode. Type 'exit' or 'quit' to leave.")

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
		if input == "exit" || input == "quit" {
			fmt.Println("Bye!")
			return nil
		}

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

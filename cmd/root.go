package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/interactive"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
	"github.com/spf13/cobra"
)

var debugFlag string

var rootCmd = &cobra.Command{
	Use:                "ai [instruction]",
	Short:              "Translate natural language into shell commands using AI",
	Long:               "AI CLI translates natural language instructions into shell commands using LLMs (OpenAI/OpenRouter).",
	RunE:               runRoot,
	DisableFlagParsing: false,
	Args:               cobra.ArbitraryArgs,
	// Treat unknown first args as instruction text, not subcommand errors
	SilenceErrors: true,
}

func init() {
	rootCmd.Flags().StringVar(&debugFlag, "debug", "", "Debug mode: screen (default) or file (overrides config)")
	// When --debug is given without a value, default to "screen"
	rootCmd.Flags().Lookup("debug").NoOptDefVal = "screen"
	// Allow flags to be interspersed with args
	rootCmd.Flags().SetInterspersed(true)
}

func Execute() {
	// Use TraverseChildren so only known subcommands (config, version) are
	// routed to child commands; everything else falls through to root.
	rootCmd.TraverseChildren = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// --debug flag overrides config; if not set, use config value
	debugMode := cfg.Debug
	if debugFlag != "" {
		debugMode = debugFlag
	}

	debugOut, closeDebug, err := llm.DebugWriter(debugMode)
	if err != nil {
		return err
	}
	defer closeDebug()

	shellInfo := shell.Detect()

	// No args: interactive mode
	if len(args) == 0 {
		client, err := llm.NewClient(cfg, debugOut)
		if err != nil {
			return err
		}
		return interactive.Run(cfg, client, shellInfo)
	}

	// Single-shot mode
	instruction := strings.Join(args, " ")

	client, err := llm.NewClient(cfg, debugOut)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	systemPrompt := llm.BuildSystemPrompt(shellInfo.OS, shellInfo.Shell, shellInfo.Version, cwd)

	resp, err := client.Chat(systemPrompt, instruction)
	if err != nil {
		return err
	}

	switch resp.Type {
	case "commands":
		return executor.Run(resp.Commands, cfg, shellInfo)
	case "config":
		return handleConfig(resp, cfg)
	default:
		return fmt.Errorf("unexpected response type: %s", resp.Type)
	}
}

func handleConfig(resp *llm.Response, cfg *config.Config) error {
	fmt.Printf("Config change: %s %s = %s\n", resp.Action, resp.Key, resp.Value)
	fmt.Print("Apply? [Y/n] ")
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "" && input != "y" && input != "yes" {
		fmt.Println("Skipped.")
		return nil
	}

	switch resp.Action {
	case "set_model":
		cfg.Provider.Model = resp.Value
	case "set_provider":
		cfg.Provider.Default = resp.Value
	case "set_key":
		switch resp.Key {
		case "openai_key":
			cfg.Provider.OpenAI.APIKey = resp.Value
		case "openrouter_key":
			cfg.Provider.OpenRouter.APIKey = resp.Value
		default:
			return fmt.Errorf("unknown key: %s", resp.Key)
		}
	case "set_safety":
		switch resp.Key {
		case "always_confirm":
			cfg.Safety.AlwaysConfirm = resp.Value == "true"
		case "min_certainty":
			var n int
			fmt.Sscanf(resp.Value, "%d", &n)
			cfg.Safety.MinCertainty = n
		}
	default:
		return fmt.Errorf("unknown config action: %s", resp.Action)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	color.Green("Config updated successfully.")
	return nil
}

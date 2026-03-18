package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/history"
	"github.com/kriserickson/ai-cli/internal/interactive"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/runner"
	"github.com/kriserickson/ai-cli/internal/shell"
)

const windows = "windows"

var (
	debugFlag        string
	retryOnErrorFlag bool
	retryDepthFlag   int
	explainFlag      bool
)

// interactiveRun is the entry point for interactive mode, stubbable for testing.
var interactiveRun = interactive.Run

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
	rootCmd.Flags().BoolVar(&retryOnErrorFlag, "retry-on-error", false, "Automatically send failed commands back to the AI for retry (uses history.retry_max_attempts for attempt limit)")
	rootCmd.Flags().IntVar(&retryDepthFlag, "retry-depth", 0, "Override how many recent command results are included in AI retry context")
	// When --debug is given without a value, default to config.DebugScreen
	rootCmd.Flags().Lookup("debug").NoOptDefVal = config.DebugScreen
	rootCmd.Flags().BoolVar(&explainFlag, "explain", false, "Show detailed explanation of each command")
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

func runRoot(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// --debug flag overrides config; if not set, use config value
	debugMode := cfg.Debug
	if debugFlag != "" {
		debugMode = debugFlag
	}
	if retryOnErrorFlag {
		cfg.History.AutoCheckOnError = true
	}
	if retryDepthFlag > 0 {
		cfg.History.RetryContextDepth = retryDepthFlag
	}

	debugOut, closeDebug, err := llm.DebugWriter(debugMode)
	if err != nil {
		return err
	}
	defer closeDebug()

	shellInfo := shell.Detect()

	// No args: interactive mode
	if len(args) == 0 {
		client, err := llm.NewClient(cfg, debugOut, debugMode)
		if err != nil {
			return err
		}
		instructionRunner := runner.New(cfg, client, shellInfo, runner.WithExplain(explainFlag), runner.WithInteractive())
		cmds := interactive.BuiltinCommands{
			Status: func() error {
				return runStatus(nil, nil)
			},
			Doctor: func() error {
				return runDoctor(nil, nil)
			},
			SetModel: func() error {
				return runSetModel(nil, nil)
			},
			ConfigRun: func(args []string) error {
				if len(args) == 0 {
					return errors.New("config requires a subcommand: show, get <key>, set <key> <value>")
				}
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				switch args[0] {
				case "show":
					data, err := toml.Marshal(config.RedactedCopy(cfg))
					if err != nil {
						return err
					}
					fmt.Print(string(data))
				case "get":
					if len(args) < 2 {
						return errors.New("config get requires a key argument")
					}
					val, err := getConfigValue(cfg, args[1])
					if err != nil {
						return err
					}
					fmt.Println(val)
				case "set":
					if len(args) < 3 {
						return errors.New("config set requires key and value arguments")
					}
					if err := setConfigValue(cfg, args[1], args[2]); err != nil {
						return err
					}
					if err := config.Save(cfg); err != nil {
						return fmt.Errorf("failed to save config: %w", err)
					}
					color.Green("Config updated successfully.")
				default:
					return fmt.Errorf("unknown config subcommand: %s\nUsage: config show | config get <key> | config set <key> <value>", args[0])
				}
				return nil
			},
			MemoryRun: func(args []string) error {
				if len(args) == 0 {
					return listMemories()
				}
				switch args[0] {
				case "list":
					return listMemories()
				case "add":
					if len(args) < 3 {
						return errors.New("usage: memory add <keyword> <content...>")
					}
					keyword := args[1]
					content := strings.Join(args[2:], " ")
					if err := memory.Add(keyword, content); err != nil {
						return err
					}
					fmt.Printf("Memory %q added.\n", keyword)
				case "remove":
					if len(args) < 2 {
						return errors.New("usage: memory remove <keyword>")
					}
					if err := memory.Remove(args[1]); err != nil {
						return err
					}
					fmt.Printf("Memory %q removed.\n", args[1])
				default:
					return fmt.Errorf("unknown memory subcommand: %s\nUsage: memory list | memory add <keyword> <content...> | memory remove <keyword>", args[0])
				}
				return nil
			},
			HistoryRun: func(args []string) error {
				if len(args) == 0 {
					return listHistory(historyVerbose, historyCount)
				}
				switch args[0] {
				case "list":
					return listHistory(historyVerbose, historyCount)
				case "show":
					if len(args) < 2 {
						return errors.New("history show requires an id argument")
					}
					return showHistory(args[1])
				case "remove":
					if len(args) < 2 {
						return errors.New("history remove requires an id argument")
					}
					if err := history.Remove(args[1]); err != nil {
						return err
					}
					fmt.Printf("History session %q removed.\n", args[1])
					return nil
				case "clear":
					if err := history.Clear(); err != nil {
						return err
					}
					fmt.Println("History cleared.")
					return nil
				default:
					return fmt.Errorf("unknown history subcommand: %s\nUsage: history list | history show <id> | history remove <id> | history clear", args[0])
				}
			},
		}
		return interactiveRun(Version, cmds, instructionRunner)
	}

	// Single-shot mode
	instruction := strings.Join(args, " ")

	client, err := llm.NewClient(cfg, debugOut, debugMode)
	if err != nil {
		return err
	}
	return runner.New(cfg, client, shellInfo, runner.WithExplain(explainFlag)).RunInstruction(instruction)
}

func handleConfig(resp *llm.Response, cfg *config.Config) error {
	fmt.Printf("Config change: %s %s = %s\n", resp.Action, resp.Key, config.DisplayValue(resp.Action, resp.Key, resp.Value))
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

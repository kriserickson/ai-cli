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
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/interactive"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
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
	// When --debug is given without a value, default to config.DebugScreen
	rootCmd.Flags().Lookup("debug").NoOptDefVal = config.DebugScreen
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
					data, err := toml.Marshal(cfg)
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
		}
		return interactive.Run(Version, cmds, cfg, client, shellInfo)
	}

	// Single-shot mode
	instruction := strings.Join(args, " ")

	client, err := llm.NewClient(cfg, debugOut)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	systemPrompt := llm.BuildSystemPrompt(shellInfo.OS, shellInfo.Shell, shellInfo.Version, cwd)

	// Inject matching memories into the system prompt
	entries, err := memory.Load()
	if err == nil {
		matches := memory.FindMatching(instruction, entries)
		if len(matches) > 0 {
			contexts := make([]llm.MemoryContext, len(matches))
			for i, m := range matches {
				contexts[i] = llm.MemoryContext{Keyword: m.Keyword, Content: m.Content}
			}
			systemPrompt = llm.AppendMemories(systemPrompt, contexts)
		}
	}

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

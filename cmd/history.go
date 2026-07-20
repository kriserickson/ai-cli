package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/history"
)

const responseCommands = "commands"

var (
	historyVerbose bool
	historyCount   int
)

func init() {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Inspect AI CLI session history",
		RunE: func(_ *cobra.Command, _ []string) error {
			return listHistory(historyVerbose, historyCount)
		},
	}
	historyCmd.PersistentFlags().BoolVar(&historyVerbose, "verbose", false, "show additional history details")
	historyCmd.PersistentFlags().IntVar(&historyCount, "count", 10, "number of history sessions to list")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recent AI sessions",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return listHistory(historyVerbose, historyCount)
		},
	}
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a saved AI session",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return showHistory(args[0])
		},
	}
	removeCmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a saved AI session",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := history.Remove(args[0]); err != nil {
				return err
			}
			fmt.Printf("History session %q removed.\n", args[0])
			return nil
		},
	}
	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove all saved AI sessions",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := history.Clear(); err != nil {
				return err
			}
			fmt.Println("History cleared.")
			return nil
		},
	}

	historyCmd.AddCommand(listCmd, showCmd, removeCmd, clearCmd)

	rootCmd.AddCommand(historyCmd)
}

func listHistory(verbose bool, count int) error {
	sessions, err := history.List()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Println("No AI history stored.")
		return nil
	}

	if count <= 0 {
		return errors.New("history count must be greater than 0")
	}
	if len(sessions) > count {
		sessions = sessions[:count]
	}

	for i := range sessions {
		session := &sessions[i]
		fmt.Printf("%s : %s retries=%d model=%s\n  prompt: %s\n  command: %s\n",
			session.UpdatedAt.Local().Format(time.DateTime),
			session.Status,
			session.RetryCount,
			session.Model,
			strings.TrimSpace(session.Instruction),
			strings.TrimSpace(historyListCommand(session)),
		)
		if verbose {
			printVerboseHistory(session)
		}
	}
	return nil
}

func showHistory(id string) error {
	session, err := history.Load(id)
	if err != nil {
		return err
	}

	fmt.Printf("ID:          %s\n", session.ID)
	fmt.Printf("Status:      %s\n", session.Status)
	fmt.Printf("Retries:     %d\n", session.RetryCount)
	fmt.Printf("Provider:    %s\n", session.Provider)
	fmt.Printf("Model:       %s\n", session.Model)
	fmt.Printf("Shell:       %s (%s)\n", session.Shell, session.ShellVersion)
	fmt.Printf("Directory:   %s\n", session.WorkingDirectory)
	fmt.Printf("Created:     %s\n", session.CreatedAt.Local().Format(time.DateTime))
	fmt.Printf("Updated:     %s\n", session.UpdatedAt.Local().Format(time.DateTime))
	fmt.Printf("Instruction: %s\n", session.Instruction)

	if len(session.Exchanges) > 0 {
		fmt.Println()
		fmt.Println("Exchanges:")
		for _, exchange := range session.Exchanges {
			fmt.Printf("  - attempt %d (%s)\n", exchange.Attempt, exchange.Kind)
			if exchange.Response != nil && exchange.Response.Explanation != "" {
				fmt.Printf("    explanation: %s\n", exchange.Response.Explanation)
			}
			if exchange.Error != "" {
				fmt.Printf("    error: %s\n", exchange.Error)
			}
			if exchange.Response != nil {
				switch exchange.Response.Type {
				case responseCommands:
					for _, command := range exchange.Response.Commands {
						fmt.Printf("    command: %s\n", command.Command)
					}
				case "config":
					fmt.Printf("    config: %s %s = %s\n", exchange.Response.Action, exchange.Response.Key, config.DisplayValue(exchange.Response.Action, exchange.Response.Key, exchange.Response.Value))
				}
			}
		}
	}

	if len(session.Executions) > 0 {
		fmt.Println()
		fmt.Println("Executions:")
		for i := range session.Executions {
			execution := &session.Executions[i]
			fmt.Printf("  - attempt %d step %d exit=%d skipped=%t\n", execution.Attempt, execution.Index+1, execution.ExitCode, execution.Skipped)
			fmt.Printf("    %s\n", execution.Command)
			if execution.Stdout != "" {
				fmt.Printf("    stdout: %s\n", oneLineHistory(execution.Stdout))
			}
			if execution.Stderr != "" {
				fmt.Printf("    stderr: %s\n", oneLineHistory(execution.Stderr))
			}
			if execution.Error != "" {
				fmt.Printf("    error: %s\n", execution.Error)
			}
		}
	}

	return nil
}

func truncateHistoryLine(input string, limit int) string {
	input = strings.TrimSpace(strings.ReplaceAll(input, "\n", " "))
	if len(input) <= limit {
		return input
	}
	if limit <= 3 {
		return input[:limit]
	}
	return input[:limit-3] + "..."
}

func oneLineHistory(input string) string {
	return truncateHistoryLine(strings.Join(strings.Fields(input), " "), 96)
}

func printVerboseHistory(session *history.Session) {
	fmt.Printf("  id: %s\n", session.ID)
	fmt.Printf("  provider: %s\n", session.Provider)
	fmt.Printf("  shell: %s (%s)\n", session.Shell, session.ShellVersion)
	if session.WorkingDirectory != "" {
		fmt.Printf("  directory: %s\n", session.WorkingDirectory)
	}
	fmt.Printf("  created: %s\n", session.CreatedAt.Local().Format(time.DateTime))

	if exchange := historyListExchange(session); exchange != nil {
		fmt.Printf("  exchange: attempt=%d kind=%s\n", exchange.Attempt, exchange.Kind)
		if exchange.Response != nil && exchange.Response.Explanation != "" {
			fmt.Printf("  explanation: %s\n", strings.TrimSpace(exchange.Response.Explanation))
		}
		if exchange.Error != "" {
			fmt.Printf("  llm_error: %s\n", oneLineHistory(exchange.Error))
		}
	}

	if execution := historyListExecution(session); execution != nil {
		fmt.Printf("  result: exit=%d skipped=%t confirmed=%t\n", execution.ExitCode, execution.Skipped, execution.Confirmed)
		if execution.Stdout != "" {
			fmt.Printf("  stdout: %s\n", oneLineHistory(execution.Stdout))
		}
		if execution.Stderr != "" {
			fmt.Printf("  stderr: %s\n", oneLineHistory(execution.Stderr))
		}
		if execution.Error != "" {
			fmt.Printf("  exec_error: %s\n", oneLineHistory(execution.Error))
		}
	}
}

func historyListCommand(session *history.Session) string {
	if execution := historyListExecution(session); execution != nil {
		return execution.Command
	}

	if exchange := historyListExchange(session); exchange != nil {
		if exchange.Response == nil || len(exchange.Response.Commands) == 0 {
			return ""
		}
		return exchange.Response.Commands[len(exchange.Response.Commands)-1].Command
	}

	return ""
}

func historyListExecution(session *history.Session) *history.CommandAttempt {
	if len(session.Executions) == 0 {
		return nil
	}
	return &session.Executions[len(session.Executions)-1]
}

func historyListExchange(session *history.Session) *history.Exchange {
	for i := len(session.Exchanges) - 1; i >= 0; i-- {
		exchange := &session.Exchanges[i]
		if exchange.Response == nil || exchange.Response.Type != responseCommands || len(exchange.Response.Commands) == 0 {
			if exchange.Error == "" {
				continue
			}
			return exchange
		}
		return exchange
	}
	return nil
}

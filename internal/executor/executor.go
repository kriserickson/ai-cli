package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

var (
	cmdColor  = color.New(color.FgCyan, color.Bold)
	descColor = color.New(color.FgWhite)
	safeColor = color.New(color.FgGreen)
	riskColor = color.New(color.FgRed)
	dimColor  = color.New(color.Faint)
)

// Run executes a list of commands sequentially, prompting for confirmation as needed.
func Run(commands []llm.Command, cfg *config.Config, shellInfo shell.Info, explain bool) error {
	for i, cmd := range commands {
		if len(commands) > 1 {
			dimColor.Printf("\n[%d/%d] ", i+1, len(commands))
		} else {
			fmt.Println()
		}

		descColor.Println(cmd.Description)
		cmdColor.Printf("  $ %s", cmd.Command)

		// Show certainty and risk
		fmt.Print("  ")
		if cmd.Risk == "risky" {
			riskColor.Printf("[risky]")
		} else {
			safeColor.Printf("[safe]")
		}
		dimColor.Printf(" %d%% certainty\n", cmd.Certainty)

		if explain && cmd.Explanation != "" {
			dimColor.Fprintf(os.Stdout, "  💡 %s\n", cmd.Explanation)
		}

		if ShouldConfirm(cmd, cfg) {
			if !askConfirmation() {
				fmt.Println("Skipped.")
				continue
			}
		}

		if err := execute(cmd.Command, shellInfo); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
	}
	return nil
}

func execute(command string, shellInfo shell.Info) error {
	shellBin, args := shell.Command(shellInfo.Shell)
	args = append(args, command)

	cmd := exec.CommandContext(context.Background(), shellBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func askConfirmation() bool {
	fmt.Print("Execute? [Y/n] ")
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

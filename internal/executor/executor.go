package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

const maxCapturedOutputBytes = 32 * 1024

var (
	cmdColor  = color.New(color.FgCyan, color.Bold)
	descColor = color.New(color.FgWhite)
	safeColor = color.New(color.FgGreen)
	riskColor = color.New(color.FgRed)
	dimColor  = color.New(color.Faint)
)

type CommandResult struct {
	Index     int
	Command   llm.Command
	Confirmed bool
	Skipped   bool
	ExitCode  int
	Stdout    string
	Stderr    string
	Error     string
	StartedAt time.Time
	Duration  time.Duration
}

type RunResult struct {
	Commands []CommandResult
}

func (r *RunResult) FirstFailure() *CommandResult {
	for i := range r.Commands {
		if r.Commands[i].ExitCode != 0 && !r.Commands[i].Skipped {
			return &r.Commands[i]
		}
		if r.Commands[i].Error != "" && !r.Commands[i].Skipped {
			return &r.Commands[i]
		}
	}
	return nil
}

// Run executes a list of commands sequentially, prompting for confirmation as needed.
func Run(commands []llm.Command, cfg *config.Config, shellInfo shell.Info) error {
	result, err := RunWithResults(commands, cfg, shellInfo)
	if err != nil {
		return err
	}
	if result.FirstFailure() != nil {
		return fmt.Errorf("command failed: exit code %d", result.FirstFailure().ExitCode)
	}
	return nil
}

func RunWithResults(commands []llm.Command, cfg *config.Config, shellInfo shell.Info) (*RunResult, error) {
	runResult := &RunResult{Commands: make([]CommandResult, 0, len(commands))}

	for i, command := range commands {
		if len(commands) > 1 {
			dimColor.Printf("\n[%d/%d] ", i+1, len(commands))
		} else {
			fmt.Println()
		}

		descColor.Println(command.Description)
		cmdColor.Printf("  $ %s", command.Command)

		fmt.Print("  ")
		if command.Risk == "risky" {
			riskColor.Printf("[risky]")
		} else {
			safeColor.Printf("[safe]")
		}
		dimColor.Printf(" %d%% certainty\n", command.Certainty)

		result := CommandResult{
			Index:   i,
			Command: command,
		}

		if ShouldConfirm(command, cfg) {
			result.Confirmed = askConfirmation()
			if !result.Confirmed {
				result.Skipped = true
				runResult.Commands = append(runResult.Commands, result)
				fmt.Println("Skipped.")
				continue
			}
		} else {
			result.Confirmed = true
		}

		execResult := execute(command.Command, shellInfo)
		result.ExitCode = execResult.ExitCode
		result.Stdout = execResult.Stdout
		result.Stderr = execResult.Stderr
		result.Error = execResult.Error
		result.StartedAt = execResult.StartedAt
		result.Duration = execResult.Duration
		runResult.Commands = append(runResult.Commands, result)
		if execResult.Err != nil {
			return runResult, fmt.Errorf("command failed: %w", execResult.Err)
		}
	}

	return runResult, nil
}

type executionResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Error     string
	StartedAt time.Time
	Duration  time.Duration
	Err       error
}

func execute(command string, shellInfo shell.Info) executionResult {
	startedAt := time.Now()
	shellBin, args := shell.Command(shellInfo.Shell)
	args = append(args, command)

	cmd := exec.CommandContext(context.Background(), shellBin, args...)

	stdoutCapture := &limitedBuffer{limit: maxCapturedOutputBytes}
	stderrCapture := &limitedBuffer{limit: maxCapturedOutputBytes}
	cmd.Stdout = io.MultiWriter(os.Stdout, stdoutCapture)
	cmd.Stderr = io.MultiWriter(os.Stderr, stderrCapture)
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	result := executionResult{
		ExitCode:  0,
		Stdout:    stdoutCapture.String(),
		Stderr:    stderrCapture.String(),
		StartedAt: startedAt,
		Duration:  time.Since(startedAt),
		Err:       err,
	}

	if err == nil {
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}
	result.Error = err.Error()
	return result
}

func askConfirmation() bool {
	fmt.Print("Execute? [Y/n] ")
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = b.buf.Write(p[:remaining])
			b.truncated = true
		} else {
			_, _ = b.buf.Write(p)
		}
	} else {
		b.truncated = true
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	if !b.truncated {
		return b.buf.String()
	}
	return b.buf.String() + "\n[output truncated]\n"
}

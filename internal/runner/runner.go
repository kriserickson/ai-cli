package runner

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/history"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
	"github.com/kriserickson/ai-cli/internal/tools"
)

var runnerStdinIsTTY = stdinIsTTY

type Interface interface {
	RunInstruction(instruction string) error
	RetryLastFailed(depth int) error
}

type Runner struct {
	cfg        *config.Config
	client     llm.Client
	shellInfo  shell.Info
	explain    bool
	lastFailed *history.Session
}

type Option func(*Runner)

func WithExplain(explain bool) Option {
	return func(r *Runner) {
		r.explain = explain
	}
}

func New(cfg *config.Config, client llm.Client, shellInfo shell.Info, opts ...Option) *Runner {
	r := &Runner{
		cfg:       cfg,
		client:    client,
		shellInfo: shellInfo,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Runner) RunInstruction(instruction string) error {
	cwd, _ := os.Getwd()
	systemPrompt := llm.BuildSystemPrompt(r.shellInfo.OS, r.shellInfo.Shell, r.shellInfo.Version, cwd, r.explain, r.toolsEnabled())
	systemPrompt = r.appendMemories(systemPrompt, instruction)

	session := history.NewSession(instruction, cwd, r.cfg, r.shellInfo)
	return r.runSession(session, systemPrompt, instruction, "initial", false, r.cfg.History.RetryContextDepth)
}

func (r *Runner) RetryLastFailed(depth int) error {
	if r.lastFailed == nil {
		return errors.New("no failed AI session to retry")
	}
	if depth <= 0 {
		depth = r.cfg.History.RetryContextDepth
	}

	cwd := r.lastFailed.WorkingDirectory
	systemPrompt := llm.BuildSystemPrompt(r.shellInfo.OS, r.shellInfo.Shell, r.shellInfo.Version, cwd, r.explain, r.toolsEnabled())
	return r.retrySession(r.lastFailed, systemPrompt, depth, false)
}

func ParseRetryDepth(input string) (int, error) {
	fields := strings.Fields(input)
	if len(fields) <= 1 {
		return 0, nil
	}
	depth, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("retry depth must be a number: %w", err)
	}
	if depth <= 0 {
		return 0, errors.New("retry depth must be greater than 0")
	}
	return depth, nil
}

func (r *Runner) runSession(session *history.Session, systemPrompt, userMessage, kind string, autoRetry bool, depth int) error {
	attempt := session.RetryCount + 1

	var (
		chatResult *llm.ChatResult
		err        error
	)
	if r.toolsEnabled() {
		var resp *llm.Response
		resp, err = tools.RunWithTools(r.client, systemPrompt, userMessage, r.cfg, r.shellInfo, 3)
		if resp != nil {
			chatResult = &llm.ChatResult{Response: resp}
		}
	} else {
		chatResult, err = r.client.ChatWithTrace(systemPrompt, userMessage)
	}
	session.RecordExchange(kind, attempt, systemPrompt, userMessage, chatResult, err, r.cfg.History)
	r.saveSession(session)
	if err != nil {
		session.MarkStatus("failed")
		r.lastFailed = session
		r.saveSession(session)
		return err
	}

	resp := chatResult.Response
	if resp.Explanation != "" {
		color.New(color.Faint).Printf("%s\n", resp.Explanation)
	}

	switch resp.Type {
	case "commands":
		runResult, runErr := executor.RunWithResults(resp.Commands, r.cfg, r.shellInfo, r.explain)
		if runResult != nil {
			session.RecordExecutions(attempt, runResult.Commands)
		}
		if runErr == nil {
			session.MarkStatus("completed")
			r.lastFailed = nil
			r.saveSession(session)
			return nil
		}

		session.MarkStatus("failed")
		r.saveSession(session)

		if r.shouldRetry(session, autoRetry) {
			return r.retrySession(session, systemPrompt, depth, true)
		}

		r.lastFailed = session
		return runErr
	case "config":
		if err := applyConfig(resp, r.cfg); err != nil {
			session.MarkStatus("failed")
			r.lastFailed = session
			r.saveSession(session)
			return err
		}
		session.MarkStatus("completed")
		r.lastFailed = nil
		r.saveSession(session)
		return nil
	default:
		session.MarkStatus("failed")
		r.lastFailed = session
		r.saveSession(session)
		return fmt.Errorf("unexpected response type: %s", resp.Type)
	}
}

func (r *Runner) retrySession(session *history.Session, systemPrompt string, depth int, auto bool) error {
	if auto && r.cfg.History.RetryMaxAttempts > 0 && session.RetryCount >= r.cfg.History.RetryMaxAttempts {
		r.lastFailed = session
		return fmt.Errorf("command failed after %d AI retries", session.RetryCount)
	}

	session.RetryCount++
	message := history.BuildRetryMessage(session, depth)
	err := r.runSession(session, systemPrompt, message, "retry", auto, depth)
	if err != nil {
		r.lastFailed = session
		return err
	}
	return nil
}

func (r *Runner) shouldRetry(session *history.Session, autoTriggered bool) bool {
	if r.cfg.History.AutoCheckOnError {
		if r.cfg.History.RetryMaxAttempts <= 0 || session.RetryCount < r.cfg.History.RetryMaxAttempts {
			return true
		}
	}

	if !r.cfg.History.AskOnError {
		return false
	}

	if !runnerStdinIsTTY() {
		fmt.Fprintln(os.Stderr, "Command failed. Re-run `retry` in interactive mode or enable history.auto_check_on_error to send the failure back to the AI.")
		return false
	}

	if autoTriggered && r.cfg.History.RetryMaxAttempts > 0 && session.RetryCount >= r.cfg.History.RetryMaxAttempts {
		return false
	}

	fmt.Print("Ask AI to debug and retry this failed command? [Y/n] ")
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

func (r *Runner) appendMemories(systemPrompt, instruction string) string {
	entries, err := memory.Load()
	if err != nil {
		return systemPrompt
	}
	matches := memory.FindMatching(instruction, entries)
	if len(matches) == 0 {
		return systemPrompt
	}

	contexts := make([]llm.MemoryContext, len(matches))
	for i, m := range matches {
		contexts[i] = llm.MemoryContext{Keyword: m.Keyword, Content: m.Content}
	}
	return llm.AppendMemories(systemPrompt, contexts)
}

func (r *Runner) saveSession(session *history.Session) {
	_ = history.Save(session)
}

func (r *Runner) toolsEnabled() bool {
	return r.cfg.Safety.ToolCalling != config.ToolCallingNever
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

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

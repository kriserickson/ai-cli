package runner

import (
	"bytes"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/history"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
)

type scriptedClient struct {
	results  []*llm.ChatResult
	errs     []error
	messages []string
}

func (c *scriptedClient) Chat(systemPrompt, userMessage string) (*llm.Response, error) {
	result, err := c.ChatWithTrace(systemPrompt, userMessage)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (c *scriptedClient) ChatWithTrace(_, userMessage string) (*llm.ChatResult, error) {
	c.messages = append(c.messages, userMessage)
	idx := len(c.messages) - 1
	var err error
	if idx < len(c.errs) {
		err = c.errs[idx]
	}
	var result *llm.ChatResult
	if idx < len(c.results) {
		result = c.results[idx]
	}
	return result, err
}

func testShellInfo() shell.Info {
	if runtime.GOOS == "windows" {
		return shell.Info{OS: "windows/amd64", Shell: "cmd", Version: "unknown"}
	}
	return shell.Info{OS: runtime.GOOS + "/" + runtime.GOARCH, Shell: "/bin/sh", Version: "unknown"}
}

const successCommand = "echo runner-ok"

func failureCommand() string {
	if runtime.GOOS == "windows" {
		return "exit 1"
	}
	return "false"
}

func withRunnerStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("WriteString(): %v", err)
	}
	_ = w.Close()

	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	fn()
}

func captureRunnerOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)
	_ = stdoutR.Close()
	_ = stderrR.Close()
	return stdoutBuf.String(), stderrBuf.String()
}

func withTTYStub(t *testing.T, isTTY bool) {
	t.Helper()
	old := runnerStdinIsTTY
	runnerStdinIsTTY = func() bool { return isTTY }
	t.Cleanup(func() { runnerStdinIsTTY = old })
}

func TestParseRetryDepth(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{input: "retry", want: 0},
		{input: "retry 3", want: 3},
		{input: "retry nope", wantErr: true},
		{input: "retry 0", wantErr: true},
	}

	for _, tt := range tests {
		got, err := ParseRetryDepth(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ParseRetryDepth(%q) error = nil, want error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseRetryDepth(%q) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseRetryDepth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRetryLastFailedNoSession(t *testing.T) {
	r := New(config.DefaultConfig(), &scriptedClient{}, testShellInfo())
	if err := r.RetryLastFailed(0); err == nil {
		t.Fatal("RetryLastFailed() error = nil, want error")
	}
}

func TestRunInstructionAutoRetriesAndSavesHistory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	cfg.History.AutoCheckOnError = true
	cfg.History.AskOnError = false
	cfg.History.IncludeDebug = true

	client := &scriptedClient{
		results: []*llm.ChatResult{
			{
				Response: &llm.Response{
					Type: "commands",
					Commands: []llm.Command{
						{Command: failureCommand(), Description: "fail once", Risk: "safe", Certainty: 100},
					},
				},
				Trace: llm.ChatTrace{RequestBody: `{"attempt":1}`, ResponseBody: `{"failed":true}`},
			},
			{
				Response: &llm.Response{
					Type:        "commands",
					Explanation: "Using a safe follow-up command after the failure.",
					Commands: []llm.Command{
						{Command: successCommand, Description: "recover", Risk: "safe", Certainty: 100},
					},
				},
				Trace: llm.ChatTrace{RequestBody: `{"attempt":2}`, ResponseBody: `{"failed":false}`},
			},
		},
	}

	r := New(cfg, client, testShellInfo())
	if err := r.RunInstruction("recover from the failed command"); err != nil {
		t.Fatalf("RunInstruction() error = %v, want nil", err)
	}
	if len(client.messages) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.messages))
	}
	if !strings.Contains(client.messages[1], "Recent command execution history:") {
		t.Fatalf("retry prompt missing execution history:\n%s", client.messages[1])
	}
	if !strings.Contains(client.messages[1], failureCommand()) {
		t.Fatalf("retry prompt missing failed command:\n%s", client.messages[1])
	}

	session := loadOnlySession(t)
	if session.RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", session.RetryCount)
	}
	if session.Status != "completed" {
		t.Fatalf("Status = %q, want completed", session.Status)
	}
	if len(session.Exchanges) != 2 {
		t.Fatalf("Exchanges = %d, want 2", len(session.Exchanges))
	}
	if session.Exchanges[0].Trace == nil {
		t.Fatal("expected trace to be stored when history.include_debug is true")
	}
	if len(session.Executions) != 2 {
		t.Fatalf("Executions = %d, want 2", len(session.Executions))
	}
	if session.Executions[0].ExitCode == 0 {
		t.Fatal("first execution should record a failure")
	}
}

func TestRetryLastFailedUsesSavedFailureContext(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	cfg.History.AutoCheckOnError = false
	cfg.History.AskOnError = false

	client := &scriptedClient{
		results: []*llm.ChatResult{
			{
				Response: &llm.Response{
					Type: "commands",
					Commands: []llm.Command{
						{Command: failureCommand(), Description: "fail once", Risk: "safe", Certainty: 100},
					},
				},
			},
			{
				Response: &llm.Response{
					Type: "commands",
					Commands: []llm.Command{
						{Command: successCommand, Description: "recover", Risk: "safe", Certainty: 100},
					},
				},
			},
		},
	}

	r := New(cfg, client, testShellInfo())
	if err := r.RunInstruction("make this succeed"); err == nil {
		t.Fatal("RunInstruction() error = nil, want command failure")
	}
	if err := r.RetryLastFailed(2); err != nil {
		t.Fatalf("RetryLastFailed() error = %v, want nil", err)
	}
	if len(client.messages) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.messages))
	}
	if !strings.Contains(client.messages[1], "Original user request:") {
		t.Fatalf("manual retry prompt missing original request:\n%s", client.messages[1])
	}
}

func TestRunInstructionClientErrorAndUnknownType(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	r := New(config.DefaultConfig(), &scriptedClient{errs: []error{errors.New("boom")}}, testShellInfo())
	if err := r.RunInstruction("explode"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("RunInstruction(client error) = %v, want boom", err)
	}
	if r.lastFailed == nil {
		t.Fatal("lastFailed should be set after client error")
	}

	r = New(config.DefaultConfig(), &scriptedClient{
		results: []*llm.ChatResult{{Response: &llm.Response{Type: "mystery"}}},
	}, testShellInfo())
	if err := r.RunInstruction("mystery"); err == nil || !strings.Contains(err.Error(), "unexpected response type") {
		t.Fatalf("RunInstruction(unknown type) = %v", err)
	}
}

func TestRunInstructionConfigPaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	r := New(cfg, &scriptedClient{
		results: []*llm.ChatResult{{Response: &llm.Response{Type: "config", Action: "set_model", Value: "gpt-4o-mini"}}},
	}, testShellInfo())
	withRunnerStdin(t, "y\n", func() {
		if err := r.RunInstruction("set model"); err != nil {
			t.Fatalf("RunInstruction(config apply) error = %v", err)
		}
	})
	if cfg.Provider.Model != "gpt-4o-mini" {
		t.Fatalf("Model = %q, want gpt-4o-mini", cfg.Provider.Model)
	}

	cfg = config.DefaultConfig()
	r = New(cfg, &scriptedClient{
		results: []*llm.ChatResult{{Response: &llm.Response{Type: "config", Action: "set_model", Value: "ignored"}}},
	}, testShellInfo())
	withRunnerStdin(t, "n\n", func() {
		if err := r.RunInstruction("skip config"); err != nil {
			t.Fatalf("RunInstruction(config skip) error = %v", err)
		}
	})
	if cfg.Provider.Model == "ignored" {
		t.Fatal("config should not be applied when skipped")
	}

	r = New(config.DefaultConfig(), &scriptedClient{
		results: []*llm.ChatResult{{Response: &llm.Response{Type: "config", Action: "invalid_action", Value: "x"}}},
	}, testShellInfo())
	withRunnerStdin(t, "y\n", func() {
		if err := r.RunInstruction("bad config"); err == nil {
			t.Fatal("RunInstruction(invalid config) error = nil, want error")
		}
	})
}

func TestRetryPromptBranches(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.History.AskOnError = true
	cfg.History.AutoCheckOnError = false
	r := New(cfg, &scriptedClient{}, testShellInfo())
	session := history.NewSession("demo", "/tmp", cfg, testShellInfo())

	withTTYStub(t, false)
	_, stderr := captureRunnerOutput(t, func() {
		if got := r.shouldRetry(session, false); got {
			t.Fatal("shouldRetry(non-tty) = true, want false")
		}
	})
	if !strings.Contains(stderr, "enable history.auto_check_on_error") {
		t.Fatalf("stderr = %q, want retry hint", stderr)
	}

	withTTYStub(t, true)
	withRunnerStdin(t, "y\n", func() {
		if got := r.shouldRetry(session, false); !got {
			t.Fatal("shouldRetry(prompt yes) = false, want true")
		}
	})
	withRunnerStdin(t, "n\n", func() {
		if got := r.shouldRetry(session, false); got {
			t.Fatal("shouldRetry(prompt no) = true, want false")
		}
	})

	cfg.History.AutoCheckOnError = true
	cfg.History.RetryMaxAttempts = 1
	session.RetryCount = 0
	if got := r.shouldRetry(session, false); !got {
		t.Fatal("shouldRetry(auto) = false, want true")
	}
	session.RetryCount = 1
	if got := r.shouldRetry(session, true); got {
		t.Fatal("shouldRetry(auto maxed) = true, want false")
	}
	cfg.History.AskOnError = false
	cfg.History.AutoCheckOnError = false
	if got := r.shouldRetry(session, false); got {
		t.Fatal("shouldRetry(ask disabled) = true, want false")
	}
}

func TestRetrySessionMaxAttemptsExceeded(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.History.RetryMaxAttempts = 1
	r := New(cfg, &scriptedClient{}, testShellInfo())
	session := history.NewSession("demo", "/tmp", cfg, testShellInfo())
	session.RetryCount = 1
	if err := r.retrySession(session, "system", 1, true); err == nil {
		t.Fatal("retrySession(max attempts) error = nil, want error")
	}
}

func TestAppendMemories(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	r := New(config.DefaultConfig(), &scriptedClient{}, testShellInfo())
	if got := r.appendMemories("base", "nothing"); got != "base" {
		t.Fatalf("appendMemories(no memory) = %q, want base", got)
	}

	if err := memory.Add("docker", "use compose"); err != nil {
		t.Fatalf("memory.Add(): %v", err)
	}
	got := r.appendMemories("base", "show docker containers")
	if !strings.Contains(got, "use compose") {
		t.Fatalf("appendMemories(match) = %q, want injected memory", got)
	}
}

func TestApplyConfigAndStdinIsTTYHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	withRunnerStdin(t, "n\n", func() {
		if err := applyConfig(&llm.Response{Action: "set_model", Value: "nope"}, cfg); err != nil {
			t.Fatalf("applyConfig(skip) error = %v", err)
		}
	})

	withRunnerStdin(t, "y\n", func() {
		if err := applyConfig(&llm.Response{Action: "set_model", Value: "gpt-4o"}, cfg); err != nil {
			t.Fatalf("applyConfig(apply) error = %v", err)
		}
	})
	if cfg.Provider.Model != "gpt-4o" {
		t.Fatalf("Model = %q, want gpt-4o", cfg.Provider.Model)
	}

	withRunnerStdin(t, "y\n", func() {
		if err := applyConfig(&llm.Response{Action: "bad"}, cfg); err == nil {
			t.Fatal("applyConfig(invalid) error = nil, want error")
		}
		if stdinIsTTY() {
			t.Fatal("stdinIsTTY() should be false while stdin is a pipe in tests")
		}
	})
}

func loadOnlySession(t *testing.T) *history.Session {
	t.Helper()

	sessions, err := history.List()
	if err != nil {
		t.Fatalf("history.List(): %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("history.List() returned no sessions")
	}
	return &sessions[0]
}

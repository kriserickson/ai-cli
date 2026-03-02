package interactive

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/chzyer/readline"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

type fakeClient struct {
	chat func(systemPrompt, userMessage string) (*llm.Response, error)
}

func (f fakeClient) Chat(systemPrompt, userMessage string) (*llm.Response, error) {
	return f.chat(systemPrompt, userMessage)
}

func (f fakeClient) ChatMessages(messages []llm.Message) (*llm.Response, error) {
	// Extract system and user message for compatibility with existing tests
	var system, user string
	for _, m := range messages {
		switch m.Role {
		case "system":
			system = m.Content
		case "user":
			user = m.Content
		}
	}
	return f.chat(system, user)
}

type fakeReadlineStep struct {
	line string
	err  error
}

type fakeReadline struct {
	steps  []fakeReadlineStep
	idx    int
	closed bool
}

func (f *fakeReadline) Readline() (string, error) {
	if f.idx >= len(f.steps) {
		return "", io.EOF
	}
	step := f.steps[f.idx]
	f.idx++
	return step.line, step.err
}

func (f *fakeReadline) Close() error {
	f.closed = true
	return nil
}

func stubReplHooks(t *testing.T) {
	t.Helper()
	oldConfigDir := replConfigDir
	oldNewReadline := replNewReadline
	oldBuildPrompt := replBuildSystemPrompt
	oldHandleResponse := replHandleResponse

	t.Cleanup(func() {
		replConfigDir = oldConfigDir
		replNewReadline = oldNewReadline
		replBuildSystemPrompt = oldBuildPrompt
		replHandleResponse = oldHandleResponse
	})
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestRun_ConfigDirError(t *testing.T) {
	stubReplHooks(t)
	replConfigDir = func() (string, error) { return "", errors.New("config dir failed") }

	err := Run("dev", BuiltinCommands{}, testCfg(t), fakeClient{chat: func(string, string) (*llm.Response, error) {
		t.Fatal("client.Chat should not be called")
		return nil, nil
	}}, shell.Info{})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "config dir failed") {
		t.Fatalf("Run() error = %q, want config dir error", err.Error())
	}
}

func TestRun_ReadlineInitError(t *testing.T) {
	stubReplHooks(t)
	replConfigDir = func() (string, error) { return t.TempDir(), nil }
	replNewReadline = func(*readline.Config) (replLineReader, error) {
		return nil, errors.New("readline init failed")
	}

	err := Run("dev", BuiltinCommands{}, testCfg(t), fakeClient{chat: func(string, string) (*llm.Response, error) {
		t.Fatal("client.Chat should not be called")
		return nil, nil
	}}, shell.Info{})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to initialize readline") {
		t.Fatalf("Run() error = %q, want wrapped init error", err.Error())
	}
}

func TestRun_ExitOnEOFAndInterrupt(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "eof", err: io.EOF},
		{name: "interrupt", err: readline.ErrInterrupt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubReplHooks(t)
			replConfigDir = func() (string, error) { return t.TempDir(), nil }
			rl := &fakeReadline{steps: []fakeReadlineStep{{err: tt.err}}}
			replNewReadline = func(*readline.Config) (replLineReader, error) { return rl, nil }

			out := captureOutput(t, func() {
				err := Run("dev", BuiltinCommands{}, testCfg(t), fakeClient{chat: func(string, string) (*llm.Response, error) {
					t.Fatal("client.Chat should not be called")
					return nil, nil
				}}, shell.Info{})
				if err != nil {
					t.Fatalf("Run() error = %v, want nil", err)
				}
			})

			if !rl.closed {
				t.Fatal("readline.Close() was not called")
			}
			if !strings.Contains(out, "Bye!") {
				t.Fatalf("output missing Bye!: %q", out)
			}
		})
	}
}

func TestRun_ReadlineUnexpectedError(t *testing.T) {
	stubReplHooks(t)
	replConfigDir = func() (string, error) { return t.TempDir(), nil }
	rl := &fakeReadline{steps: []fakeReadlineStep{{err: errors.New("boom")}}}
	replNewReadline = func(*readline.Config) (replLineReader, error) { return rl, nil }

	err := Run("dev", BuiltinCommands{}, testCfg(t), fakeClient{chat: func(string, string) (*llm.Response, error) {
		t.Fatal("client.Chat should not be called")
		return nil, nil
	}}, shell.Info{})
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Run() error = %q, want readline error", err.Error())
	}
	if !rl.closed {
		t.Fatal("readline.Close() was not called")
	}
}

func TestRun_DispatchesBuiltinsAndLLM(t *testing.T) {
	stubReplHooks(t)
	replConfigDir = func() (string, error) { return t.TempDir(), nil }
	rl := &fakeReadline{
		steps: []fakeReadlineStep{
			{line: "   "},
			{line: "help"},
			{line: "version"},
			{line: "status"},
			{line: "doctor"},
			{line: "set-model"},
			{line: "config"},
			{line: "config show"},
			{line: "do something"},
			{line: "exit"},
		},
	}
	replNewReadline = func(*readline.Config) (replLineReader, error) { return rl, nil }

	var promptArgs []string
	replBuildSystemPrompt = func(osName, shellName, shellVersion, cwd string, _ bool) string {
		promptArgs = []string{osName, shellName, shellVersion, cwd}
		return "system-prompt"
	}

	var handled []*llm.Response
	replHandleResponse = func(resp *llm.Response, _ *config.Config, info shell.Info) error {
		if info.Shell != "powershell" {
			t.Fatalf("shellInfo not passed through: %+v", info)
		}
		handled = append(handled, resp)
		return nil
	}

	statusCalls := 0
	doctorCalls := 0
	setModelCalls := 0
	var configArgs [][]string
	clientCalls := 0
	client := fakeClient{chat: func(systemPrompt, userMessage string) (*llm.Response, error) {
		clientCalls++
		if systemPrompt != "system-prompt" {
			t.Fatalf("systemPrompt = %q, want %q", systemPrompt, "system-prompt")
		}
		if userMessage != "do something" {
			t.Fatalf("userMessage = %q, want %q", userMessage, "do something")
		}
		return &llm.Response{Type: "commands"}, nil
	}}

	cmds := BuiltinCommands{
		Status: func() error { statusCalls++; return nil },
		Doctor: func() error { doctorCalls++; return nil },
		SetModel: func() error {
			setModelCalls++
			return nil
		},
		ConfigRun: func(args []string) error {
			cp := append([]string(nil), args...)
			configArgs = append(configArgs, cp)
			return nil
		},
	}

	err := Run("1.2.3", cmds, testCfg(t), client, shell.Info{
		OS:      "windows/amd64",
		Shell:   "powershell",
		Version: "7.5.0",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !rl.closed {
		t.Fatal("readline.Close() was not called")
	}
	if statusCalls != 1 || doctorCalls != 1 || setModelCalls != 1 {
		t.Fatalf("builtin calls = status:%d doctor:%d set-model:%d", statusCalls, doctorCalls, setModelCalls)
	}
	if clientCalls != 1 {
		t.Fatalf("client.Chat calls = %d, want 1", clientCalls)
	}
	if len(handled) != 1 || handled[0].Type != "commands" {
		t.Fatalf("handled responses = %+v, want one commands response", handled)
	}
	if len(configArgs) != 2 {
		t.Fatalf("ConfigRun call count = %d, want 2", len(configArgs))
	}
	if configArgs[0] != nil {
		t.Fatalf("ConfigRun first args = %#v, want nil slice for bare 'config'", configArgs[0])
	}
	if !reflect.DeepEqual(configArgs[1], []string{"show"}) {
		t.Fatalf("ConfigRun second args = %#v, want %#v", configArgs[1], []string{"show"})
	}
	if !reflect.DeepEqual(promptArgs, []string{"windows/amd64", "powershell", "7.5.0", ""}) {
		t.Fatalf("BuildSystemPrompt args = %#v", promptArgs)
	}
}

func TestRun_ContinuesAfterBuiltinLLMAndHandleErrors(t *testing.T) {
	stubReplHooks(t)
	replConfigDir = func() (string, error) { return t.TempDir(), nil }
	rl := &fakeReadline{
		steps: []fakeReadlineStep{
			{line: "status"},
			{line: "doctor"},
			{line: "set-model"},
			{line: "config show"},
			{line: "first ai"},
			{line: "second ai"},
			{line: "quit"},
		},
	}
	replNewReadline = func(*readline.Config) (replLineReader, error) { return rl, nil }
	replBuildSystemPrompt = func(_, _, _, _ string, _ bool) string { return "sys" }

	handleCalls := 0
	replHandleResponse = func(_ *llm.Response, _ *config.Config, _ shell.Info) error {
		handleCalls++
		return errors.New("handle failed")
	}

	clientCalls := 0
	client := fakeClient{chat: func(_, user string) (*llm.Response, error) {
		clientCalls++
		if user == "first ai" {
			return nil, errors.New("chat failed")
		}
		return &llm.Response{Type: "commands"}, nil
	}}

	cmds := BuiltinCommands{
		Status:   func() error { return errors.New("status failed") },
		Doctor:   func() error { return errors.New("doctor failed") },
		SetModel: func() error { return errors.New("set-model failed") },
		ConfigRun: func([]string) error {
			return errors.New("config failed")
		},
	}

	out := captureOutput(t, func() {
		if err := Run("dev", cmds, testCfg(t), client, shell.Info{}); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	})

	if !rl.closed {
		t.Fatal("readline.Close() was not called")
	}
	if clientCalls != 2 {
		t.Fatalf("client.Chat calls = %d, want 2", clientCalls)
	}
	if handleCalls != 1 {
		t.Fatalf("handleResponse calls = %d, want 1", handleCalls)
	}
	if !strings.Contains(out, "Bye!") {
		t.Fatalf("output missing Bye!\n%s", out)
	}
}

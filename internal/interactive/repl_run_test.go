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
)

type fakeRunner struct {
	runInstruction func(string) error
	retryLast      func(int) error
}

func (f fakeRunner) RunInstruction(instruction string) error {
	if f.runInstruction == nil {
		return nil
	}
	return f.runInstruction(instruction)
}

func (f fakeRunner) RetryLastFailed(depth int) error {
	if f.retryLast == nil {
		return nil
	}
	return f.retryLast(depth)
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

	t.Cleanup(func() {
		replConfigDir = oldConfigDir
		replNewReadline = oldNewReadline
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

	err := Run("dev", BuiltinCommands{}, fakeRunner{})
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

	err := Run("dev", BuiltinCommands{}, fakeRunner{})
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
				err := Run("dev", BuiltinCommands{}, fakeRunner{})
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

	err := Run("dev", BuiltinCommands{}, fakeRunner{})
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

func TestRun_DispatchesBuiltinsRunnerAndRetry(t *testing.T) {
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
			{line: "history"},
			{line: "history show abc123"},
			{line: "do something"},
			{line: "retry 4"},
			{line: "exit"},
		},
	}
	replNewReadline = func(*readline.Config) (replLineReader, error) { return rl, nil }

	statusCalls := 0
	doctorCalls := 0
	setModelCalls := 0
	var configArgs [][]string
	var historyArgs [][]string
	var runInputs []string
	var retryDepths []int

	cmds := BuiltinCommands{
		Status:   func() error { statusCalls++; return nil },
		Doctor:   func() error { doctorCalls++; return nil },
		SetModel: func() error { setModelCalls++; return nil },
		ConfigRun: func(args []string) error {
			configArgs = append(configArgs, append([]string(nil), args...))
			return nil
		},
		HistoryRun: func(args []string) error {
			historyArgs = append(historyArgs, append([]string(nil), args...))
			return nil
		},
	}

	err := Run("1.2.3", cmds, fakeRunner{
		runInstruction: func(instruction string) error {
			runInputs = append(runInputs, instruction)
			return nil
		},
		retryLast: func(depth int) error {
			retryDepths = append(retryDepths, depth)
			return nil
		},
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
	if !reflect.DeepEqual(runInputs, []string{"do something"}) {
		t.Fatalf("RunInstruction inputs = %#v", runInputs)
	}
	if !reflect.DeepEqual(retryDepths, []int{4}) {
		t.Fatalf("RetryLastFailed depths = %#v", retryDepths)
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
	if len(historyArgs) != 2 {
		t.Fatalf("HistoryRun call count = %d, want 2", len(historyArgs))
	}
	if historyArgs[0] != nil {
		t.Fatalf("HistoryRun first args = %#v, want nil slice for bare 'history'", historyArgs[0])
	}
	if !reflect.DeepEqual(historyArgs[1], []string{"show", "abc123"}) {
		t.Fatalf("HistoryRun second args = %#v", historyArgs[1])
	}
}

func TestRun_ContinuesAfterErrors(t *testing.T) {
	stubReplHooks(t)
	replConfigDir = func() (string, error) { return t.TempDir(), nil }
	rl := &fakeReadline{
		steps: []fakeReadlineStep{
			{line: "status"},
			{line: "doctor"},
			{line: "set-model"},
			{line: "config show"},
			{line: "history list"},
			{line: "first ai"},
			{line: "retry"},
			{line: "quit"},
		},
	}
	replNewReadline = func(*readline.Config) (replLineReader, error) { return rl, nil }

	runCalls := 0
	retryCalls := 0
	cmds := BuiltinCommands{
		Status:   func() error { return errors.New("status failed") },
		Doctor:   func() error { return errors.New("doctor failed") },
		SetModel: func() error { return errors.New("set-model failed") },
		ConfigRun: func([]string) error {
			return errors.New("config failed")
		},
		HistoryRun: func([]string) error {
			return errors.New("history failed")
		},
	}

	out := captureOutput(t, func() {
		if err := Run("dev", cmds, fakeRunner{
			runInstruction: func(string) error {
				runCalls++
				return errors.New("runner failed")
			},
			retryLast: func(int) error {
				retryCalls++
				return errors.New("retry failed")
			},
		}); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	})

	if !rl.closed {
		t.Fatal("readline.Close() was not called")
	}
	if runCalls != 1 {
		t.Fatalf("RunInstruction calls = %d, want 1", runCalls)
	}
	if retryCalls != 1 {
		t.Fatalf("RetryLastFailed calls = %d, want 1", retryCalls)
	}
	if !strings.Contains(out, "Bye!") {
		t.Fatalf("output missing Bye!\n%s", out)
	}
}

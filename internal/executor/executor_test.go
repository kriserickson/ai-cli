package executor

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

func testShellInfo() shell.Info {
	if runtime.GOOS == "windows" {
		return shell.Info{Shell: "cmd"}
	}
	return shell.Info{Shell: "/bin/sh"}
}

func withTestStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	_ = w.Close()

	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	fn()
}

func TestAskConfirmation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty input defaults yes", input: "", want: true},
		{name: "y accepted", input: "y\n", want: true},
		{name: "yes accepted", input: "YES\n", want: true},
		{name: "n rejected", input: "n\n", want: false},
		{name: "other rejected", input: "nope\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTestStdin(t, tt.input, func() {
				if got := askConfirmation(); got != tt.want {
					t.Errorf("askConfirmation() = %v, want %v", got, tt.want)
				}
			})
		})
	}
}

func TestExecute(t *testing.T) {
	shellInfo := testShellInfo()

	if err := execute("echo executor-test", shellInfo); err != nil {
		t.Fatalf("execute() success case error = %v", err)
	}

	err := execute("__ai_cli_missing_command_for_test__", shellInfo)
	if err == nil {
		t.Fatal("execute() error case returned nil, want error")
	}
}

func TestRunSkipsWhenConfirmationDeclined(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmds := []llm.Command{{
		Command:     "__ai_cli_missing_command_for_skip_test__",
		Description: "should be skipped",
		Risk:        "risky",
		Certainty:   100,
	}}

	withTestStdin(t, "n\n", func() {
		if err := Run(cmds, cfg, testShellInfo()); err != nil {
			t.Fatalf("Run() error = %v, want nil when skipped", err)
		}
	})
}

func TestRunExecutesAndReturnsWrappedError(t *testing.T) {
	cfg := defaultSafetyCfg()
	cfg.Safety.AllowlistPrefixes = nil
	cfg.Safety.MinCertainty = 0

	cmds := []llm.Command{{
		Command:     "__ai_cli_missing_command_for_run_error_test__",
		Description: "should fail",
		Risk:        "safe",
		Certainty:   100,
	}}

	err := Run(cmds, cfg, testShellInfo())
	if err == nil {
		t.Fatal("Run() error = nil, want wrapped error")
	}
	if !strings.Contains(err.Error(), "command failed:") {
		t.Fatalf("Run() error = %q, want wrapped prefix", err.Error())
	}
}

func TestRunExecutesSafeCommandWithoutConfirmation(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmds := []llm.Command{{
		Command:     "echo run-ok",
		Description: "echo test",
		Risk:        "safe",
		Certainty:   100,
	}}

	if err := Run(cmds, cfg, testShellInfo()); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

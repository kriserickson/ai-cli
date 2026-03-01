package executor

import (
	"bytes"
	"io"
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
		if err := Run(cmds, cfg, testShellInfo(), false); err != nil {
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

	err := Run(cmds, cfg, testShellInfo(), false)
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

	if err := Run(cmds, cfg, testShellInfo(), false); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestRunWithExplainTrue(t *testing.T) {
	cfg := defaultSafetyCfg()
	const explanationText = "Unique explanation string that only appears in the explanation output."
	cmds := []llm.Command{{
		Command:     "echo hello",
		Description: "echo with explanation",
		Risk:        "safe",
		Certainty:   100,
		Explanation: explanationText,
	}}

	// Capture stdout to verify explanation is printed
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := Run(cmds, cfg, testShellInfo(), true)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("Run() error = %v, want nil", runErr)
	}
	output := buf.String()
	if !strings.Contains(output, explanationText) {
		t.Errorf("output missing explanation text:\n%s", output)
	}
}

func TestRunWithExplainFalse(t *testing.T) {
	cfg := defaultSafetyCfg()
	cmds := []llm.Command{{
		Command:     "echo no-explain",
		Description: "echo without explanation display",
		Risk:        "safe",
		Certainty:   100,
		Explanation: "This should NOT appear in output.",
	}}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := Run(cmds, cfg, testShellInfo(), false)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("Run() error = %v, want nil", runErr)
	}
	output := buf.String()
	if strings.Contains(output, "This should NOT appear") {
		t.Errorf("output should not contain explanation when explain=false:\n%s", output)
	}
}

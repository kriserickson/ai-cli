package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

func TestBuildRetryMessageIncludesRecentExecutionContext(t *testing.T) {
	session := &Session{
		Instruction: "fix the broken command",
		Exchanges: []Exchange{
			{
				Response: &llm.Response{
					Type:        "commands",
					Explanation: "Initial plan",
					Commands: []llm.Command{
						{Command: "false", Description: "fail", Risk: "safe", Certainty: 90},
					},
				},
			},
		},
	}
	session.RecordExecutions(1, []executor.CommandResult{
		{
			Index: 0,
			Command: llm.Command{
				Command:     "false",
				Description: "fail",
				Risk:        "safe",
				Certainty:   90,
			},
			Confirmed: true,
			ExitCode:  1,
			Stderr:    "boom",
			StartedAt: time.Now(),
		},
	})

	message := BuildRetryMessage(session, 1)
	for _, want := range []string{
		"Original user request:",
		"fix the broken command",
		"Most recent AI response:",
		"false",
		"Exit code: 1",
		"boom",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("retry message missing %q\n%s", want, message)
		}
	}
}

func TestListLoadRemoveAndClear(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	sessionA := NewSession("first command", "/tmp/one", cfg, shellInfo())
	sessionB := NewSession("second command", "/tmp/two", cfg, shellInfo())
	sessionA.ID = "alpha-session"
	sessionB.ID = "beta-session"
	sessionA.UpdatedAt = time.Now().UTC().Add(-time.Minute)
	sessionB.UpdatedAt = time.Now().UTC()

	if err := Save(sessionA); err != nil {
		t.Fatalf("Save(sessionA): %v", err)
	}
	if err := Save(sessionB); err != nil {
		t.Fatalf("Save(sessionB): %v", err)
	}

	sessions, err := List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("List() returned %d sessions, want 2", len(sessions))
	}
	if sessions[0].ID != sessionB.ID {
		t.Fatalf("List() order = %s first, want most recent %s", sessions[0].ID, sessionB.ID)
	}

	loaded, err := Load("alpha")
	if err != nil {
		t.Fatalf("Load(prefix): %v", err)
	}
	if loaded.Instruction != sessionA.Instruction {
		t.Fatalf("Load(prefix) instruction = %q, want %q", loaded.Instruction, sessionA.Instruction)
	}

	if err := Remove("alpha"); err != nil {
		t.Fatalf("Remove(prefix): %v", err)
	}
	sessions, err = List()
	if err != nil {
		t.Fatalf("List() after remove: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("List() after remove returned %d sessions, want 1", len(sessions))
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear(): %v", err)
	}
	sessions, err = List()
	if err != nil {
		t.Fatalf("List() after clear: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("List() after clear returned %d sessions, want 0", len(sessions))
	}

	log := filepath.Join(tmpDir, ".ai-cli", "history.log")
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("history.log should be removed after clear, stat err=%v", err)
	}
}

func TestSaveRotatesLog(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	session := NewSession("rotate", "/tmp", cfg, shellInfo())
	session.Exchanges = []Exchange{
		{
			Attempt:     1,
			Kind:        "initial",
			UserMessage: strings.Repeat("x", maxLogBytes),
			CreatedAt:   time.Now().UTC(),
		},
	}

	if err := Save(session); err != nil {
		t.Fatalf("Save(first): %v", err)
	}

	session.RetryCount = 1
	session.UpdatedAt = time.Now().UTC().Add(time.Second)
	if err := Save(session); err != nil {
		t.Fatalf("Save(second): %v", err)
	}

	base := filepath.Join(tmpDir, ".ai-cli", "history.log")
	rotated := base + ".1"
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("history.log stat: %v", err)
	}
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("history.log.1 stat: %v", err)
	}
}

func TestRecordExchangeAndLastFailedExecution(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.History.IncludeLLMOutput = false
	cfg.History.IncludeDebug = true

	session := NewSession("demo", "/tmp", cfg, shellInfo())
	session.RecordExchange("initial", 1, "system", "user", &llm.ChatResult{
		Response: &llm.Response{Type: "commands"},
		Trace:    llm.ChatTrace{Endpoint: "http://example"},
	}, errors.New("chat failed"), cfg.History)

	if session.Status != "failed" {
		t.Fatalf("session status = %q, want failed", session.Status)
	}
	if session.Exchanges[0].SystemPrompt != "" {
		t.Fatalf("system prompt should be omitted when IncludeLLMOutput is false")
	}
	if session.Exchanges[0].Response != nil {
		t.Fatalf("response should be omitted when IncludeLLMOutput is false")
	}
	if session.Exchanges[0].Trace == nil {
		t.Fatalf("trace should be included when IncludeDebug is true")
	}

	session.RecordExecutions(1, []executor.CommandResult{
		{Index: 0, Command: llm.Command{Command: "ok"}, ExitCode: 0},
		{Index: 1, Command: llm.Command{Command: "bad"}, ExitCode: 1},
	})
	failed := session.LastFailedExecution()
	if failed == nil || failed.Command != "bad" {
		t.Fatalf("LastFailedExecution() = %#v, want command 'bad'", failed)
	}
	onlySuccess := &Session{Executions: []CommandAttempt{{Command: "ok", ExitCode: 0}}}
	if onlySuccess.LastFailedExecution() != nil {
		t.Fatal("LastFailedExecution() on successful session = non-nil, want nil")
	}
	session.MarkStatus("completed")
	if session.Status != "completed" {
		t.Fatalf("MarkStatus() status = %q, want completed", session.Status)
	}
}

func TestRecordExchangeIncludesOutputWithoutDebug(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.History.IncludeLLMOutput = true
	cfg.History.IncludeDebug = false

	session := NewSession("demo", "/tmp", cfg, shellInfo())
	session.RecordExchange("retry", 2, "system prompt", "user prompt", &llm.ChatResult{
		Response: &llm.Response{
			Type:        "commands",
			Explanation: "retry with a fixed command",
			Commands: []llm.Command{
				{Command: "echo ok", Description: "confirm fix", Risk: "safe", Certainty: 95},
			},
		},
		Trace: llm.ChatTrace{Endpoint: "http://example"},
	}, nil, cfg.History)

	if got := session.Exchanges[0].SystemPrompt; got != "system prompt" {
		t.Fatalf("SystemPrompt = %q, want system prompt", got)
	}
	if session.Exchanges[0].Response == nil || session.Exchanges[0].Response.Explanation != "retry with a fixed command" {
		t.Fatalf("Response = %#v, want stored llm response", session.Exchanges[0].Response)
	}
	if session.Exchanges[0].Trace != nil {
		t.Fatalf("Trace = %#v, want nil when IncludeDebug is false", session.Exchanges[0].Trace)
	}
	if session.Exchanges[0].Error != "" {
		t.Fatalf("Error = %q, want empty", session.Exchanges[0].Error)
	}
	if session.Status != "pending" {
		t.Fatalf("Status = %q, want pending", session.Status)
	}
}

func TestLoadAndRemoveErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	a := NewSession("one", "/tmp", cfg, shellInfo())
	b := NewSession("two", "/tmp", cfg, shellInfo())
	a.ID = "shared-prefix-a"
	b.ID = "shared-prefix-b"
	if err := Save(a); err != nil {
		t.Fatalf("Save(a): %v", err)
	}
	if err := Save(b); err != nil {
		t.Fatalf("Save(b): %v", err)
	}

	if _, err := Load("missing"); err == nil {
		t.Fatal("Load(missing) error = nil, want error")
	}
	if _, err := Load("shared-prefix"); err == nil {
		t.Fatal("Load(ambiguous) error = nil, want error")
	}
	if err := Remove("missing"); err == nil {
		t.Fatal("Remove(missing) error = nil, want error")
	}
	if err := Remove("shared-prefix"); err == nil {
		t.Fatal("Remove(ambiguous) error = nil, want error")
	}
}

func TestReadLogAndClearHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	path, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}

	sessions, err := readLog(path)
	if err != nil {
		t.Fatalf("readLog(missing) error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("readLog(missing) len = %d, want 0", len(sessions))
	}

	if err := os.WriteFile(path, []byte("{not-json}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(invalid): %v", err)
	}
	if _, err := readLog(path); err == nil {
		t.Fatal("readLog(invalid) error = nil, want error")
	}
	if _, err := latestSessions(); err == nil {
		t.Fatal("latestSessions(invalid) error = nil, want error")
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(invalid log): %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear(missing logs) error = %v", err)
	}
}

func TestSaveAndClearErrorPaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, ".ai-cli"), []byte("blocker"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker): %v", err)
	}
	cfg := config.DefaultConfig()
	session := NewSession("blocked", "/tmp", cfg, shellInfo())
	if err := Save(session); err == nil {
		t.Fatal("Save() error = nil, want error when .ai-cli is a file")
	}

	tmpDir = t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	log, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(log, "nested"), 0o700); err != nil {
		t.Fatalf("MkdirAll(non-empty dir): %v", err)
	}
	if err := Clear(); err == nil {
		t.Fatal("Clear() error = nil, want error for non-empty history.log directory")
	}
}

func TestSaveOpenFileAndListErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	log, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(log, 0o700); err != nil {
		t.Fatalf("MkdirAll(log dir): %v", err)
	}
	cfg := config.DefaultConfig()
	session := NewSession("dir-blocked", "/tmp", cfg, shellInfo())
	if err := Save(session); err == nil {
		t.Fatal("Save() error = nil, want open-file error when history.log is a directory")
	}

	tmpDir = t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	log, err = logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o700); err != nil {
		t.Fatalf("MkdirAll(parent): %v", err)
	}
	if err := os.WriteFile(log, []byte("{bad json}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(invalid): %v", err)
	}
	if _, err := List(); err == nil {
		t.Fatal("List() error = nil, want invalid log error")
	}
}

func TestBuildRetryMessageAndPathHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	cfg := config.DefaultConfig()
	session := NewSession("fresh command", "/tmp", cfg, shellInfo())
	session.Executions = []CommandAttempt{{Index: 0, Command: "echo hi", ExitCode: 0, Stdout: "ok"}}
	message := BuildRetryMessage(session, 0)
	if !strings.Contains(message, "Recent command execution history:") {
		t.Fatalf("BuildRetryMessage() missing execution section:\n%s", message)
	}
	if strings.Contains(message, "Most recent AI response:") {
		t.Fatalf("BuildRetryMessage() should not include response section when there are no exchanges:\n%s", message)
	}

	log, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if !strings.HasSuffix(log, filepath.Join(".ai-cli", "history.log")) {
		t.Fatalf("logPath() = %q", log)
	}
	paths, err := logPaths()
	if err != nil {
		t.Fatalf("logPaths(): %v", err)
	}
	if len(paths) != maxBackups+1 {
		t.Fatalf("logPaths() len = %d, want %d", len(paths), maxBackups+1)
	}

	session.Exchanges = []Exchange{{Error: "llm parse failure"}}
	message = BuildRetryMessage(session, 1)
	if !strings.Contains(message, "llm parse failure") {
		t.Fatalf("BuildRetryMessage() missing exchange error:\n%s", message)
	}
}

func TestRotateIfNeededAndRewriteLogsErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	log, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o700); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := rotateIfNeeded(log, 1); err != nil {
		t.Fatalf("rotateIfNeeded(missing) error = %v", err)
	}
	if err := os.WriteFile(log, []byte("small"), 0o600); err != nil {
		t.Fatalf("WriteFile(small): %v", err)
	}
	if err := rotateIfNeeded(log, 1); err != nil {
		t.Fatalf("rotateIfNeeded(no rotate) error = %v", err)
	}

	if err := os.WriteFile(log, []byte(strings.Repeat("x", maxLogBytes)), 0o600); err != nil {
		t.Fatalf("WriteFile(big): %v", err)
	}
	if err := rotateIfNeeded(log, 1); err != nil {
		t.Fatalf("rotateIfNeeded(rotate) error = %v", err)
	}
	if _, err := os.Stat(log + ".1"); err != nil {
		t.Fatalf("rotated backup stat error = %v", err)
	}

	tmpDir = t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, ".ai-cli"), []byte("blocker"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker): %v", err)
	}
	cfg := config.DefaultConfig()
	session := NewSession("rewrite", "/tmp", cfg, shellInfo())
	if err := rewriteLogs([]Session{*session}); err == nil {
		t.Fatal("rewriteLogs() error = nil, want Save failure")
	}
}

func TestRotateIfNeededRemoveError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	log, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o700); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(log, []byte(strings.Repeat("x", maxLogBytes)), 0o600); err != nil {
		t.Fatalf("WriteFile(big): %v", err)
	}
	if err := os.MkdirAll(log+".5/nested", 0o700); err != nil {
		t.Fatalf("MkdirAll(blocking backup): %v", err)
	}
	if err := rotateIfNeeded(log, 1); err == nil {
		t.Fatal("rotateIfNeeded() error = nil, want remove error for non-empty .5 backup dir")
	}
}

func TestLatestSessionsDeduplicatesAcrossLogs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	log, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o700); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}

	older := Session{ID: "same", Instruction: "old", UpdatedAt: time.Now().UTC().Add(-time.Minute)}
	newer := Session{ID: "same", Instruction: "new", UpdatedAt: time.Now().UTC()}
	other := Session{ID: "other", Instruction: "other", UpdatedAt: time.Now().UTC()}
	writeLine := func(path string, session Session) {
		t.Helper()
		data, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("Marshal(): %v", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatalf("OpenFile(%s): %v", path, err)
		}
		defer f.Close()
		if _, err := f.Write(append(data, '\n')); err != nil {
			t.Fatalf("Write(%s): %v", path, err)
		}
	}

	writeLine(log+".1", older)
	writeLine(log, newer)
	writeLine(log, other)

	sessions, err := latestSessions()
	if err != nil {
		t.Fatalf("latestSessions(): %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("latestSessions() len = %d, want 2", len(sessions))
	}
	loaded, err := Load("same")
	if err != nil {
		t.Fatalf("Load(same): %v", err)
	}
	if loaded.Instruction != "new" {
		t.Fatalf("Load(same).Instruction = %q, want newest entry", loaded.Instruction)
	}
}

func TestRewriteLogsAndIndentBlock(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	if err := rewriteLogs(nil); err != nil {
		t.Fatalf("rewriteLogs(nil) error = %v", err)
	}

	cfg := config.DefaultConfig()
	session := NewSession("rewrite", "/tmp", cfg, shellInfo())
	if err := rewriteLogs([]Session{*session}); err != nil {
		t.Fatalf("rewriteLogs(non-empty) error = %v", err)
	}
	listed, err := List()
	if err != nil {
		t.Fatalf("List() after rewriteLogs: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() len = %d, want 1", len(listed))
	}

	if got := indentBlock("a\nb", "> "); got != "> a\n> b" {
		t.Fatalf("indentBlock() = %q", got)
	}
}

func TestClearRemovesBackupLogs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	base, err := logPath()
	if err != nil {
		t.Fatalf("logPath(): %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o700); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}

	paths, err := logPaths()
	if err != nil {
		t.Fatalf("logPaths(): %v", err)
	}
	for i := range paths {
		if err := os.WriteFile(paths[i], []byte("session\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", paths[i], err)
		}
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear(): %v", err)
	}
	for i := range paths {
		if _, err := os.Stat(paths[i]); !os.IsNotExist(err) {
			t.Fatalf("Stat(%s) err = %v, want not exist", paths[i], err)
		}
	}
}

func shellInfo() shell.Info {
	return shell.Info{OS: "darwin/arm64", Shell: "/bin/zsh", Version: "zsh 5"}
}

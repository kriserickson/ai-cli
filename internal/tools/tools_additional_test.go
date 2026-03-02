package tools

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
)

func writeFakeCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%s): %v", name, err)
	}
}

func TestDefaultConfirm(t *testing.T) {
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdin: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdout: %v", err)
	}

	os.Stdin = inR
	os.Stdout = outW

	if _, err := inW.WriteString("yes\n"); err != nil {
		t.Fatalf("stdin write: %v", err)
	}
	_ = inW.Close()

	if ok := defaultConfirm("read_file", map[string]string{"path": "README.md"}, "warning"); !ok {
		t.Fatal("defaultConfirm() = false, want true")
	}

	_ = outW.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, outR); err != nil {
		t.Fatalf("stdout read: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `AI wants to use tool "read_file"`) {
		t.Fatalf("prompt output missing tool name: %q", output)
	}
}

func TestCheckToolSafety(t *testing.T) {
	dir := t.TempDir()

	if got := checkToolSafety("disk_usage", nil); got != "" {
		t.Fatalf("checkToolSafety(non-path tool) = %q, want empty", got)
	}
	if got := checkToolSafety("read_file", map[string]string{"path": ""}); got != "" {
		t.Fatalf("checkToolSafety(empty path) = %q, want empty", got)
	}
	if got := checkToolSafety("read_file", map[string]string{"path": "."}); got != "" {
		t.Fatalf("checkToolSafety(dot path) = %q, want empty", got)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()

	if got := checkToolSafety("read_file", map[string]string{"path": "notes.txt"}); got != "" {
		t.Fatalf("checkToolSafety(safe file) = %q, want empty", got)
	}
	if got := checkToolSafety("read_file", map[string]string{"path": ".env"}); !strings.Contains(got, "blocked") {
		t.Fatalf("checkToolSafety(blocked file) = %q, want blocked message", got)
	}
}

func TestExecListDirectory_OutsideCWD(t *testing.T) {
	dir := t.TempDir()
	_, err := execListDirectory("../", dir)
	if err == nil {
		t.Fatal("execListDirectory() error = nil, want error")
	}
}

func TestExecListDirectory_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	output, err := execListDirectory("", dir)
	if err != nil {
		t.Fatalf("execListDirectory(\"\"): %v", err)
	}
	if !strings.Contains(output, "file.txt") {
		t.Fatalf("execListDirectory(\"\") = %q, want listed file", output)
	}
}

func TestExecListDirectory_NonExistent(t *testing.T) {
	dir := t.TempDir()
	_, err := execListDirectory("nonexistent", dir)
	if err == nil {
		t.Fatal("execListDirectory(nonexistent) error = nil, want error")
	}
}

func TestExecListDirectory_BlockedEntriesFiltered(t *testing.T) {
	dir := t.TempDir()
	// Create a safe file and several blocked files/dirs
	for _, name := range []string{"visible.txt", ".env", "secret.key", "id_rsa"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%s): %v", name, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, ".ssh"), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(.ssh): %v", err)
	}

	output, err := execListDirectory("", dir)
	if err != nil {
		t.Fatalf("execListDirectory: %v", err)
	}
	if !strings.Contains(output, "visible.txt") {
		t.Errorf("output should contain visible.txt, got: %s", output)
	}
	for _, blocked := range []string{".env", "secret.key", "id_rsa", ".ssh"} {
		if strings.Contains(output, blocked) {
			t.Errorf("output should not contain blocked entry %q, got: %s", blocked, output)
		}
	}
}

func TestExecListDirectory_CommandFailure(t *testing.T) {
	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "does_not_exist")

	_, err := execListDirectory(nonExistent, dir)
	if err == nil {
		t.Fatal("execListDirectory() error = nil, want error")
	}
}

func TestExecReadFile_DirectoryAndMissing(t *testing.T) {
	dir := t.TempDir()

	_, err := execReadFile(".", dir)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("execReadFile(directory) error = %v, want directory error", err)
	}

	_, err = execReadFile("missing.txt", dir)
	if err == nil || !strings.Contains(err.Error(), "cannot stat") {
		t.Fatalf("execReadFile(missing) error = %v, want stat error", err)
	}
}

func TestExecCommandHelp(t *testing.T) {
	_, err := execCommandHelp("")
	if err == nil {
		t.Fatal("execCommandHelp(\"\") error = nil, want error")
	}

	if runtime.GOOS == windowsOS {
		t.Skip("success path is shell-dependent on Windows")
	}

	output, err := execCommandHelp("ls")
	if err != nil {
		t.Fatalf("execCommandHelp(ls): %v", err)
	}
	if output == "" {
		t.Fatal("execCommandHelp(ls) returned empty output")
	}
}

func TestExecCommandHelp_UsesTldrAndManError(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("PATH-based command stubs are Unix-specific")
	}

	dir := t.TempDir()
	writeFakeCommand(t, dir, "tldr", "#!/bin/sh\necho 'TLDR help'\n")
	t.Setenv("PATH", dir)

	output, err := execCommandHelp("ls")
	if err != nil {
		t.Fatalf("execCommandHelp(tldr): %v", err)
	}
	if !strings.Contains(output, "TLDR help") {
		t.Fatalf("execCommandHelp(tldr) = %q, want fake tldr output", output)
	}

	dir = t.TempDir()
	writeFakeCommand(t, dir, "man", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir)

	_, err = execCommandHelp("ls")
	if err == nil || !strings.Contains(err.Error(), "man page not found") {
		t.Fatalf("execCommandHelp(man error) = %v, want man error", err)
	}
}

func TestExecCommandHelp_UsesMan(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("PATH-based command stubs are Unix-specific")
	}

	dir := t.TempDir()
	writeFakeCommand(t, dir, "man", "#!/bin/sh\necho 'MANPAGE help'\n")
	t.Setenv("PATH", dir)

	output, err := execCommandHelp("ls")
	if err != nil {
		t.Fatalf("execCommandHelp(man): %v", err)
	}
	if !strings.Contains(output, "MANPAGE help") {
		t.Fatalf("execCommandHelp(man) = %q, want fake man output", output)
	}
}

func TestExecCommandHelp_FallsBackFromTldrToMan(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("PATH-based command stubs are Unix-specific")
	}

	dir := t.TempDir()
	writeFakeCommand(t, dir, "tldr", "#!/bin/sh\nexit 1\n")
	writeFakeCommand(t, dir, "man", "#!/bin/sh\necho 'fallback man output'\n")
	t.Setenv("PATH", dir)

	output, err := execCommandHelp("ls")
	if err != nil {
		t.Fatalf("execCommandHelp(tldr fallback): %v", err)
	}
	if !strings.Contains(output, "fallback man output") {
		t.Fatalf("execCommandHelp(tldr fallback) = %q, want man fallback output", output)
	}
}

func TestExecListMemories_WithEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := memory.Save([]memory.Entry{
		{Keyword: "server", Content: "ssh user@example.com"},
	}); err != nil {
		t.Fatalf("memory.Save: %v", err)
	}

	output, err := execListMemories()
	if err != nil {
		t.Fatalf("execListMemories: %v", err)
	}
	if !strings.Contains(output, "server") || !strings.Contains(output, "ssh user@example.com") {
		t.Fatalf("execListMemories() = %q, want saved entry", output)
	}
}

func TestExecListMemories_ParseError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	memDir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "memory.json"), []byte("{invalid json"), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	_, err := execListMemories()
	if err == nil || !strings.Contains(err.Error(), "failed to parse memory.json") {
		t.Fatalf("execListMemories() error = %v, want parse error", err)
	}
}

func TestExecute_ProcessAndNetworkTools(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("PATH-based command stubs are Unix-specific")
	}

	dir := t.TempDir()
	writeFakeCommand(t, dir, "ps", "#!/bin/sh\necho 'fake process list'\n")
	writeFakeCommand(t, dir, "top", "#!/bin/sh\necho 'fake system resources'\n")
	writeFakeCommand(t, dir, "netstat", "#!/bin/sh\necho 'fake network connections'\n")
	writeFakeCommand(t, dir, "ping", "#!/bin/sh\necho 'fake ping output'\n")
	t.Setenv("PATH", dir)

	tests := []struct {
		name string
		tool string
		args map[string]string
	}{
		{name: "list processes", tool: "list_processes"},
		{name: "system resources", tool: "system_resources"},
		{name: "network connections", tool: "network_connections"},
		{name: "ping", tool: "ping", args: map[string]string{"host": "127.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := Execute(tt.tool, tt.args, shell.Info{})
			if err != nil {
				t.Fatalf("Execute(%s) error: %v", tt.tool, err)
			}
			if output == "" {
				t.Fatalf("Execute(%s) returned empty output", tt.tool)
			}
		})
	}
}

func TestExecPing_NoHost(t *testing.T) {
	_, err := execPing("")
	if err == nil {
		t.Fatal("execPing(\"\") error = nil, want error")
	}
}

func TestShellOutHelpers_ErrorPaths(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("PATH-based command stubs are Unix-specific")
	}

	tests := []struct {
		name    string
		command string
		body    string
		run     func() (string, error)
	}{
		{
			name:    "list_processes",
			command: "ps",
			body:    "#!/bin/sh\nexit 1\n",
			run:     execListProcesses,
		},
		{
			name:    "system_resources",
			command: "top",
			body:    "#!/bin/sh\nexit 1\n",
			run:     execSystemResources,
		},
		{
			name:    "network_connections",
			command: "netstat",
			body:    "#!/bin/sh\nexit 1\n",
			run:     execNetworkConnections,
		},
		{
			name:    "disk_usage",
			command: "df",
			body:    "#!/bin/sh\nexit 1\n",
			run:     execDiskUsage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFakeCommand(t, dir, tt.command, tt.body)
			t.Setenv("PATH", dir)

			_, err := tt.run()
			if err == nil {
				t.Fatalf("%s error = nil, want error", tt.name)
			}
		})
	}
}

func TestShellOutHelpers_SuccessPaths(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("PATH-based command stubs are Unix-specific")
	}

	tests := []struct {
		name    string
		command string
		output  string
		run     func() (string, error)
	}{
		{
			name:    "list_processes",
			command: "ps",
			output:  "process list",
			run:     execListProcesses,
		},
		{
			name:    "system_resources",
			command: "top",
			output:  "system resources",
			run:     execSystemResources,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFakeCommand(t, dir, tt.command, "#!/bin/sh\necho '"+tt.output+"'\n")
			t.Setenv("PATH", dir)

			output, err := tt.run()
			if err != nil {
				t.Fatalf("%s error: %v", tt.name, err)
			}
			if !strings.Contains(output, tt.output) {
				t.Fatalf("%s output = %q, want %q", tt.name, output, tt.output)
			}
		})
	}
}

func TestExecCheckCommand_NoArgument(t *testing.T) {
	_, err := execCheckCommand("")
	if err == nil {
		t.Fatal("execCheckCommand(\"\") error = nil, want error")
	}
}

func TestExecute_CommandHelp(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("success path is shell-dependent on Windows")
	}

	output, err := Execute("command_help", map[string]string{"command": "ls"}, shell.Info{})
	if err != nil {
		t.Fatalf("Execute(command_help) error: %v", err)
	}
	if !strings.Contains(strings.ToLower(output), "ls") {
		t.Fatalf("Execute(command_help) output = %q, want command help", output)
	}
}

func TestExecute_ListMemories_WithConfigHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	memDir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "memory.json"), []byte(`[{"keyword":"db","content":"postgres://localhost"}]`), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	output, err := Execute("list_memories", nil, shell.Info{})
	if err != nil {
		t.Fatalf("Execute(list_memories) error: %v", err)
	}
	if !strings.Contains(output, "postgres://localhost") {
		t.Fatalf("Execute(list_memories) = %q, want memory content", output)
	}
}

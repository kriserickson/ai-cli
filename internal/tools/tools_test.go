package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/shell"
)

func TestValidTool(t *testing.T) {
	if !ValidTool("list_directory") {
		t.Error("list_directory should be valid")
	}
	if !ValidTool("read_file") {
		t.Error("read_file should be valid")
	}
	if ValidTool("nonexistent") {
		t.Error("nonexistent should not be valid")
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	_, err := Execute("nonexistent", nil, shell.Info{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error = %q, want 'unknown tool'", err.Error())
	}
}

func TestExecute_ListDirectory(t *testing.T) {
	dir := t.TempDir()
	// Create a test file
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir so path validation works
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	output, err := Execute("list_directory", map[string]string{"path": "."}, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(output, "test.txt") {
		t.Errorf("output should contain test.txt, got: %s", output)
	}
}

func TestExecute_ReadFile(t *testing.T) {
	dir := t.TempDir()
	content := "hello world"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	output, err := Execute("read_file", map[string]string{"path": "test.txt"}, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if output != content {
		t.Errorf("output = %q, want %q", output, content)
	}
}

func TestExecute_ReadFile_TooLarge(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than 10KB
	data := make([]byte, 11*1024)
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	_, err := Execute("read_file", map[string]string{"path": "big.txt"}, shell.Info{})
	if err == nil {
		t.Fatal("expected error for file too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want 'too large'", err.Error())
	}
}

func TestExecute_ReadFile_Blocked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=x"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	_, err := Execute("read_file", map[string]string{"path": ".env"}, shell.Info{})
	if err == nil {
		t.Fatal("expected error for blocked file")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error = %q, want 'blocked'", err.Error())
	}
}

func TestExecute_ReadFile_NoPath(t *testing.T) {
	_, err := Execute("read_file", map[string]string{}, shell.Info{})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestExecute_CheckCommand(t *testing.T) {
	output, err := Execute("check_command", map[string]string{"command": "ls"}, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// On Unix, "which ls" returns a path
	if !strings.Contains(output, "ls") {
		t.Errorf("output should mention ls, got: %s", output)
	}
}

func TestExecute_CheckCommand_NotFound(t *testing.T) {
	output, err := Execute("check_command", map[string]string{"command": "nonexistent_command_xyz"}, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(output, "not found") {
		t.Errorf("output should say 'not found', got: %s", output)
	}
}

func TestExecute_Environment(t *testing.T) {
	t.Setenv("TEST_SAFE_VAR", "visible")
	t.Setenv("TEST_API_KEY", "should_be_hidden")

	output, err := Execute("environment", nil, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(output, "TEST_SAFE_VAR=visible") {
		t.Error("expected safe var to be visible")
	}
	if strings.Contains(output, "should_be_hidden") {
		t.Error("expected API_KEY value to be redacted")
	}
	if !strings.Contains(output, "TEST_API_KEY=[REDACTED]") {
		t.Error("expected API_KEY to show [REDACTED]")
	}
}

func TestExecute_DiskUsage(t *testing.T) {
	output, err := Execute("disk_usage", nil, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if output == "" {
		t.Error("disk_usage should return some output")
	}
}

func TestExecute_ListMemories(t *testing.T) {
	// This will return "No memories stored." since there's no memory file in test
	output, err := Execute("list_memories", nil, shell.Info{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if output == "" {
		t.Error("list_memories should return some output")
	}
}

func TestTruncateOutput(t *testing.T) {
	short := "hello"
	if truncateOutput(short) != short {
		t.Error("short output should not be truncated")
	}

	long := strings.Repeat("x", maxOutputBytes+100)
	result := truncateOutput(long)
	if len(result) > maxOutputBytes+30 {
		t.Errorf("truncated output too long: %d", len(result))
	}
	if !strings.Contains(result, "[output truncated]") {
		t.Error("truncated output should contain truncation notice")
	}
}

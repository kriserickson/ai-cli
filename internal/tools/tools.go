package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
)

const (
	maxOutputBytes = 4096
	windowsOS      = "windows"
)

// ToolDef describes a tool the AI can request.
type ToolDef struct {
	Name        string
	Description string
	Args        []string // expected arg keys
}

// Registry is the list of available tools.
var Registry = []ToolDef{
	{Name: "list_directory", Description: "List files in a directory", Args: []string{"path"}},
	{Name: "read_file", Description: "Read file contents (max 10KB, safety-checked)", Args: []string{"path"}},
	{Name: "command_help", Description: "Get help/man page for a command", Args: []string{"command"}},
	{Name: "list_memories", Description: "List stored AI CLI memories", Args: nil},
	{Name: "list_processes", Description: "List running processes", Args: nil},
	{Name: "system_resources", Description: "Show top processes by CPU/memory", Args: nil},
	{Name: "network_connections", Description: "Show active network connections", Args: nil},
	{Name: "ping", Description: "Check host connectivity (3 packets)", Args: []string{"host"}},
	{Name: "check_command", Description: "Check if a command is installed", Args: []string{"command"}},
	{Name: "disk_usage", Description: "Show disk space usage", Args: nil},
	{Name: "environment", Description: "Show environment variables (sensitive values masked)", Args: nil},
}

// Execute runs the named tool with the given args and returns its output.
func Execute(toolName string, args map[string]string, _ shell.Info) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get working directory: %w", err)
	}

	var output string

	switch toolName {
	case "list_directory":
		output, err = execListDirectory(args["path"], cwd)
	case "read_file":
		output, err = execReadFile(args["path"], cwd)
	case "command_help":
		output, err = execCommandHelp(args["command"])
	case "list_memories":
		output, err = execListMemories()
	case "list_processes":
		output, err = execListProcesses()
	case "system_resources":
		output, err = execSystemResources()
	case "network_connections":
		output, err = execNetworkConnections()
	case "ping":
		output, err = execPing(args["host"])
	case "check_command":
		output, err = execCheckCommand(args["command"])
	case "disk_usage":
		output, err = execDiskUsage()
	case "environment":
		output = execEnvironment()
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	if err != nil {
		return "", err
	}

	return truncateOutput(output), nil
}

// ValidTool returns true if the named tool exists in the registry.
func ValidTool(name string) bool {
	for _, t := range Registry {
		if t.Name == name {
			return true
		}
	}
	return false
}

func execListDirectory(path, cwd string) (string, error) {
	if path == "" {
		path = "."
	}
	absPath, err := ValidatePath(path, cwd)
	if err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", fmt.Sprintf("Get-ChildItem '%s'", absPath))
	} else {
		cmd = exec.CommandContext(context.Background(), "ls", "-la", absPath)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list_directory failed: %s", string(out))
	}
	return string(out), nil
}

func execReadFile(path, cwd string) (string, error) {
	if path == "" {
		return "", errors.New("read_file requires a path argument")
	}
	absPath, err := ValidatePath(path, cwd)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%q is a directory, not a file", path)
	}
	if info.Size() > 10*1024 {
		return "", fmt.Errorf("file %q is too large (%d bytes, max 10KB)", path, info.Size())
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %q: %w", path, err)
	}
	return string(data), nil
}

func execCommandHelp(command string) (string, error) {
	if command == "" {
		return "", errors.New("command_help requires a command argument")
	}

	if runtime.GOOS == windowsOS {
		cmd := exec.CommandContext(context.Background(), "powershell", "-Command", "Get-Help -Name $env:HELP_COMMAND")
		cmd.Env = append(os.Environ(), "HELP_COMMAND="+command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("Get-Help failed: %s", string(out))
		}
		return string(out), nil
	}

	// Try tldr first, fall back to man
	if tldrPath, err := exec.LookPath("tldr"); err == nil {
		cmd := exec.CommandContext(context.Background(), tldrPath, command)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), nil
		}
	}

	cmd := exec.CommandContext(context.Background(), "man", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("man page not found for %q", command)
	}
	return string(out), nil
}

func execListMemories() (string, error) {
	entries, err := memory.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load memories: %w", err)
	}
	if len(entries) == 0 {
		return "No memories stored.", nil
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "- %s: %s\n", e.Keyword, e.Content)
	}
	return b.String(), nil
}

func execListProcesses() (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", "Get-Process | Format-Table -AutoSize")
	} else {
		cmd = exec.CommandContext(context.Background(), "ps", "aux")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list_processes failed: %s", string(out))
	}
	return string(out), nil
}

func execSystemResources() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case windowsOS:
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", "Get-Process | Sort-Object CPU -Descending | Select-Object -First 5 | Format-Table Name,CPU,WorkingSet -AutoSize")
	case "darwin":
		cmd = exec.CommandContext(context.Background(), "top", "-l", "1", "-n", "5", "-s", "0")
	default: // linux
		cmd = exec.CommandContext(context.Background(), "top", "-bn1", "-o", "%CPU")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("system_resources failed: %s", string(out))
	}
	return string(out), nil
}

func execNetworkConnections() (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", "Get-NetTCPConnection | Format-Table -AutoSize")
	} else {
		cmd = exec.CommandContext(context.Background(), "netstat", "-an")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("network_connections failed: %s", string(out))
	}
	return string(out), nil
}

func execPing(host string) (string, error) {
	if host == "" {
		return "", errors.New("ping requires a host argument")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(context.Background(), "ping", "-n", "3", host)
	} else {
		cmd = exec.CommandContext(context.Background(), "ping", "-c", "3", host)
	}
	out, _ := cmd.CombinedOutput()
	return string(out), nil
}

func execCheckCommand(command string) (string, error) {
	if command == "" {
		return "", errors.New("check_command requires a command argument")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", "Get-Command -Name $env:CHECK_COMMAND")
		cmd.Env = append(os.Environ(), "CHECK_COMMAND="+command)
	} else {
		cmd = exec.CommandContext(context.Background(), "which", command)
	}
	out, _ := cmd.CombinedOutput()
	if len(out) == 0 {
		return command + ": not found", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func execDiskUsage() (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", "Get-PSDrive -PSProvider FileSystem | Format-Table Name,Used,Free -AutoSize")
	} else {
		cmd = exec.CommandContext(context.Background(), "df", "-h")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("disk_usage failed: %s", string(out))
	}
	return string(out), nil
}

func execEnvironment() string {
	vars := os.Environ()
	filtered := FilterEnvironment(vars)
	return strings.Join(filtered, "\n")
}

func truncateOutput(s string) string {
	if len(s) <= maxOutputBytes {
		return s
	}
	return s[:maxOutputBytes] + "\n... [output truncated]"
}

package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kriserickson/ai-cli/internal/memory"
	"github.com/kriserickson/ai-cli/internal/shell"
)

const (
	maxOutputBytes  = 4096
	windowsOS       = "windows"
	toolExecTimeout = 30 * time.Second
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

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return "", fmt.Errorf("list_directory failed: %w", err)
	}

	var b strings.Builder
	for _, entry := range entries {
		// Skip entries that match blocked patterns. If the relative path cannot
		// be computed, skip the entry to avoid accidentally exposing a blocked file.
		rel, relErr := filepath.Rel(cwd, filepath.Join(absPath, entry.Name()))
		if relErr != nil || isBlocked(rel) {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			// Entry may have been removed after ReadDir; skip it.
			continue
		}

		var size int64
		if !entry.IsDir() {
			size = info.Size()
		}

		entryType := "-"
		if entry.IsDir() {
			entryType = "d"
		} else if info.Mode()&fs.ModeSymlink != 0 {
			entryType = "l"
		}

		fmt.Fprintf(&b, "%s %10d %s %s\n", entryType+info.Mode().Perm().String(), size, info.ModTime().Format("Jan _2 15:04"), entry.Name())
	}
	return b.String(), nil
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

	ctx, cancel := context.WithTimeout(context.Background(), toolExecTimeout)
	defer cancel()

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
		cmd := exec.CommandContext(ctx, tldrPath, command)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), nil
		}
	}

	cmd := exec.CommandContext(ctx, "man", command)
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
	ctx, cancel := context.WithTimeout(context.Background(), toolExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(ctx, "powershell", "-Command", "Get-Process | Format-Table -AutoSize")
	} else {
		cmd = exec.CommandContext(ctx, "ps", "aux")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list_processes failed: %s", string(out))
	}
	return string(out), nil
}

func execSystemResources() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case windowsOS:
		cmd = exec.CommandContext(ctx, "powershell", "-Command", "Get-Process | Sort-Object CPU -Descending | Select-Object -First 5 | Format-Table Name,CPU,WorkingSet -AutoSize")
	case "darwin":
		cmd = exec.CommandContext(ctx, "top", "-l", "1", "-n", "5", "-s", "0")
	default: // linux
		cmd = exec.CommandContext(ctx, "top", "-bn1", "-o", "%CPU")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("system_resources failed: %s", string(out))
	}
	return string(out), nil
}

func execNetworkConnections() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(ctx, "powershell", "-Command", "Get-NetTCPConnection | Format-Table -AutoSize")
	} else if ssPath, err := exec.LookPath("ss"); err == nil {
		cmd = exec.CommandContext(ctx, ssPath, "-an")
	} else if netstatPath, err := exec.LookPath("netstat"); err == nil {
		cmd = exec.CommandContext(ctx, netstatPath, "-an")
	} else {
		return "", errors.New("network_connections failed: neither ss nor netstat found")
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

	ctx, cancel := context.WithTimeout(context.Background(), toolExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(ctx, "ping", "-n", "3", host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", "3", host)
	}
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() != nil {
		return "", fmt.Errorf("ping timed out after %s", toolExecTimeout)
	}
	return string(out), nil
}

func execCheckCommand(command string) (string, error) {
	if command == "" {
		return "", errors.New("check_command requires a command argument")
	}

	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("%s: %w", command, err)
	}

	return path, nil
}

func execDiskUsage() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == windowsOS {
		cmd = exec.CommandContext(ctx, "powershell", "-Command", "Get-PSDrive -PSProvider FileSystem | Format-Table Name,Used,Free -AutoSize")
	} else {
		cmd = exec.CommandContext(ctx, "df", "-h")
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

package shell

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Info struct {
	OS      string
	Shell   string
	Version string
}

func Detect() Info {
	info := Info{
		OS: runtime.GOOS + "/" + runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "windows":
		info.Shell = detectWindowsShell()
	default:
		info.Shell = detectUnixShell()
	}

	info.Version = detectShellVersion(info.Shell)
	return info
}

func detectUnixShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return shell
	}
	return "/bin/sh"
}

func detectWindowsShell() string {
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	return "cmd"
}

func detectShellVersion(shell string) string {
	base := shellBaseName(shell)
	var cmd *exec.Cmd

	switch base {
	case "bash", "zsh", "fish":
		cmd = exec.Command(shell, "--version")
	case "powershell", "pwsh":
		cmd = exec.Command(shell, "-Command", "$PSVersionTable.PSVersion.ToString()")
	default:
		return "unknown"
	}

	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	firstLine := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	return strings.TrimSpace(firstLine)
}

func shellBaseName(shell string) string {
	// Handle both Unix (/) and Windows (\) path separators
	name := shell
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".exe")
	return name
}

// ShellCommand returns the command prefix for executing a string in the detected shell.
func ShellCommand(shell string) (string, []string) {
	base := shellBaseName(shell)
	switch base {
	case "powershell", "pwsh":
		return shell, []string{"-Command"}
	case "cmd":
		return shell, []string{"/c"}
	default:
		return shell, []string{"-c"}
	}
}

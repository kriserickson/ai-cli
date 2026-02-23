package shell

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const shellPowershell = "powershell"

var (
	detectParentShellProcess  = parentShellProcess
	detectPreferredPowershell = preferredPowerShell
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
	// Git Bash / MSYS2 / Cygwin: check before anything else because
	// PSModulePath may still be set in their environments.
	if os.Getenv("MSYSTEM") != "" || os.Getenv("BASH_VERSION") != "" {
		if shell := os.Getenv("SHELL"); shell != "" {
			return shell
		}
		return "bash"
	}

	// Walk the process tree to find the actual parent shell.
	if name := detectParentShellProcess(); name != "" {
		return name
	}

	// Fallback: PSModulePath heuristic (set by PowerShell in its child processes).
	if os.Getenv("PSModulePath") != "" {
		return detectPreferredPowershell()
	}

	return "cmd"
}

func detectShellVersion(shell string) string {
	base := shellBaseName(shell)
	var cmd *exec.Cmd

	switch base {
	case "bash", "zsh", "fish":
		cmd = exec.CommandContext(context.Background(), shell, "--version")
	case shellPowershell, "pwsh":
		cmd = exec.CommandContext(context.Background(), shell, "-Command", "$PSVersionTable.PSVersion.ToString()")
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
func ShellCommand(shell string) (bin string, args []string) {
	base := shellBaseName(shell)
	switch base {
	case shellPowershell, "pwsh":
		return shell, []string{"-Command"}
	case "cmd":
		return shell, []string{"/c"}
	default:
		return shell, []string{"-c"}
	}
}

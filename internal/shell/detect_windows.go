//go:build windows

package shell

import (
	"os"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	windowsLookPath = exec.LookPath
	windowsStat     = os.Stat
)

// parentShellProcess walks up the process parent chain to identify the
// shell that ultimately launched this process (cmd, powershell, pwsh, bash).
func parentShellProcess() string {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(snapshot)

	type procInfo struct {
		ppid uint32
		name string
	}
	procs := make(map[uint32]procInfo)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))

	if err := windows.Process32First(snapshot, &pe); err != nil {
		return ""
	}
	for {
		name := windows.UTF16ToString(pe.ExeFile[:])
		procs[pe.ProcessID] = procInfo{pe.ParentProcessID, name}
		if err := windows.Process32Next(snapshot, &pe); err != nil {
			break
		}
	}

	// Walk up from the current process up to 10 levels.
	pid := uint32(os.Getpid())
	for i := 0; i < 10; i++ {
		info, ok := procs[pid]
		if !ok {
			break
		}
		pid = info.ppid
		parent, ok := procs[pid]
		if !ok {
			break
		}
		switch strings.ToLower(parent.name) {
		case "cmd.exe":
			return "cmd"
		case "powershell.exe":
			return preferredPowerShell()
		case "pwsh.exe":
			return pwshExePath()
		case "bash.exe", "git-bash.exe", "sh.exe":
			// Only use SHELL if it looks like a native Windows path; MSYS-style
			// paths (e.g. /usr/bin/bash) are not valid Windows executables.
			if shell := os.Getenv("SHELL"); shell != "" && !strings.HasPrefix(shell, "/") {
				return shell
			}
			return "bash"
		}
	}
	return ""
}

// preferredPowerShell returns the path to the best available PowerShell binary.
// It prefers PowerShell 7+ (pwsh) over the legacy Windows PowerShell 5 (powershell).
func preferredPowerShell() string {
	if path, err := windowsLookPath("pwsh"); err == nil {
		return path
	}
	for _, candidate := range []string{
		`C:\Program Files\PowerShell\7\pwsh.exe`,
		`C:\Program Files\PowerShell\6\pwsh.exe`,
	} {
		if _, err := windowsStat(candidate); err == nil {
			return candidate
		}
	}
	return "powershell"
}

// pwshExePath returns the full path to pwsh.exe, falling back to "pwsh".
func pwshExePath() string {
	if path, err := windowsLookPath("pwsh"); err == nil {
		return path
	}
	for _, candidate := range []string{
		`C:\Program Files\PowerShell\7\pwsh.exe`,
		`C:\Program Files\PowerShell\6\pwsh.exe`,
	} {
		if _, err := windowsStat(candidate); err == nil {
			return candidate
		}
	}
	return "pwsh"
}

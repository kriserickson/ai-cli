package shell

import (
	"runtime"
	"strings"
	"testing"
)

func TestDetect_ReturnsOS(t *testing.T) {
	info := Detect()
	want := runtime.GOOS + "/" + runtime.GOARCH
	if info.OS != want {
		t.Errorf("OS = %q, want %q", info.OS, want)
	}
}

func TestDetect_ShellNotEmpty(t *testing.T) {
	info := Detect()
	if info.Shell == "" {
		t.Error("Shell should not be empty")
	}
}

func TestDetect_VersionNotEmpty(t *testing.T) {
	info := Detect()
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestShellBaseName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/bin/zsh", "zsh"},
		{"/usr/bin/bash", "bash"},
		{"/bin/sh", "sh"},
		{"bash", "bash"},
		{"powershell.exe", "powershell"},
		{"C:\\Windows\\System32\\cmd.exe", "cmd"},
		{"C:\\Program Files\\PowerShell\\7\\pwsh.exe", "pwsh"},
		{"pwsh.exe", "pwsh"},
		{"C:\\Program Files\\Git\\usr\\bin\\bash.exe", "bash"},
	}
	for _, tt := range tests {
		got := shellBaseName(tt.input)
		if got != tt.want {
			t.Errorf("shellBaseName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShellCommand_Unix(t *testing.T) {
	tests := []struct {
		shell    string
		wantBin  string
		wantArgs []string
	}{
		{"/bin/zsh", "/bin/zsh", []string{"-c"}},
		{"/bin/bash", "/bin/bash", []string{"-c"}},
		{"/bin/sh", "/bin/sh", []string{"-c"}},
		{"powershell", "powershell", []string{"-Command"}},
		{"pwsh", "pwsh", []string{"-Command"}},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, `C:\Program Files\PowerShell\7\pwsh.exe`, []string{"-Command"}},
		{"cmd", "cmd", []string{"/c"}},
		{"bash", "bash", []string{"-c"}},
	}
	for _, tt := range tests {
		bin, args := ShellCommand(tt.shell)
		if bin != tt.wantBin {
			t.Errorf("ShellCommand(%q) bin = %q, want %q", tt.shell, bin, tt.wantBin)
		}
		if len(args) != len(tt.wantArgs) || args[0] != tt.wantArgs[0] {
			t.Errorf("ShellCommand(%q) args = %v, want %v", tt.shell, args, tt.wantArgs)
		}
	}
}

func TestDetectWindowsShell_GitBash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}

	tests := []struct {
		name   string
		envKey string
		envVal string
		want   string
	}{
		{"MSYSTEM set (MINGW64)", "MSYSTEM", "MINGW64", "bash"},
		{"BASH_VERSION set", "BASH_VERSION", "5.2.0(1)-release", "bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			// Also clear SHELL so result is predictable
			t.Setenv("SHELL", "")
			got := detectWindowsShell()
			if got != tt.want {
				t.Errorf("detectWindowsShell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectUnixShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	if got := detectUnixShell(); got != "/bin/zsh" {
		t.Fatalf("detectUnixShell() = %q, want %q", got, "/bin/zsh")
	}

	t.Setenv("SHELL", "")
	if got := detectUnixShell(); got != "/bin/sh" {
		t.Fatalf("detectUnixShell() = %q, want %q", got, "/bin/sh")
	}
}

func TestDetectShellVersion_Branches(t *testing.T) {
	if got := detectShellVersion("cmd"); got != "unknown" {
		t.Fatalf("detectShellVersion(cmd) = %q, want unknown", got)
	}

	// Base name resolves to "bash", forcing the exec path and error branch.
	if got := detectShellVersion("/definitely/not/a/real/bash"); got != "unknown" {
		t.Fatalf("detectShellVersion(nonexistent bash path) = %q, want unknown", got)
	}

	// Smoke test a likely available shell on this environment without making
	// the test fail if it is missing.
	if runtime.GOOS == "windows" {
		got := detectShellVersion("pwsh")
		if got != "unknown" && strings.TrimSpace(got) == "" {
			t.Fatalf("detectShellVersion(pwsh) returned empty non-unknown value")
		}
	}
}

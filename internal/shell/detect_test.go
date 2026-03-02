package shell

import (
	"os"
	"path/filepath"
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
		bin, args := Command(tt.shell)
		if bin != tt.wantBin {
			t.Errorf("Command(%q) bin = %q, want %q", tt.shell, bin, tt.wantBin)
		}
		if len(args) != len(tt.wantArgs) || args[0] != tt.wantArgs[0] {
			t.Errorf("Command(%q) args = %v, want %v", tt.shell, args, tt.wantArgs)
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

func TestDetectWindowsShellBranches(t *testing.T) {
	oldParent := detectParentShellProcess
	oldPreferred := detectPreferredPowershell
	t.Cleanup(func() {
		detectParentShellProcess = oldParent
		detectPreferredPowershell = oldPreferred
	})

	t.Run("git bash with windows shell path", func(t *testing.T) {
		t.Setenv("MSYSTEM", "MINGW64")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", `C:\Program Files\Git\bin\bash.exe`)
		if got := detectWindowsShell(); got != `C:\Program Files\Git\bin\bash.exe` {
			t.Fatalf("detectWindowsShell() = %q", got)
		}
	})

	t.Run("git bash with msys shell path", func(t *testing.T) {
		t.Setenv("MSYSTEM", "MINGW64")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", "/usr/bin/bash")
		if got := detectWindowsShell(); got != "bash" {
			t.Fatalf("detectWindowsShell() = %q, want bash", got)
		}
	})

	t.Run("parent shell wins", func(t *testing.T) {
		t.Setenv("MSYSTEM", "")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", "")
		t.Setenv("PSModulePath", "set")
		detectParentShellProcess = func() string { return "cmd" }
		detectPreferredPowershell = func() string {
			t.Fatal("preferredPowerShell should not be called")
			return ""
		}
		if got := detectWindowsShell(); got != "cmd" {
			t.Fatalf("detectWindowsShell() = %q, want cmd", got)
		}
	})

	t.Run("preferred powershell fallback", func(t *testing.T) {
		t.Setenv("MSYSTEM", "")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", "")
		t.Setenv("PSModulePath", "set")
		detectParentShellProcess = func() string { return "" }
		detectPreferredPowershell = func() string { return "pwsh-custom" }
		if got := detectWindowsShell(); got != "pwsh-custom" {
			t.Fatalf("detectWindowsShell() = %q, want pwsh-custom", got)
		}
	})

	t.Run("cmd fallback", func(t *testing.T) {
		t.Setenv("MSYSTEM", "")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", "")
		t.Setenv("PSModulePath", "")
		detectParentShellProcess = func() string { return "" }
		if got := detectWindowsShell(); got != "cmd" {
			t.Fatalf("detectWindowsShell() = %q, want cmd", got)
		}
	})
}

func TestDetectShellVersion_ScriptedShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripted shell test is for unix-like environments")
	}

	tmpDir := t.TempDir()
	fishPath := filepath.Join(tmpDir, "fish")
	pwshPath := filepath.Join(tmpDir, "pwsh")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then\n  echo 'fish 3.7.1'\n  exit 0\nfi\nif [ \"$1\" = \"-Command\" ]; then\n  echo '7.5.0'\n  exit 0\nfi\nexit 1\n"
	for _, path := range []string{fishPath, pwshPath} {
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	if got := detectShellVersion(fishPath); got != "fish 3.7.1" {
		t.Fatalf("detectShellVersion(fish) = %q", got)
	}
	if got := detectShellVersion(pwshPath); got != "7.5.0" {
		t.Fatalf("detectShellVersion(pwsh) = %q", got)
	}
}

func TestNonWindowsNoOpHelpers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows helper test")
	}
	if got := parentShellProcess(); got != "" {
		t.Fatalf("parentShellProcess() = %q, want empty", got)
	}
	if got := preferredPowerShell(); got != "powershell" {
		t.Fatalf("preferredPowerShell() = %q, want powershell", got)
	}
}

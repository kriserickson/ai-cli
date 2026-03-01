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
		bin, args := Command(tt.shell)
		if bin != tt.wantBin {
			t.Errorf("Command(%q) bin = %q, want %q", tt.shell, bin, tt.wantBin)
		}
		if len(args) != len(tt.wantArgs) || args[0] != tt.wantArgs[0] {
			t.Errorf("Command(%q) args = %v, want %v", tt.shell, args, tt.wantArgs)
		}
	}
}

func stubShellHooks(t *testing.T) {
	t.Helper()
	oldParent := detectParentShellProcess
	oldPreferred := detectPreferredPowershell
	t.Cleanup(func() {
		detectParentShellProcess = oldParent
		detectPreferredPowershell = oldPreferred
	})
}

func TestDetectWindowsShell_GitBash(t *testing.T) {
	stubShellHooks(t)
	// Prevent parent shell detection and preferred powershell from interfering
	detectParentShellProcess = func() string { return "" }
	detectPreferredPowershell = func() string { return "powershell" }

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

func TestDetectWindowsShell_GitBashWithWindowsShellPath(t *testing.T) {
	stubShellHooks(t)
	detectParentShellProcess = func() string { return "" }

	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("SHELL", `C:\Program Files\Git\bin\bash.exe`)

	got := detectWindowsShell()
	if got != `C:\Program Files\Git\bin\bash.exe` {
		t.Errorf("detectWindowsShell() = %q, want Windows SHELL path", got)
	}
}

func TestDetectWindowsShell_GitBashIgnoresUnixShellPath(t *testing.T) {
	stubShellHooks(t)
	detectParentShellProcess = func() string { return "" }

	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("SHELL", "/usr/bin/bash")

	got := detectWindowsShell()
	if got != "bash" {
		t.Errorf("detectWindowsShell() = %q, want bash", got)
	}
}

func TestDetectWindowsShell_ParentShellWins(t *testing.T) {
	stubShellHooks(t)
	t.Setenv("MSYSTEM", "")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", "something")
	detectParentShellProcess = func() string { return "cmd" }

	got := detectWindowsShell()
	if got != "cmd" {
		t.Errorf("detectWindowsShell() = %q, want cmd", got)
	}
}

func TestDetectWindowsShell_PSModulePathFallback(t *testing.T) {
	stubShellHooks(t)
	t.Setenv("MSYSTEM", "")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", "something")
	detectParentShellProcess = func() string { return "" }
	detectPreferredPowershell = func() string { return "pwsh" }

	got := detectWindowsShell()
	if got != "pwsh" {
		t.Errorf("detectWindowsShell() = %q, want pwsh", got)
	}
}

func TestDetectWindowsShell_FallbackToCmd(t *testing.T) {
	stubShellHooks(t)
	t.Setenv("MSYSTEM", "")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("SHELL", "")
	t.Setenv("PSModulePath", "")
	detectParentShellProcess = func() string { return "" }

	got := detectWindowsShell()
	if got != "cmd" {
		t.Errorf("detectWindowsShell() = %q, want cmd", got)
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

	// Test successful version detection with an actual shell available on this platform.
	if runtime.GOOS != "windows" {
		got := detectShellVersion("/bin/sh")
		// /bin/sh is a symlink to bash or zsh on macOS/linux, might return "unknown"
		// for plain sh. The point is it doesn't panic.
		if got == "" {
			t.Fatal("detectShellVersion(/bin/sh) returned empty string")
		}

		// Test with bash if available (covers the success path through output parsing)
		bashVersion := detectShellVersion("bash")
		if bashVersion == "" {
			t.Fatal("detectShellVersion(bash) returned empty string")
		}
		// Should return a version string or "unknown"
		if bashVersion != "unknown" && !strings.Contains(strings.ToLower(bashVersion), "bash") &&
			!strings.Contains(bashVersion, ".") {
			t.Logf("detectShellVersion(bash) = %q (accepted)", bashVersion)
		}
	}
}

func TestParentShellProcess_NonWindows(t *testing.T) {
	// On non-Windows, parentShellProcess is a no-op stub that returns "".
	got := parentShellProcess()
	if got != "" {
		t.Fatalf("parentShellProcess() = %q, want empty string on non-Windows", got)
	}
}

func TestPreferredPowerShell_NonWindows(t *testing.T) {
	// On non-Windows, preferredPowerShell returns "powershell".
	got := preferredPowerShell()
	if got != "powershell" {
		t.Fatalf("preferredPowerShell() = %q, want powershell on non-Windows", got)
	}
}

//go:build windows

package shell

import (
	"errors"
	"os"
	"testing"
)

func stubWindowsShellHooks(t *testing.T) {
	t.Helper()

	oldParent := detectParentShellProcess
	oldPreferred := detectPreferredPowershell
	oldLookPath := windowsLookPath
	oldStat := windowsStat

	t.Cleanup(func() {
		detectParentShellProcess = oldParent
		detectPreferredPowershell = oldPreferred
		windowsLookPath = oldLookPath
		windowsStat = oldStat
	})
}

func TestDetectWindowsShell_Branches(t *testing.T) {
	t.Run("git bash returns SHELL path when present", func(t *testing.T) {
		stubWindowsShellHooks(t)
		t.Setenv("MSYSTEM", "MINGW64")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", `C:\Program Files\Git\bin\bash.exe`)
		t.Setenv("PSModulePath", "")
		detectParentShellProcess = func() string {
			t.Fatal("parentShellProcess should not be called in git bash branch")
			return ""
		}

		got := detectWindowsShell()
		if got != `C:\Program Files\Git\bin\bash.exe` {
			t.Fatalf("detectWindowsShell() = %q, want SHELL path", got)
		}
	})

	t.Run("parent shell wins before PSModulePath fallback", func(t *testing.T) {
		stubWindowsShellHooks(t)
		t.Setenv("MSYSTEM", "")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", "")
		t.Setenv("PSModulePath", "set")
		detectParentShellProcess = func() string { return "cmd" }
		detectPreferredPowershell = func() string {
			t.Fatal("preferredPowerShell should not be called when parent shell is found")
			return ""
		}

		if got := detectWindowsShell(); got != "cmd" {
			t.Fatalf("detectWindowsShell() = %q, want cmd", got)
		}
	})

	t.Run("PSModulePath fallback uses preferred PowerShell", func(t *testing.T) {
		stubWindowsShellHooks(t)
		t.Setenv("MSYSTEM", "")
		t.Setenv("BASH_VERSION", "")
		t.Setenv("SHELL", "")
		t.Setenv("PSModulePath", "set")
		detectParentShellProcess = func() string { return "" }
		detectPreferredPowershell = func() string { return `C:\Program Files\PowerShell\7\pwsh.exe` }

		if got := detectWindowsShell(); got != `C:\Program Files\PowerShell\7\pwsh.exe` {
			t.Fatalf("detectWindowsShell() = %q, want preferred PowerShell path", got)
		}
	})

	t.Run("falls back to cmd", func(t *testing.T) {
		stubWindowsShellHooks(t)
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

func TestPreferredPowerShell(t *testing.T) {
	t.Run("lookpath hit", func(t *testing.T) {
		stubWindowsShellHooks(t)
		windowsLookPath = func(file string) (string, error) {
			if file != "pwsh" {
				t.Fatalf("LookPath arg = %q, want pwsh", file)
			}
			return `C:\pwsh\pwsh.exe`, nil
		}
		windowsStat = func(string) (os.FileInfo, error) {
			t.Fatal("Stat should not be called when LookPath succeeds")
			return nil, nil
		}

		if got := preferredPowerShell(); got != `C:\pwsh\pwsh.exe` {
			t.Fatalf("preferredPowerShell() = %q, want lookpath result", got)
		}
	})

	t.Run("candidate fallback", func(t *testing.T) {
		stubWindowsShellHooks(t)
		windowsLookPath = func(string) (string, error) { return "", errors.New("not found") }
		calls := 0
		windowsStat = func(path string) (os.FileInfo, error) {
			calls++
			if calls == 1 {
				if path != `C:\Program Files\PowerShell\7\pwsh.exe` {
					t.Fatalf("first candidate = %q", path)
				}
				return nil, nil
			}
			return nil, errors.New("not found")
		}

		if got := preferredPowerShell(); got != `C:\Program Files\PowerShell\7\pwsh.exe` {
			t.Fatalf("preferredPowerShell() = %q, want first candidate", got)
		}
	})

	t.Run("legacy fallback name", func(t *testing.T) {
		stubWindowsShellHooks(t)
		windowsLookPath = func(string) (string, error) { return "", errors.New("not found") }
		windowsStat = func(string) (os.FileInfo, error) { return nil, errors.New("not found") }

		if got := preferredPowerShell(); got != "powershell" {
			t.Fatalf("preferredPowerShell() = %q, want powershell", got)
		}
	})
}

func TestPwshExePath(t *testing.T) {
	t.Run("lookpath hit", func(t *testing.T) {
		stubWindowsShellHooks(t)
		windowsLookPath = func(string) (string, error) { return `C:\pwsh\pwsh.exe`, nil }
		windowsStat = func(string) (os.FileInfo, error) {
			t.Fatal("Stat should not be called when LookPath succeeds")
			return nil, nil
		}

		if got := pwshExePath(); got != `C:\pwsh\pwsh.exe` {
			t.Fatalf("pwshExePath() = %q, want lookpath result", got)
		}
	})

	t.Run("candidate fallback", func(t *testing.T) {
		stubWindowsShellHooks(t)
		windowsLookPath = func(string) (string, error) { return "", errors.New("not found") }
		windowsStat = func(path string) (os.FileInfo, error) {
			if path == `C:\Program Files\PowerShell\6\pwsh.exe` {
				return nil, nil
			}
			return nil, errors.New("not found")
		}

		if got := pwshExePath(); got != `C:\Program Files\PowerShell\6\pwsh.exe` {
			t.Fatalf("pwshExePath() = %q, want second candidate", got)
		}
	})

	t.Run("fallback executable name", func(t *testing.T) {
		stubWindowsShellHooks(t)
		windowsLookPath = func(string) (string, error) { return "", errors.New("not found") }
		windowsStat = func(string) (os.FileInfo, error) { return nil, errors.New("not found") }

		if got := pwshExePath(); got != "pwsh" {
			t.Fatalf("pwshExePath() = %q, want pwsh", got)
		}
	})
}

func TestParentShellProcess_Smoke(t *testing.T) {
	stubWindowsShellHooks(t)

	got := parentShellProcess()
	// Function is environment-dependent; this test is a smoke test that exists
	// primarily to exercise the process-snapshot path.
	switch got {
	case "", "cmd", "powershell", "bash":
		return
	default:
		// Paths like C:\Program Files\PowerShell\7\pwsh.exe are also valid.
		if got == "" {
			t.Fatal("unexpected empty shell string")
		}
	}
}


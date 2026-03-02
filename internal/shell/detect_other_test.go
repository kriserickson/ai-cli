//go:build !windows

package shell

import "testing"

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

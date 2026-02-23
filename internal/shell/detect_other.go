//go:build !windows

package shell

// parentShellProcess is a no-op on non-Windows platforms.
func parentShellProcess() string { return "" }

// preferredPowerShell is a no-op on non-Windows platforms.
func preferredPowerShell() string { return "powershell" }

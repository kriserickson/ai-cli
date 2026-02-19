package shell

import (
	"runtime"
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
		{"cmd", "cmd", []string{"/c"}},
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

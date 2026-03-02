package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_Valid(t *testing.T) {
	cwd := "/home/user/project"
	tests := []struct {
		path string
		want string
	}{
		{".", "/home/user/project"},
		{"src", "/home/user/project/src"},
		{"./src/main.go", "/home/user/project/src/main.go"},
		{"src/../src/main.go", "/home/user/project/src/main.go"},
	}
	for _, tt := range tests {
		got, err := ValidatePath(tt.path, cwd)
		if err != nil {
			t.Errorf("ValidatePath(%q, %q) error: %v", tt.path, cwd, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ValidatePath(%q, %q) = %q, want %q", tt.path, cwd, got, tt.want)
		}
	}
}

func TestValidatePath_OutsideCWD(t *testing.T) {
	cwd := "/home/user/project"
	paths := []string{
		"../other",
		"/etc/passwd",
		"../../..",
	}
	for _, path := range paths {
		_, err := ValidatePath(path, cwd)
		if err == nil {
			t.Errorf("ValidatePath(%q, %q) should have failed", path, cwd)
		}
		if !strings.Contains(err.Error(), "outside") {
			t.Errorf("ValidatePath(%q) error = %q, want 'outside'", path, err.Error())
		}
	}
}

func TestValidatePath_BlockedPatterns(t *testing.T) {
	cwd := "/home/user/project"
	blocked := []string{
		".ssh/id_rsa",
		".env",
		".env.production",
		"keys/server.pem",
		"keys/server.key",
		"cert.p12",
		"store.pfx",
		"id_rsa",
		"id_rsa.pub",
		"id_ed25519",
		"credentials.json",
		"secrets.yaml",
		"tokens.json",
		".netrc",
		".npmrc",
		"private.key",
		"app.keystore",
		"config.toml",
		".gnupg/pubring.gpg",
	}
	for _, path := range blocked {
		_, err := ValidatePath(path, cwd)
		if err == nil {
			t.Errorf("ValidatePath(%q) should have been blocked", path)
		}
		if !strings.Contains(err.Error(), "blocked") {
			t.Errorf("ValidatePath(%q) error = %q, want 'blocked'", path, err.Error())
		}
	}
}

func TestValidatePath_AllowedFiles(t *testing.T) {
	cwd := "/home/user/project"
	allowed := []string{
		"main.go",
		"src/app.js",
		"README.md",
		"Makefile",
	}
	for _, path := range allowed {
		_, err := ValidatePath(path, cwd)
		if err != nil {
			t.Errorf("ValidatePath(%q) should be allowed, got error: %v", path, err)
		}
	}
}

func TestValidatePath_Symlink(t *testing.T) {
	// Create a temporary directory to act as cwd
	cwd := t.TempDir()
	// Create a file outside cwd
	outside := t.TempDir()
	secretFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a symlink inside cwd pointing to the file outside cwd
	symlink := filepath.Join(cwd, "link.txt")
	if err := os.Symlink(secretFile, symlink); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, err := ValidatePath("link.txt", cwd)
	if err == nil {
		t.Error("ValidatePath with symlink escaping cwd should have failed")
	}
	if err != nil && !strings.Contains(err.Error(), "outside") {
		t.Errorf("expected 'outside' error, got: %v", err)
	}
}

func TestFilterEnvironment(t *testing.T) {
	vars := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
		"API_KEY=secret123",
		"AWS_SECRET_ACCESS_KEY=mysecret",
		"DATABASE_URL=postgres://localhost",
		"AUTH_TOKEN=tok123",
		"MY_PASSWORD=pass",
		"PRIVATE_DATA=stuff",
		"CREDENTIAL_FILE=/etc/creds",
		"NORMAL_VAR=hello",
	}

	filtered := FilterEnvironment(vars)

	expect := map[string]string{
		"HOME":                  "/home/user",
		"PATH":                  "/usr/bin",
		"API_KEY":               "[REDACTED]",
		"AWS_SECRET_ACCESS_KEY": "[REDACTED]",
		"DATABASE_URL":          "[REDACTED]",
		"AUTH_TOKEN":            "[REDACTED]",
		"MY_PASSWORD":           "[REDACTED]",
		"PRIVATE_DATA":          "[REDACTED]",
		"CREDENTIAL_FILE":       "[REDACTED]",
		"NORMAL_VAR":            "hello",
	}

	for _, v := range filtered {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			t.Errorf("bad format: %q", v)
			continue
		}
		if want, ok := expect[parts[0]]; ok {
			if parts[1] != want {
				t.Errorf("%s = %q, want %q", parts[0], parts[1], want)
			}
		}
	}
}

func TestValidatePath_SymlinkOutsideCWD(t *testing.T) {
	// Create a temporary directory to act as the project root (cwd).
	cwd, err := os.MkdirTemp("", "cwd-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cwd)

	// Create a real file outside the cwd.
	outside, err := os.MkdirTemp("", "outside-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)

	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside cwd pointing to the outside file.
	link := filepath.Join(cwd, "safe.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	_, err = ValidatePath("safe.txt", cwd)
	if err == nil {
		t.Error("ValidatePath should have rejected symlink pointing outside cwd")
	}
	if err != nil && !strings.Contains(err.Error(), "outside") {
		t.Errorf("expected 'outside' in error, got: %v", err)
	}
}

func TestValidatePath_SymlinkToBlockedFile(t *testing.T) {
	// Create a temporary directory to act as the project root (cwd).
	cwd, err := os.MkdirTemp("", "cwd-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cwd)

	// Create a blocked-pattern file inside cwd (e.g. id_rsa).
	blocked := filepath.Join(cwd, "id_rsa")
	if err := os.WriteFile(blocked, []byte("private key"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink with an innocuous name pointing to the blocked file.
	link := filepath.Join(cwd, "readme.txt")
	if err := os.Symlink(blocked, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	_, err = ValidatePath("readme.txt", cwd)
	if err == nil {
		t.Error("ValidatePath should have rejected symlink pointing to a blocked file")
	}
	if err != nil && !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'blocked' in error, got: %v", err)
	}
}

func TestValidatePath_SymlinkWithinCWD(t *testing.T) {
	// Create a temporary directory to act as the project root (cwd).
	cwd, err := os.MkdirTemp("", "cwd-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cwd)

	// Create a regular (non-blocked) file inside cwd.
	real := filepath.Join(cwd, "main.go")
	if err := os.WriteFile(real, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside cwd pointing to the file within cwd.
	link := filepath.Join(cwd, "alias.go")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	got, err := ValidatePath("alias.go", cwd)
	if err != nil {
		t.Errorf("ValidatePath should allow symlink within cwd, got error: %v", err)
	}
	if got != link {
		t.Errorf("ValidatePath returned %q, want %q", got, link)
	}
}

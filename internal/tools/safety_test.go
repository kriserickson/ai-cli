package tools

import (
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
		"DATABASE_URL":          "postgres://localhost",
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

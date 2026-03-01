package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// blockedPatterns lists file/directory patterns that must never be read or listed,
// even within the current working directory.
var blockedPatterns = []string{
	".ssh/",
	".gnupg/",
	".env",
	".env.",
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	"id_rsa",
	"id_ed25519",
	"credentials",
	"secrets",
	"tokens",
	".netrc",
	".npmrc",
	"shadow",
	"passwd",
	"private",
	"*.keystore",
	"config.toml",
}

// sensitiveEnvKeys lists substrings that, if found in an environment variable
// name, cause the value to be redacted.
var sensitiveEnvKeys = []string{
	"KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"CREDENTIAL",
	"AUTH",
	"PRIVATE",
}

// ValidatePath resolves path to an absolute path and checks that it is
// contained within cwd and does not match any blocked pattern.
func ValidatePath(path, cwd string) (string, error) {
	// Resolve relative to cwd
	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Clean(filepath.Join(cwd, path))
	}

	// Ensure the path is under cwd
	cwdClean := filepath.Clean(cwd)
	if abs != cwdClean && !strings.HasPrefix(abs, cwdClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside the working directory", path)
	}

	// Check blocked patterns against the relative path
	rel, err := filepath.Rel(cwdClean, abs)
	if err != nil {
		return "", fmt.Errorf("cannot compute relative path: %w", err)
	}

	if isBlocked(rel) {
		return "", fmt.Errorf("access to %q is blocked for security", path)
	}

	return abs, nil
}

// isBlocked checks if any component of the relative path matches a blocked pattern.
func isBlocked(relPath string) bool {
	// Normalize separators
	normalized := filepath.ToSlash(relPath)
	parts := strings.Split(normalized, "/")

	for _, pattern := range blockedPatterns {
		// Directory pattern (ends with /)
		if strings.HasSuffix(pattern, "/") {
			dirName := strings.TrimSuffix(pattern, "/")
			for _, part := range parts {
				if strings.EqualFold(part, dirName) {
					return true
				}
			}
			continue
		}

		// Check the final component (filename)
		filename := parts[len(parts)-1]

		// Glob-style pattern (starts with *)
		if strings.HasPrefix(pattern, "*") {
			suffix := pattern[1:]
			if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(suffix)) {
				return true
			}
			continue
		}

		// Prefix match (e.g., "id_rsa" matches "id_rsa", "id_rsa.pub")
		if strings.HasPrefix(strings.ToLower(filename), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// FilterEnvironment takes a list of "KEY=VALUE" strings and returns
// a filtered copy where sensitive values are replaced with [REDACTED].
func FilterEnvironment(vars []string) []string {
	result := make([]string, 0, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		upperKey := strings.ToUpper(key)
		for _, sensitive := range sensitiveEnvKeys {
			if strings.Contains(upperKey, sensitive) {
				value = "[REDACTED]"
				break
			}
		}
		result = append(result, key+"="+value)
	}
	return result
}

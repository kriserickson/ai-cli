package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	patternDotEnv  = ".env"
	patternIDRSA   = "id_rsa"
	redactedValue  = "[REDACTED]"
)

// blockedPatterns lists file/directory patterns that must never be read or listed,
// even within the current working directory.
var blockedPatterns = []string{
	".ssh/",
	".gnupg/",
	patternDotEnv,
	".env.",
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	patternIDRSA,
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
	"DATABASE",
	"DB",
	"DSN",
}

// safeEnvValueKeys lists environment variable names whose values are useful for
// command generation and generally non-secret. Unknown keys are shown by name
// only to avoid leaking poorly named secrets.
var safeEnvValueKeys = map[string]struct{}{
	"COLORTERM":    {},
	"COMPUTERNAME": {},
	"COMSPEC":      {},
	"EDITOR":       {},
	"HOME":         {},
	"HOSTNAME":     {},
	"LANG":         {},
	"NO_COLOR":     {},
	"OS":           {},
	"PATH":         {},
	"PATHEXT":      {},
	"PWD":          {},
	"SHELL":        {},
	"SYSTEMROOT":   {},
	"TEMP":         {},
	"TERM":         {},
	"TMP":          {},
	"TZ":           {},
	"USER":         {},
	"USERNAME":     {},
	"VISUAL":       {},
	"WINDIR":       {},
}

// ValidatePath resolves path to an absolute path and checks that it is
// contained within cwd and does not match any blocked pattern.
// It also resolves symlinks and re-validates the resolved path to prevent
// symlink attacks that could point outside cwd or to blocked files.
func ValidatePath(path, cwd string) (string, error) {
	cwdClean := filepath.Clean(cwd)
	cwdResolved := cwdClean
	if resolved, err := filepath.EvalSymlinks(cwdClean); err == nil {
		cwdResolved = filepath.Clean(resolved)
	}

	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Clean(filepath.Join(cwdClean, path))
	}

	// Allow either the lexical cwd or its resolved path to support symlinked
	// temp roots such as /var -> /private/var on macOS.
	if !withinPath(abs, cwdClean) && !withinPath(abs, cwdResolved) {
		return "", fmt.Errorf("path %q is outside the working directory", path)
	}

	rel, err := relativeToBase(abs, cwdClean, cwdResolved)
	if err != nil {
		return "", fmt.Errorf("cannot compute relative path: %w", err)
	}
	if isBlocked(rel) {
		return "", fmt.Errorf("access to %q is blocked for security", path)
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		resolvedClean := filepath.Clean(resolved)
		if !withinPath(resolvedClean, cwdResolved) {
			return "", fmt.Errorf("path %q is outside the working directory", path)
		}
		resolvedRel, relErr := relativeToBase(resolvedClean, cwdResolved, cwdClean)
		if relErr != nil {
			return "", fmt.Errorf("cannot compute relative path: %w", relErr)
		}
		if isBlocked(resolvedRel) {
			return "", fmt.Errorf("access to %q is blocked for security", path)
		}
		return resolvedClean, nil
	}

	return abs, nil
}

func withinPath(path, base string) bool {
	pathClean := filepath.Clean(path)
	baseClean := filepath.Clean(base)
	if pathClean == baseClean {
		return true
	}
	if len(pathClean) <= len(baseClean) {
		return false
	}
	if !strings.HasPrefix(pathClean, baseClean) {
		return false
	}
	return os.IsPathSeparator(pathClean[len(baseClean)])
}

func relativeToBase(path string, bases ...string) (string, error) {
	for _, base := range bases {
		if base == "" || !withinPath(path, base) {
			continue
		}
		return filepath.Rel(base, path)
	}
	return "", fmt.Errorf("path %q is outside the working directory", path)
}

// isBlocked checks if any component of the relative path matches a blocked pattern.
func isBlocked(relPath string) bool {
	normalized := filepath.ToSlash(relPath)
	parts := strings.Split(normalized, "/")

	for _, pattern := range blockedPatterns {
		if strings.HasSuffix(pattern, "/") {
			dirName := strings.TrimSuffix(pattern, "/")
			for _, part := range parts {
				if strings.EqualFold(part, dirName) {
					return true
				}
			}
			continue
		}

		filename := parts[len(parts)-1]

		if strings.HasPrefix(pattern, "*") {
			suffix := pattern[1:]
			if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(suffix)) {
				return true
			}
			continue
		}

		if strings.HasPrefix(strings.ToLower(filename), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// FilterEnvironment takes a list of "KEY=VALUE" strings and returns a filtered
// copy where only a safe allowlist of values is shown. Sensitive values are
// replaced with [REDACTED]; all other values are hidden as [VALUE HIDDEN].
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
		valueShown := false
		for _, sensitive := range sensitiveEnvKeys {
			if strings.Contains(upperKey, sensitive) {
				value = redactedValue
				valueShown = true
				break
			}
		}
		if !valueShown {
			if _, ok := safeEnvValueKeys[upperKey]; !ok {
				value = "[VALUE HIDDEN]"
			}
		}
		result = append(result, key+"="+value)
	}
	return result
}

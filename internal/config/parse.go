package config

import (
	"fmt"
	"strings"
)

// ParseBool parses a string as a boolean, accepting "true" or "false"
// (case-insensitive, trimmed). Any other value returns an error.
//
// This is stricter than strconv.ParseBool — it rejects "1", "0", "yes", "no",
// etc. — because config values come from LLM responses and user input where
// only "true"/"false" are documented as valid.
func ParseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("must be 'true' or 'false', got %q", s)
	}
}

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	boolStringTrue  = "true"
	boolStringFalse = "false"
)

// ParseBool parses a string as a boolean, accepting "true" or "false"
// (case-insensitive, trimmed). Any other value returns an error.
//
// This is stricter than strconv.ParseBool — it rejects "1", "0", "yes", "no",
// etc. — because config values come from LLM responses and user input where
// only "true"/"false" are documented as valid.
func ParseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case boolStringTrue:
		return true, nil
	case boolStringFalse:
		return false, nil
	default:
		return false, fmt.Errorf("must be 'true' or 'false', got %q", s)
	}
}

// ParseModelParameters parses a JSON object used for provider request parameters.
// Blank input and an empty object both mean provider defaults.
func ParseModelParameters(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return nil, nil
	}
	var parameters map[string]any
	if err := json.Unmarshal([]byte(s), &parameters); err != nil {
		return nil, fmt.Errorf("must be a JSON object: %w", err)
	}
	if parameters == nil {
		return nil, errors.New("must be a JSON object")
	}
	for _, reserved := range []string{"model", "messages"} {
		if _, exists := parameters[reserved]; exists {
			return nil, fmt.Errorf("parameter %q is managed by ai-cli and cannot be overridden", reserved)
		}
	}
	return parameters, nil
}

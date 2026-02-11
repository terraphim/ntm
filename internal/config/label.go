package config

import (
	"fmt"
	"regexp"
	"strings"
)

// labelRegex validates label strings: must start with alphanumeric,
// then alphanumeric, dash, or underscore.
var labelRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ParseSessionLabel splits a session name into base project name and optional label.
// The separator is the FIRST occurrence of "--" (double-dash).
// Returns (base, label) where label is empty if no "--" present.
//
// Examples:
//
//	ParseSessionLabel("myproject")            -> ("myproject", "")
//	ParseSessionLabel("my-project--frontend") -> ("my-project", "frontend")
//	ParseSessionLabel("foo--bar--baz")        -> ("foo", "bar--baz")
func ParseSessionLabel(sessionName string) (base, label string) {
	idx := strings.Index(sessionName, "--")
	if idx < 0 {
		return sessionName, ""
	}
	return sessionName[:idx], sessionName[idx+2:]
}

// FormatSessionName constructs a labeled session name from base and label.
// If label is empty, returns base unchanged.
func FormatSessionName(base, label string) string {
	if label == "" {
		return base
	}
	return base + "--" + label
}

// HasLabel returns true if the session name contains a label separator.
func HasLabel(sessionName string) bool {
	return strings.Contains(sessionName, "--")
}

// SessionBase returns just the base project name, stripping any label.
// Convenience wrapper around ParseSessionLabel.
//
// Examples:
//
//	SessionBase("myproject")            -> "myproject"
//	SessionBase("myproject--frontend")  -> "myproject"
//	SessionBase("my-project--frontend") -> "my-project"
func SessionBase(sessionName string) string {
	base, _ := ParseSessionLabel(sessionName)
	return base
}

// ValidateLabel checks if a label string is valid.
// Rules:
//   - Must start with alphanumeric character
//   - Can contain alphanumeric, dash, underscore
//   - Must not be empty
//   - Must not exceed 50 characters
//   - Must not itself contain "--" (the separator)
func ValidateLabel(label string) error {
	if label == "" {
		return fmt.Errorf("label must not be empty")
	}
	if len(label) > 50 {
		return fmt.Errorf("label must not exceed 50 characters (got %d)", len(label))
	}
	if strings.Contains(label, "--") {
		return fmt.Errorf("label must not contain '--' (reserved as separator)")
	}
	if !labelRegex.MatchString(label) {
		return fmt.Errorf("label must start with alphanumeric and contain only alphanumeric, dash, underscore")
	}
	return nil
}

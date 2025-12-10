package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func stripANSI(str string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansi.ReplaceAllString(str, "")
}

func TestPrintStunningHelp(t *testing.T) {
	// Use buffer instead of stdout
	var buf bytes.Buffer

	// Run function with buffer
	PrintStunningHelp(&buf)

	// Read output
	output := stripANSI(buf.String())

	// Verify key components exist
	expected := []string{
		"Named Tmux Session Manager for AI Agents", // Subtitle
		"SESSION CREATION",                         // Section 1
		"AGENT MANAGEMENT",                         // Section 2
		"spawn",                                    // Command
		"Create session and launch agents",         // Description
		"Aliases:",                                 // Footer
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("Expected help output to contain %q, but it didn't", exp)
		}
	}
}

func TestPrintCompactHelp(t *testing.T) {
	// Use buffer instead of stdout
	var buf bytes.Buffer

	// Run function with buffer
	PrintCompactHelp(&buf)

	// Read output
	output := stripANSI(buf.String())

	// Verify key components exist
	expected := []string{
		"NTM - Named Tmux Manager",
		"Commands:",
		"spawn",
		"Send prompts to agents",
		"Shell setup:",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("Expected compact help output to contain %q, but it didn't", exp)
		}
	}
}

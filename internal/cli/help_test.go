package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func stripANSIForTest(str string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansi.ReplaceAllString(str, "")
}

func TestPrintStunningHelp(t *testing.T) {
	// Use buffer instead of stdout
	var buf bytes.Buffer

	// Run function with buffer
	PrintStunningHelp(&buf)

	// Read output
	output := stripANSIForTest(buf.String())

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
	output := stripANSIForTest(buf.String())

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

func TestPrintMinimalHelp(t *testing.T) {
	// Use buffer instead of stdout
	var buf bytes.Buffer

	// Run function with buffer
	PrintMinimalHelp(&buf)

	// Read output
	output := stripANSIForTest(buf.String())

	// Verify key components exist
	expected := []string{
		"Named Tmux Session Manager for AI Agents", // Subtitle
		"ESSENTIAL COMMANDS",                       // Section header
		"spawn",                                    // Essential command
		"send",                                     // Essential command
		"status",                                   // Essential command
		"kill",                                     // Essential command
		"help",                                     // Essential command
		"Create session and launch agents",         // spawn description
		"Send prompt to agents",                    // send description
		"Show detailed session status",             // status description
		"Kill a session",                          // kill description
		"Show help information",                   // help description
		"QUICK START",                             // Quick start section
		"For all commands: ntm --full",            // Instructions for full help
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("Expected minimal help output to contain %q, but it didn't", exp)
		}
	}

	// Verify that non-essential commands are NOT present
	shouldNotContain := []string{
		"SESSION CREATION",  // This is from full help
		"AGENT MANAGEMENT",  // This is from full help
		"create",           // Non-essential command
		"quick",            // Non-essential command
		"add",              // Non-essential command
		"interrupt",        // Non-essential command
	}

	for _, notExp := range shouldNotContain {
		if strings.Contains(output, notExp) {
			t.Errorf("Expected minimal help output to NOT contain %q, but it did", notExp)
		}
	}
}

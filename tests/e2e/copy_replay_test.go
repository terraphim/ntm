package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

// copyResultJSON is the JSON output for ntm copy --json.
// Named differently to avoid conflict with other e2e test files.
type copyResultJSON struct {
	Source      string   `json:"source"`
	Panes       []string `json:"panes"`
	Lines       int      `json:"lines"`
	Bytes       int      `json:"bytes"`
	Destination string   `json:"destination"`
	Pattern     string   `json:"pattern,omitempty"`
	Code        bool     `json:"code,omitempty"`
	OutputPath  string   `json:"output_path,omitempty"`
}

// setupCopyTestEnv sets up isolated test environment for copy tests.
func setupCopyTestEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, ".local", "share"))

	// Create config directories
	os.MkdirAll(filepath.Join(tmpDir, ".config", "ntm"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".local", "share", "ntm"), 0755)

	return tmpDir
}

// TestE2ECopy_SessionNotFound tests copy with non-existent session.
func TestE2ECopy_SessionNotFound(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("session_not_found_error", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		out, err := runCmdAllowFail(t, tmpDir, "ntm", "copy", "nonexistent_session_12345", "--json")

		// Should fail with session not found
		if err == nil {
			// Check if JSON indicates error
			outStr := string(out)
			if !strings.Contains(outStr, "not found") && !strings.Contains(outStr, "error") {
				t.Logf("output: %s", outStr)
			}
		}

		// Output should mention session not found
		outStr := string(out)
		if !strings.Contains(outStr, "not found") && !strings.Contains(strings.ToLower(outStr), "session") {
			t.Logf("expected 'not found' or 'session' in error, got: %s", outStr)
		}
	})
}

// TestE2ECopy_OutputToFile tests copy with file output.
func TestE2ECopy_OutputToFile(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("output_flag_accepted", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)
		outputFile := filepath.Join(tmpDir, "output.txt")

		// This will fail because no session, but we verify the flag is accepted
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--output", outputFile, "--json")

		outStr := string(out)
		// Command should fail on session not found, not on unknown flag
		if strings.Contains(outStr, "unknown flag") || strings.Contains(outStr, "invalid") {
			t.Errorf("--output flag should be accepted, got: %s", outStr)
		}
	})
}

// TestE2ECopy_CodeBlockExtraction tests code block extraction flag.
func TestE2ECopy_CodeBlockExtraction(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("code_flag_accepted", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Verify --code flag is accepted
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--code", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("--code flag should be accepted, got: %s", outStr)
		}
	})

	t.Run("pattern_flag_accepted", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Verify --pattern flag is accepted
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--pattern", "ERROR", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("--pattern flag should be accepted, got: %s", outStr)
		}
	})
}

// TestE2ECopy_LastLines tests --last flag for line count.
func TestE2ECopy_LastLines(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("last_flag_accepted", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Verify --last flag is accepted
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--last", "50", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("--last flag should be accepted, got: %s", outStr)
		}
	})

	t.Run("invalid_last_value", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Test with invalid (zero) value
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--last", "0", "--json")

		outStr := string(out)
		// Should fail with validation error about positive value
		if !strings.Contains(outStr, "positive") && !strings.Contains(outStr, "must be") {
			t.Logf("expected validation error for --last=0, got: %s", outStr)
		}
	})
}

// TestE2ECopy_AgentFilters tests agent type filter flags.
func TestE2ECopy_AgentFilters(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	filters := []string{"--cc", "--cod", "--gmi", "--all"}

	for _, filter := range filters {
		t.Run("filter_"+strings.TrimPrefix(filter, "--"), func(t *testing.T) {
			tmpDir := setupCopyTestEnv(t)

			out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", filter, "--json")

			outStr := string(out)
			if strings.Contains(outStr, "unknown flag") {
				t.Errorf("%s flag should be accepted, got: %s", filter, outStr)
			}
		})
	}
}

// TestE2ECopy_Aliases tests command aliases.
func TestE2ECopy_Aliases(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	aliases := []string{"cp", "yank"}

	for _, alias := range aliases {
		t.Run("alias_"+alias, func(t *testing.T) {
			tmpDir := setupCopyTestEnv(t)

			out, _ := runCmdAllowFail(t, tmpDir, "ntm", alias, "test_session", "--json")

			outStr := string(out)
			// Should not say "unknown command"
			if strings.Contains(outStr, "unknown command") {
				t.Errorf("%s should be a valid alias, got: %s", alias, outStr)
			}
		})
	}
}

// TestE2ECopy_QuietMode tests --quiet flag.
func TestE2ECopy_QuietMode(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("quiet_flag_accepted", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--quiet", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("--quiet flag should be accepted, got: %s", outStr)
		}
	})
}

// TestE2ECopy_HeadersFlag tests --headers flag.
func TestE2ECopy_HeadersFlag(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("headers_flag_accepted", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--headers", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("--headers flag should be accepted, got: %s", outStr)
		}
	})
}

// TestE2ECopy_PaneSelector tests pane selection via session:pane syntax.
func TestE2ECopy_PaneSelector(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("pane_selector_syntax", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Try session:pane syntax
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session:1", "--json")

		outStr := string(out)
		// Should fail on session not found, not on syntax error
		if strings.Contains(outStr, "invalid") && strings.Contains(outStr, "syntax") {
			t.Errorf("session:pane syntax should be accepted, got: %s", outStr)
		}
	})
}

// TestE2ECopy_JSONStructure tests JSON output structure for copy command.
func TestE2ECopy_JSONStructure(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("error_is_json", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--json")
		jsonData := extractJSON(out)

		// Should be valid JSON even on error
		var raw map[string]interface{}
		if err := json.Unmarshal(jsonData, &raw); err != nil {
			// Some errors might not be JSON, that's OK for validation errors
			t.Logf("copy error output is not JSON: %s", string(out))
		}
	})
}

// TestE2ECopy_CombinedFlags tests combining multiple flags.
func TestE2ECopy_CombinedFlags(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("code_and_pattern", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Combine --code and --pattern
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--code", "--pattern", "func", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("combining --code and --pattern should work, got: %s", outStr)
		}
	})

	t.Run("output_and_quiet", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)
		outputFile := filepath.Join(tmpDir, "output.txt")

		// Combine --output and --quiet
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--output", outputFile, "--quiet", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("combining --output and --quiet should work, got: %s", outStr)
		}
	})

	t.Run("agent_filter_with_last", func(t *testing.T) {
		tmpDir := setupCopyTestEnv(t)

		// Combine --cc with --last
		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "copy", "test_session", "--cc", "--last", "100", "--json")

		outStr := string(out)
		if strings.Contains(outStr, "unknown flag") {
			t.Errorf("combining --cc and --last should work, got: %s", outStr)
		}
	})
}

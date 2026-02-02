package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

// robotRestartPaneResponse is the JSON output for ntm --robot-restart-pane.
type robotRestartPaneResponse struct {
	Success      bool           `json:"success"`
	Timestamp    string         `json:"timestamp"`
	Version      string         `json:"version"`
	OutputFormat string         `json:"output_format"`
	Session      string         `json:"session"`
	RestartedAt  string         `json:"restarted_at"`
	Restarted    []string       `json:"restarted"`
	Failed       []restartError `json:"failed"`
	DryRun       bool           `json:"dry_run,omitempty"`
	WouldAffect  []string       `json:"would_affect,omitempty"`
	BeadAssigned string         `json:"bead_assigned,omitempty"`
	PromptSent   bool           `json:"prompt_sent,omitempty"`
	PromptError  string         `json:"prompt_error,omitempty"`
	Error        string         `json:"error,omitempty"`
	ErrorCode    string         `json:"error_code,omitempty"`
	Hint         string         `json:"hint,omitempty"`
}

type restartError struct {
	Pane   string `json:"pane"`
	Reason string `json:"reason"`
}

func runRobotRestartPane(t *testing.T, dir, session string, args ...string) (robotRestartPaneResponse, error) {
	t.Helper()
	cmdArgs := []string{"--robot-restart-pane=" + session}
	cmdArgs = append(cmdArgs, args...)
	out, err := runCmdAllowFail(t, dir, "ntm", cmdArgs...)
	var resp robotRestartPaneResponse
	json.Unmarshal(extractJSON(out), &resp)
	return resp, err
}

// TestE2ERobotRestartPane_SessionNotFound tests error handling for non-existent sessions.
func TestE2ERobotRestartPane_SessionNotFound(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("nonexistent_session_error", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "nonexistent_session_12345", "--panes=1")

		// Should return success=false
		if resp.Success {
			t.Errorf("expected success=false for nonexistent session")
		}

		// Error should be set
		if resp.Error == "" {
			t.Errorf("expected error message to be set")
		}

		// Error code should be SESSION_NOT_FOUND
		if resp.ErrorCode != "SESSION_NOT_FOUND" {
			t.Errorf("expected error_code='SESSION_NOT_FOUND', got '%s'", resp.ErrorCode)
		}

		// Hint should suggest using robot-status
		if !strings.Contains(resp.Hint, "status") {
			t.Errorf("expected hint to suggest status, got '%s'", resp.Hint)
		}

		// Session name should still be in response
		if resp.Session != "nonexistent_session_12345" {
			t.Errorf("expected session='nonexistent_session_12345', got '%s'", resp.Session)
		}

		// Failed array should have an entry
		if len(resp.Failed) == 0 {
			t.Errorf("expected failed array to have entries")
		}
	})
}

// TestE2ERobotRestartPane_JSONStructure tests JSON response structure.
func TestE2ERobotRestartPane_JSONStructure(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("required_fields_present", func(t *testing.T) {
		tmpDir := t.TempDir()

		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "--robot-restart-pane=test_session", "--panes=1")
		jsonData := extractJSON(out)

		// Parse as raw JSON to check field presence
		var raw map[string]interface{}
		if err := json.Unmarshal(jsonData, &raw); err != nil {
			t.Fatalf("unmarshal raw json: %v\nout=%s", err, string(out))
		}

		// Check required top-level fields
		requiredFields := []string{"success", "timestamp", "version", "output_format", "session", "restarted_at", "restarted", "failed"}
		for _, field := range requiredFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("missing required field: %s", field)
			}
		}
	})

	t.Run("arrays_not_null", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "test_session", "--panes=1")

		// Arrays should never be nil (per envelope spec)
		if resp.Restarted == nil {
			t.Errorf("restarted should be empty array, not null")
		}
		if resp.Failed == nil {
			t.Errorf("failed should be empty array, not null")
		}
	})

	t.Run("timestamp_format", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "test_session", "--panes=1")

		// Timestamp should be RFC3339 format
		if resp.Timestamp == "" {
			t.Errorf("expected timestamp to be set")
		}
		if resp.RestartedAt == "" {
			t.Errorf("expected restarted_at to be set")
		}

		// Should contain date markers
		if !strings.Contains(resp.Timestamp, "-") || !strings.Contains(resp.Timestamp, "T") {
			t.Errorf("timestamp doesn't look like RFC3339: %s", resp.Timestamp)
		}
	})
}

// TestE2ERobotRestartPane_DryRun tests dry run mode.
func TestE2ERobotRestartPane_DryRun(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("dry_run_flag_accepted", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Dry run should still fail for nonexistent session (validates input)
		resp, _ := runRobotRestartPane(t, tmpDir, "test_session", "--dry-run", "--panes=1")

		// Session not found, so dry_run field may or may not be set
		// Just verify command accepted the flag (didn't error on unknown flag)
		if resp.Error != "" && !strings.Contains(resp.Error, "session") {
			t.Errorf("unexpected error: %s", resp.Error)
		}
	})
}

// TestE2ERobotRestartPane_ErrorResponse tests error response format.
func TestE2ERobotRestartPane_ErrorResponse(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("error_response_format", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "nonexistent_session", "--panes=1")

		// Error response should still include base robot response fields
		if resp.Timestamp == "" {
			t.Errorf("expected timestamp even in error response")
		}

		if resp.Version == "" {
			t.Errorf("expected version even in error response")
		}

		// Should have error details
		if resp.Error == "" {
			t.Errorf("expected error message")
		}

		// Hint should be helpful
		if resp.Hint == "" {
			t.Errorf("expected hint for error resolution")
		}
	})

	t.Run("failed_array_has_reason", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "nonexistent_session", "--panes=1")

		if len(resp.Failed) > 0 {
			fail := resp.Failed[0]
			if fail.Reason == "" {
				t.Errorf("expected failed entry to have reason")
			}
		}
	})
}

// TestE2ERobotRestartPane_OutputFormat tests output format consistency.
func TestE2ERobotRestartPane_OutputFormat(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("output_format_json", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "test_session", "--panes=1")

		if resp.OutputFormat != "json" {
			t.Errorf("expected output_format='json', got '%s'", resp.OutputFormat)
		}
	})
}

// TestE2ERobotRestartPane_BeadOption tests bead assignment option validation.
func TestE2ERobotRestartPane_BeadOption(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("invalid_bead_error", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Try to assign a nonexistent bead
		resp, _ := runRobotRestartPane(t, tmpDir, "test_session", "--panes=1", "--restart-bead=bd-nonexistent")

		// Should fail early due to invalid bead
		if resp.Success {
			t.Errorf("expected success=false for invalid bead")
		}

		// Error should mention bead
		if resp.Error != "" && !strings.Contains(resp.Error, "not found") && !strings.Contains(resp.Error, "Bead") && !strings.Contains(resp.Error, "bead") {
			t.Logf("error message: %s (may still be valid)", resp.Error)
		}
	})
}

// TestE2ERobotRestartPane_SessionField tests session field consistency.
func TestE2ERobotRestartPane_SessionField(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("session_echoed_back", func(t *testing.T) {
		tmpDir := t.TempDir()

		sessionName := "my_test_session_123"
		resp, _ := runRobotRestartPane(t, tmpDir, sessionName, "--panes=1")

		// Session should be echoed back in response
		if resp.Session != sessionName {
			t.Errorf("expected session='%s', got '%s'", sessionName, resp.Session)
		}
	})
}

// TestE2ERobotRestartPane_VersionConsistency tests version field.
func TestE2ERobotRestartPane_VersionConsistency(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("version_set", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotRestartPane(t, tmpDir, "test_session", "--panes=1")

		if resp.Version == "" {
			t.Errorf("expected version to be set")
		}

		// Version should be semver-like
		if !strings.Contains(resp.Version, ".") {
			t.Errorf("version doesn't look like semver: %s", resp.Version)
		}
	})
}

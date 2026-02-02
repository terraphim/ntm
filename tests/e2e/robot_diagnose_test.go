package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

// robotDiagnoseResponse is the JSON output for ntm --robot-diagnose.
type robotDiagnoseResponse struct {
	Success         bool                     `json:"success"`
	Timestamp       string                   `json:"timestamp"`
	Version         string                   `json:"version"`
	OutputFormat    string                   `json:"output_format"`
	Session         string                   `json:"session"`
	OverallHealth   string                   `json:"overall_health"`
	Summary         diagnoseSummary          `json:"summary"`
	Panes           diagnosePanes            `json:"panes"`
	Recommendations []diagnoseRecommendation `json:"recommendations"`
	AutoFixAvail    bool                     `json:"auto_fix_available"`
	AutoFixCommand  string                   `json:"auto_fix_command,omitempty"`
	Error           string                   `json:"error,omitempty"`
	ErrorCode       string                   `json:"error_code,omitempty"`
	Hint            string                   `json:"hint,omitempty"`
}

type diagnoseSummary struct {
	TotalPanes   int `json:"total_panes"`
	Healthy      int `json:"healthy"`
	Degraded     int `json:"degraded"`
	RateLimited  int `json:"rate_limited"`
	Unresponsive int `json:"unresponsive"`
	Crashed      int `json:"crashed"`
	Unknown      int `json:"unknown"`
}

type diagnosePanes struct {
	Healthy      []int `json:"healthy"`
	Degraded     []int `json:"degraded"`
	RateLimited  []int `json:"rate_limited"`
	Unresponsive []int `json:"unresponsive"`
	Crashed      []int `json:"crashed"`
	Unknown      []int `json:"unknown"`
}

type diagnoseRecommendation struct {
	Pane        int    `json:"pane"`
	Status      string `json:"status"`
	Action      string `json:"action"`
	Reason      string `json:"reason"`
	AutoFixable bool   `json:"auto_fixable"`
	FixCommand  string `json:"fix_command"`
}

func runRobotDiagnose(t *testing.T, dir, session string, args ...string) (robotDiagnoseResponse, error) {
	t.Helper()
	cmdArgs := []string{"--robot-diagnose=" + session}
	cmdArgs = append(cmdArgs, args...)
	out, err := runCmdAllowFail(t, dir, "ntm", cmdArgs...)
	var resp robotDiagnoseResponse
	json.Unmarshal(extractJSON(out), &resp)
	return resp, err
}

// TestE2ERobotDiagnose_SessionNotFound tests error handling for non-existent sessions.
func TestE2ERobotDiagnose_SessionNotFound(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("nonexistent_session_error", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "nonexistent_session_12345")

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

		// Hint should suggest using ntm list
		if !strings.Contains(resp.Hint, "list") {
			t.Errorf("expected hint to suggest 'ntm list', got '%s'", resp.Hint)
		}

		// Session name should still be in response
		if resp.Session != "nonexistent_session_12345" {
			t.Errorf("expected session='nonexistent_session_12345', got '%s'", resp.Session)
		}
	})
}

// TestE2ERobotDiagnose_JSONStructure tests JSON response structure.
func TestE2ERobotDiagnose_JSONStructure(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("required_fields_present", func(t *testing.T) {
		tmpDir := t.TempDir()

		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "--robot-diagnose=test_session")
		jsonData := extractJSON(out)

		// Parse as raw JSON to check field presence
		var raw map[string]interface{}
		if err := json.Unmarshal(jsonData, &raw); err != nil {
			t.Fatalf("unmarshal raw json: %v\nout=%s", err, string(out))
		}

		// Check required top-level fields
		requiredFields := []string{"success", "timestamp", "version", "output_format", "session", "overall_health", "summary", "panes", "recommendations", "auto_fix_available"}
		for _, field := range requiredFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("missing required field: %s", field)
			}
		}

		// Check summary subfields
		if summary, ok := raw["summary"].(map[string]interface{}); ok {
			summaryFields := []string{"total_panes", "healthy", "degraded", "rate_limited", "unresponsive", "crashed", "unknown"}
			for _, field := range summaryFields {
				if _, ok := summary[field]; !ok {
					t.Errorf("missing summary.%s field", field)
				}
			}
		} else {
			t.Errorf("summary field is not an object")
		}

		// Check panes subfields
		if panes, ok := raw["panes"].(map[string]interface{}); ok {
			panesFields := []string{"healthy", "degraded", "rate_limited", "unresponsive", "crashed", "unknown"}
			for _, field := range panesFields {
				if _, ok := panes[field]; !ok {
					t.Errorf("missing panes.%s field", field)
				}
			}
		} else {
			t.Errorf("panes field is not an object")
		}
	})

	t.Run("arrays_not_null", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// Arrays should never be nil (per envelope spec)
		if resp.Panes.Healthy == nil {
			t.Errorf("panes.healthy should be empty array, not null")
		}
		if resp.Panes.Degraded == nil {
			t.Errorf("panes.degraded should be empty array, not null")
		}
		if resp.Panes.RateLimited == nil {
			t.Errorf("panes.rate_limited should be empty array, not null")
		}
		if resp.Panes.Unresponsive == nil {
			t.Errorf("panes.unresponsive should be empty array, not null")
		}
		if resp.Panes.Crashed == nil {
			t.Errorf("panes.crashed should be empty array, not null")
		}
		if resp.Panes.Unknown == nil {
			t.Errorf("panes.unknown should be empty array, not null")
		}
		if resp.Recommendations == nil {
			t.Errorf("recommendations should be empty array, not null")
		}
	})

	t.Run("overall_health_valid", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// overall_health should be one of: healthy, degraded, critical
		validHealth := map[string]bool{
			"healthy":  true,
			"degraded": true,
			"critical": true,
		}

		if !validHealth[resp.OverallHealth] {
			t.Errorf("overall_health='%s' is not valid (expected: healthy, degraded, critical)", resp.OverallHealth)
		}
	})
}

// TestE2ERobotDiagnose_ErrorResponse tests error response format.
func TestE2ERobotDiagnose_ErrorResponse(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("error_response_format", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "nonexistent_session")

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
}

// TestE2ERobotDiagnose_SummaryConsistency tests summary field consistency.
func TestE2ERobotDiagnose_SummaryConsistency(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("summary_counts_consistent", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// Summary counts should add up to total_panes
		counted := resp.Summary.Healthy + resp.Summary.Degraded +
			resp.Summary.RateLimited + resp.Summary.Unresponsive +
			resp.Summary.Crashed + resp.Summary.Unknown

		if counted != resp.Summary.TotalPanes {
			t.Errorf("summary counts (%d) don't add up to total_panes (%d)", counted, resp.Summary.TotalPanes)
		}
	})

	t.Run("panes_arrays_match_summary", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// Pane arrays should have lengths matching summary counts
		if len(resp.Panes.Healthy) != resp.Summary.Healthy {
			t.Errorf("panes.healthy length (%d) != summary.healthy (%d)",
				len(resp.Panes.Healthy), resp.Summary.Healthy)
		}
		if len(resp.Panes.Degraded) != resp.Summary.Degraded {
			t.Errorf("panes.degraded length (%d) != summary.degraded (%d)",
				len(resp.Panes.Degraded), resp.Summary.Degraded)
		}
		if len(resp.Panes.RateLimited) != resp.Summary.RateLimited {
			t.Errorf("panes.rate_limited length (%d) != summary.rate_limited (%d)",
				len(resp.Panes.RateLimited), resp.Summary.RateLimited)
		}
		if len(resp.Panes.Unresponsive) != resp.Summary.Unresponsive {
			t.Errorf("panes.unresponsive length (%d) != summary.unresponsive (%d)",
				len(resp.Panes.Unresponsive), resp.Summary.Unresponsive)
		}
		if len(resp.Panes.Crashed) != resp.Summary.Crashed {
			t.Errorf("panes.crashed length (%d) != summary.crashed (%d)",
				len(resp.Panes.Crashed), resp.Summary.Crashed)
		}
		if len(resp.Panes.Unknown) != resp.Summary.Unknown {
			t.Errorf("panes.unknown length (%d) != summary.unknown (%d)",
				len(resp.Panes.Unknown), resp.Summary.Unknown)
		}
	})
}

// TestE2ERobotDiagnose_AutoFixField tests auto_fix field behavior.
func TestE2ERobotDiagnose_AutoFixField(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("auto_fix_available_boolean", func(t *testing.T) {
		tmpDir := t.TempDir()

		out, _ := runCmdAllowFail(t, tmpDir, "ntm", "--robot-diagnose=test_session")
		jsonData := extractJSON(out)

		// Parse as raw JSON
		var raw map[string]interface{}
		if err := json.Unmarshal(jsonData, &raw); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		// auto_fix_available should be a boolean
		if _, ok := raw["auto_fix_available"].(bool); !ok {
			t.Errorf("auto_fix_available should be a boolean")
		}
	})

	t.Run("auto_fix_command_conditional", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// If auto_fix_available is false, auto_fix_command should be empty
		if !resp.AutoFixAvail && resp.AutoFixCommand != "" {
			t.Errorf("auto_fix_command should be empty when auto_fix_available=false")
		}
	})
}

// TestE2ERobotDiagnose_RecommendationStructure tests recommendation fields.
func TestE2ERobotDiagnose_RecommendationStructure(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("empty_recommendations_valid", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// Empty recommendations array is valid for healthy or error states
		if resp.Recommendations == nil {
			t.Errorf("recommendations should be empty array, not null")
		}

		// If session not found, should have no recommendations
		if !resp.Success && len(resp.Recommendations) > 0 {
			t.Errorf("error response should not have recommendations")
		}
	})
}

// TestE2ERobotDiagnose_OutputFormat tests output format consistency.
func TestE2ERobotDiagnose_OutputFormat(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("output_format_json", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		if resp.OutputFormat != "json" {
			t.Errorf("expected output_format='json', got '%s'", resp.OutputFormat)
		}
	})

	t.Run("timestamp_format", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotDiagnose(t, tmpDir, "test_session")

		// Timestamp should be RFC3339 format
		if resp.Timestamp == "" {
			t.Errorf("expected timestamp to be set")
		}

		// Should contain date markers
		if !strings.Contains(resp.Timestamp, "-") || !strings.Contains(resp.Timestamp, "T") {
			t.Errorf("timestamp doesn't look like RFC3339: %s", resp.Timestamp)
		}
	})
}

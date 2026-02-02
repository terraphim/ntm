package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

// robotEnvResponse is the JSON output for ntm --robot-env.
type robotEnvResponse struct {
	Success          bool               `json:"success"`
	Timestamp        string             `json:"timestamp"`
	Version          string             `json:"version"`
	OutputFormat     string             `json:"output_format"`
	Error            string             `json:"error,omitempty"`
	ErrorCode        string             `json:"error_code,omitempty"`
	Hint             string             `json:"hint,omitempty"`
	Session          string             `json:"session"`
	Tmux             tmuxEnvInfo        `json:"tmux"`
	SessionStructure *sessionStructInfo `json:"session_structure,omitempty"`
	Shell            *shellEnvInfo      `json:"shell,omitempty"`
	Timing           *timingInfo        `json:"timing,omitempty"`
	Targeting        *targetingInfo     `json:"targeting,omitempty"`
}

type tmuxEnvInfo struct {
	BinaryPath         string `json:"binary_path"`
	Version            string `json:"version"`
	ShellAliasDetected bool   `json:"shell_alias_detected"`
	RecommendedPath    string `json:"recommended_path"`
	Warning            string `json:"warning,omitempty"`
	OhMyZshTmuxPlugin  bool   `json:"oh_my_zsh_tmux_plugin"`
	TmuxinatorDetected bool   `json:"tmuxinator_detected"`
	TmuxResurrect      bool   `json:"tmux_resurrect"`
}

type sessionStructInfo struct {
	WindowIndex     int `json:"window_index"`
	ControlPane     int `json:"control_pane"`
	AgentPaneStart  int `json:"agent_pane_start"`
	AgentPaneEnd    int `json:"agent_pane_end"`
	TotalAgentPanes int `json:"total_agent_panes"`
}

type shellEnvInfo struct {
	Type               string `json:"type"`
	TmuxPluginDetected bool   `json:"tmux_plugin_detected"`
	OhMyZshDetected    bool   `json:"oh_my_zsh_detected"`
	ConfigPath         string `json:"config_path,omitempty"`
}

type timingInfo struct {
	CtrlCGapMs          int `json:"ctrl_c_gap_ms"`
	PostExitWaitMs      int `json:"post_exit_wait_ms"`
	CCInitWaitMs        int `json:"cc_init_wait_ms"`
	PromptSubmitDelayMs int `json:"prompt_submit_delay_ms"`
}

type targetingInfo struct {
	PaneFormat         string `json:"pane_format"`
	ExampleAgentPane   string `json:"example_agent_pane"`
	ExampleControlPane string `json:"example_control_pane"`
}

func runRobotEnv(t *testing.T, dir, session string) robotEnvResponse {
	t.Helper()
	out := runCmd(t, dir, "ntm", "--robot-env="+session)
	jsonData := extractJSON(out)
	var resp robotEnvResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		t.Fatalf("unmarshal robot-env: %v\nout=%s", err, string(out))
	}
	return resp
}

func runRobotEnvAllowFail(t *testing.T, dir, session string) (robotEnvResponse, error) {
	t.Helper()
	out, err := runCmdAllowFail(t, dir, "ntm", "--robot-env="+session)
	var resp robotEnvResponse
	json.Unmarshal(extractJSON(out), &resp)
	return resp, err
}

// TestE2ERobotEnv_BasicInfoRetrieval tests basic environment info retrieval.
func TestE2ERobotEnv_BasicInfoRetrieval(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("global_env_info", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		// Verify success response
		if !resp.Success {
			t.Errorf("expected success=true, got false")
		}

		// Verify tmux info is populated
		if resp.Tmux.BinaryPath == "" {
			t.Errorf("expected tmux.binary_path to be set")
		}

		if resp.Tmux.Version == "" {
			t.Errorf("expected tmux.version to be set")
		}

		// Version should not contain "tmux " prefix
		if strings.HasPrefix(resp.Tmux.Version, "tmux ") {
			t.Errorf("version should not have 'tmux ' prefix, got: %s", resp.Tmux.Version)
		}

		// Recommended path should be set
		if resp.Tmux.RecommendedPath == "" {
			t.Errorf("expected tmux.recommended_path to be set")
		}

		// Session should be "global"
		if resp.Session != "global" {
			t.Errorf("expected session='global', got '%s'", resp.Session)
		}
	})

	t.Run("tmux_binary_path_valid", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		// Binary path should be a valid file
		if resp.Tmux.BinaryPath != "" {
			if _, err := os.Stat(resp.Tmux.BinaryPath); os.IsNotExist(err) {
				t.Errorf("tmux binary_path does not exist: %s", resp.Tmux.BinaryPath)
			}
		}
	})

	t.Run("output_format_json", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		if resp.OutputFormat != "json" {
			t.Errorf("expected output_format='json', got '%s'", resp.OutputFormat)
		}
	})
}

// TestE2ERobotEnv_ShellDetection tests shell environment detection.
func TestE2ERobotEnv_ShellDetection(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("shell_type_detected", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		if resp.Shell == nil {
			t.Fatalf("expected shell info to be set")
		}

		// Shell type should be one of the common shells
		validShells := map[string]bool{
			"bash": true,
			"zsh":  true,
			"fish": true,
			"sh":   true,
		}

		if !validShells[resp.Shell.Type] {
			t.Logf("shell type '%s' not in common shells list (may still be valid)", resp.Shell.Type)
		}

		// Verify shell type matches SHELL env var
		shellEnv := os.Getenv("SHELL")
		if shellEnv != "" {
			expectedType := filepath.Base(shellEnv)
			if resp.Shell.Type != expectedType {
				t.Errorf("shell.type='%s' doesn't match SHELL env '%s'", resp.Shell.Type, expectedType)
			}
		}
	})

	t.Run("config_path_set", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		if resp.Shell == nil {
			t.Fatalf("expected shell info to be set")
		}

		// Config path should be set for common shells
		if resp.Shell.ConfigPath == "" {
			t.Logf("config_path not set (may be expected for some shells)")
		} else {
			// Should be an absolute path
			if !filepath.IsAbs(resp.Shell.ConfigPath) {
				t.Errorf("config_path should be absolute, got: %s", resp.Shell.ConfigPath)
			}
		}
	})
}

// TestE2ERobotEnv_TimingValues tests timing constants.
func TestE2ERobotEnv_TimingValues(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("timing_values_present", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		if resp.Timing == nil {
			t.Fatalf("expected timing info to be set")
		}

		// Verify all timing values are set to reasonable defaults
		if resp.Timing.CtrlCGapMs <= 0 {
			t.Errorf("expected ctrl_c_gap_ms > 0, got %d", resp.Timing.CtrlCGapMs)
		}

		if resp.Timing.PostExitWaitMs <= 0 {
			t.Errorf("expected post_exit_wait_ms > 0, got %d", resp.Timing.PostExitWaitMs)
		}

		if resp.Timing.CCInitWaitMs <= 0 {
			t.Errorf("expected cc_init_wait_ms > 0, got %d", resp.Timing.CCInitWaitMs)
		}

		if resp.Timing.PromptSubmitDelayMs <= 0 {
			t.Errorf("expected prompt_submit_delay_ms > 0, got %d", resp.Timing.PromptSubmitDelayMs)
		}
	})

	t.Run("timing_values_reasonable", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		if resp.Timing == nil {
			t.Fatalf("expected timing info to be set")
		}

		// Verify timing values are in reasonable ranges
		// ctrl_c_gap should be between 50ms and 500ms
		if resp.Timing.CtrlCGapMs < 50 || resp.Timing.CtrlCGapMs > 500 {
			t.Errorf("ctrl_c_gap_ms=%d seems unreasonable (expected 50-500)", resp.Timing.CtrlCGapMs)
		}

		// post_exit_wait should be between 1000ms and 10000ms
		if resp.Timing.PostExitWaitMs < 1000 || resp.Timing.PostExitWaitMs > 10000 {
			t.Errorf("post_exit_wait_ms=%d seems unreasonable (expected 1000-10000)", resp.Timing.PostExitWaitMs)
		}

		// cc_init_wait should be between 3000ms and 15000ms
		if resp.Timing.CCInitWaitMs < 3000 || resp.Timing.CCInitWaitMs > 15000 {
			t.Errorf("cc_init_wait_ms=%d seems unreasonable (expected 3000-15000)", resp.Timing.CCInitWaitMs)
		}

		// prompt_submit_delay should be between 500ms and 3000ms
		if resp.Timing.PromptSubmitDelayMs < 500 || resp.Timing.PromptSubmitDelayMs > 3000 {
			t.Errorf("prompt_submit_delay_ms=%d seems unreasonable (expected 500-3000)", resp.Timing.PromptSubmitDelayMs)
		}
	})
}

// TestE2ERobotEnv_SessionNotFound tests error handling for non-existent sessions.
func TestE2ERobotEnv_SessionNotFound(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("nonexistent_session_error", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp, _ := runRobotEnvAllowFail(t, tmpDir, "nonexistent_session_12345")

		// Should still return response with success=false
		if resp.Success {
			t.Errorf("expected success=false for nonexistent session")
		}

		if resp.Error == "" {
			t.Errorf("expected error to be set for nonexistent session")
		}
		if resp.ErrorCode == "" {
			t.Errorf("expected error_code to be set for nonexistent session")
		}
	})
}

// TestE2ERobotEnv_TmuxPluginDetection tests tmux plugin detection.
func TestE2ERobotEnv_TmuxPluginDetection(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("plugin_detection_runs", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		// These fields should exist (values depend on environment)
		// We just verify the struct was populated correctly
		t.Logf("tmux plugin detection: alias=%v, omz=%v, tmuxinator=%v, resurrect=%v",
			resp.Tmux.ShellAliasDetected,
			resp.Tmux.OhMyZshTmuxPlugin,
			resp.Tmux.TmuxinatorDetected,
			resp.Tmux.TmuxResurrect)

		if resp.Shell != nil {
			t.Logf("shell plugin detection: omz=%v, tmux_plugin=%v",
				resp.Shell.OhMyZshDetected,
				resp.Shell.TmuxPluginDetected)
		}
	})

	t.Run("warning_on_alias", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		// If alias detected, warning should be set
		if resp.Tmux.ShellAliasDetected {
			if resp.Tmux.Warning == "" {
				t.Errorf("expected warning when shell alias detected")
			}
		}
	})
}

// TestE2ERobotEnv_JSONStructure tests JSON output structure completeness.
func TestE2ERobotEnv_JSONStructure(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("required_fields_present", func(t *testing.T) {
		tmpDir := t.TempDir()

		out := runCmd(t, tmpDir, "ntm", "--robot-env=global")
		jsonData := extractJSON(out)

		// Parse as raw JSON to check field presence
		var raw map[string]interface{}
		if err := json.Unmarshal(jsonData, &raw); err != nil {
			t.Fatalf("unmarshal raw json: %v", err)
		}

		// Check required top-level fields
		requiredFields := []string{"success", "timestamp", "version", "output_format", "session", "tmux"}
		for _, field := range requiredFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("missing required field: %s", field)
			}
		}

		// Check tmux subfields
		if tmux, ok := raw["tmux"].(map[string]interface{}); ok {
			tmuxFields := []string{"binary_path", "version", "recommended_path"}
			for _, field := range tmuxFields {
				if _, ok := tmux[field]; !ok {
					t.Errorf("missing tmux.%s field", field)
				}
			}
		} else {
			t.Errorf("tmux field is not an object")
		}
	})

	t.Run("timestamp_format", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

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

// TestE2ERobotEnv_VersionConsistency tests version field consistency.
func TestE2ERobotEnv_VersionConsistency(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	t.Run("version_field_set", func(t *testing.T) {
		tmpDir := t.TempDir()

		resp := runRobotEnv(t, tmpDir, "global")

		// Version should be set (robot protocol version)
		if resp.Version == "" {
			t.Errorf("expected version field to be set")
		}

		// Version should be semver-like
		if !strings.Contains(resp.Version, ".") {
			t.Errorf("version doesn't look like semver: %s", resp.Version)
		}
	})
}

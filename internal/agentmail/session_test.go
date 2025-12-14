package agentmail

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeSessionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myproject", "myproject"},
		{"my-project", "my_project"},
		{"my_project", "my_project"},
		{"MyProject", "myproject"},
		{"my.project.name", "my_project_name"},
		{"project@123", "project_123"},
		{"---project---", "project"},
		{"Project With Spaces", "project_with_spaces"},
		{"...", "hex_2e2e2e"}, // "..." -> "" -> hex
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeSessionName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeSessionName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSessionAgentPath(t *testing.T) {
	path := sessionAgentPath("myproject", "/abs/path/to/project")
	if !filepath.IsAbs(path) {
		t.Errorf("sessionAgentPath should return absolute path, got %q", path)
	}
	if !contains(path, "myproject") {
		t.Errorf("sessionAgentPath should contain session name, got %q", path)
	}
	if !contains(path, "agent.json") {
		t.Errorf("sessionAgentPath should end with agent.json, got %q", path)
	}
}

func TestLoadSaveSessionAgent(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "agentmail-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override the config dir for testing
	// On Linux, os.UserConfigDir() uses XDG_CONFIG_HOME, not HOME
	// On macOS, os.UserConfigDir() uses HOME/Library/Application Support
	// t.Setenv handles cleanup automatically
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sessionName := "test-session"

	// Initially no agent should be loaded
	info, err := LoadSessionAgent(sessionName, "/path/to/project")
	if err != nil {
		t.Fatalf("LoadSessionAgent failed: %v", err)
	}
	if info != nil {
		t.Error("Expected nil info for non-existent session")
	}

	// Save agent info
	saveInfo := &SessionAgentInfo{
		AgentName:  "ntm_test_session",
		ProjectKey: "/path/to/project",
	}
	if err := SaveSessionAgent(sessionName, saveInfo.ProjectKey, saveInfo); err != nil {
		t.Fatalf("SaveSessionAgent failed: %v", err)
	}

	// Load it back
	loaded, err := LoadSessionAgent(sessionName, saveInfo.ProjectKey)
	if err != nil {
		t.Fatalf("LoadSessionAgent failed after save: %v", err)
	}
	if loaded == nil {
		t.Fatal("Expected loaded info to be non-nil")
	}
	if loaded.AgentName != saveInfo.AgentName {
		t.Errorf("AgentName = %q, want %q", loaded.AgentName, saveInfo.AgentName)
	}
	if loaded.ProjectKey != saveInfo.ProjectKey {
		t.Errorf("ProjectKey = %q, want %q", loaded.ProjectKey, saveInfo.ProjectKey)
	}

	// Delete agent info
	if err := DeleteSessionAgent(sessionName, saveInfo.ProjectKey); err != nil {
		t.Fatalf("DeleteSessionAgent failed: %v", err)
	}

	// Verify it's gone
	info, err = LoadSessionAgent(sessionName, saveInfo.ProjectKey)
	if err != nil {
		t.Fatalf("LoadSessionAgent failed after delete: %v", err)
	}
	if info != nil {
		t.Error("Expected nil info after delete")
	}
}

func TestIsNameTakenError(t *testing.T) {
	tests := []struct {
		errStr   string
		expected bool
	}{
		{"name already in use", true},
		{"agent name taken", true},
		{"agent already registered", true},
		{"some other error", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.errStr, func(t *testing.T) {
			var err error
			if tt.errStr != "" {
				err = NewAPIError("test", 0, &testError{msg: tt.errStr})
			}
			result := IsNameTakenError(err)
			if result != tt.expected {
				t.Errorf("IsNameTakenError(%q) = %v, want %v", tt.errStr, result, tt.expected)
			}
		})
	}
}

// Helper functions for tests

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

package status

import (
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("NewDetector returned nil")
	}

	config := d.Config()
	if config.ActivityThreshold != 5 {
		t.Errorf("Expected ActivityThreshold 5, got %d", config.ActivityThreshold)
	}
	if config.OutputPreviewLength != 200 {
		t.Errorf("Expected OutputPreviewLength 200, got %d", config.OutputPreviewLength)
	}
	if config.ScanLines != 50 {
		t.Errorf("Expected ScanLines 50, got %d", config.ScanLines)
	}
}

func TestNewDetectorWithConfig(t *testing.T) {
	config := DetectorConfig{
		ActivityThreshold:   10,
		OutputPreviewLength: 100,
		ScanLines:           25,
	}
	d := NewDetectorWithConfig(config)

	got := d.Config()
	if got.ActivityThreshold != 10 {
		t.Errorf("Expected ActivityThreshold 10, got %d", got.ActivityThreshold)
	}
	if got.OutputPreviewLength != 100 {
		t.Errorf("Expected OutputPreviewLength 100, got %d", got.OutputPreviewLength)
	}
	if got.ScanLines != 25 {
		t.Errorf("Expected ScanLines 25, got %d", got.ScanLines)
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncate from start",
			input:    "hello world",
			maxLen:   5,
			expected: "world",
		},
		{
			name:     "with whitespace",
			input:    "  hello world  ",
			maxLen:   100,
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOutput(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateOutput(%q, %d) = %q, want %q",
					tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestGetStateSummary(t *testing.T) {
	statuses := []AgentStatus{
		{State: StateIdle},
		{State: StateIdle},
		{State: StateWorking},
		{State: StateError},
		{State: StateUnknown},
	}

	summary := GetStateSummary(statuses)

	if summary[StateIdle] != 2 {
		t.Errorf("Expected 2 idle, got %d", summary[StateIdle])
	}
	if summary[StateWorking] != 1 {
		t.Errorf("Expected 1 working, got %d", summary[StateWorking])
	}
	if summary[StateError] != 1 {
		t.Errorf("Expected 1 error, got %d", summary[StateError])
	}
	if summary[StateUnknown] != 1 {
		t.Errorf("Expected 1 unknown, got %d", summary[StateUnknown])
	}
}

func TestFilterByState(t *testing.T) {
	statuses := []AgentStatus{
		{PaneID: "%0", State: StateIdle},
		{PaneID: "%1", State: StateWorking},
		{PaneID: "%2", State: StateIdle},
		{PaneID: "%3", State: StateError},
	}

	idle := FilterByState(statuses, StateIdle)
	if len(idle) != 2 {
		t.Errorf("Expected 2 idle statuses, got %d", len(idle))
	}

	working := FilterByState(statuses, StateWorking)
	if len(working) != 1 {
		t.Errorf("Expected 1 working status, got %d", len(working))
	}

	error := FilterByState(statuses, StateError)
	if len(error) != 1 {
		t.Errorf("Expected 1 error status, got %d", len(error))
	}

	unknown := FilterByState(statuses, StateUnknown)
	if len(unknown) != 0 {
		t.Errorf("Expected 0 unknown statuses, got %d", len(unknown))
	}
}

func TestFilterByAgentType(t *testing.T) {
	statuses := []AgentStatus{
		{PaneID: "%0", AgentType: "cc"},
		{PaneID: "%1", AgentType: "cod"},
		{PaneID: "%2", AgentType: "cc"},
		{PaneID: "%3", AgentType: "user"},
	}

	claude := FilterByAgentType(statuses, "cc")
	if len(claude) != 2 {
		t.Errorf("Expected 2 claude agents, got %d", len(claude))
	}

	codex := FilterByAgentType(statuses, "cod")
	if len(codex) != 1 {
		t.Errorf("Expected 1 codex agent, got %d", len(codex))
	}

	gemini := FilterByAgentType(statuses, "gmi")
	if len(gemini) != 0 {
		t.Errorf("Expected 0 gemini agents, got %d", len(gemini))
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name     string
		statuses []AgentStatus
		expected bool
	}{
		{
			name: "no errors",
			statuses: []AgentStatus{
				{State: StateIdle},
				{State: StateWorking},
			},
			expected: false,
		},
		{
			name: "has error",
			statuses: []AgentStatus{
				{State: StateIdle},
				{State: StateError},
			},
			expected: true,
		},
		{
			name:     "empty list",
			statuses: []AgentStatus{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasErrors(tt.statuses)
			if result != tt.expected {
				t.Errorf("HasErrors = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAllHealthy(t *testing.T) {
	tests := []struct {
		name     string
		statuses []AgentStatus
		expected bool
	}{
		{
			name: "all healthy",
			statuses: []AgentStatus{
				{State: StateIdle},
				{State: StateWorking},
			},
			expected: true,
		},
		{
			name: "has error",
			statuses: []AgentStatus{
				{State: StateIdle},
				{State: StateError},
			},
			expected: false,
		},
		{
			name: "has unknown",
			statuses: []AgentStatus{
				{State: StateIdle},
				{State: StateUnknown},
			},
			expected: false,
		},
		{
			name:     "empty list",
			statuses: []AgentStatus{},
			expected: false, // Empty list is not "all healthy"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllHealthy(tt.statuses)
			if result != tt.expected {
				t.Errorf("AllHealthy = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAgentStatusIsHealthy(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected bool
	}{
		{StateIdle, true},
		{StateWorking, true},
		{StateError, false},
		{StateUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			status := AgentStatus{State: tt.state}
			if status.IsHealthy() != tt.expected {
				t.Errorf("IsHealthy() for %s = %v, want %v",
					tt.state, status.IsHealthy(), tt.expected)
			}
		})
	}
}

func TestAgentStatusIdleDuration(t *testing.T) {
	// Set LastActive to 5 minutes ago
	status := AgentStatus{
		LastActive: time.Now().Add(-5 * time.Minute),
	}

	duration := status.IdleDuration()

	// Should be approximately 5 minutes
	if duration < 4*time.Minute || duration > 6*time.Minute {
		t.Errorf("IdleDuration = %v, expected around 5 minutes", duration)
	}
}

func TestAgentStateIcon(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected string
	}{
		{StateIdle, "\u26aa"},      // white circle
		{StateWorking, "\U0001f7e2"}, // green circle
		{StateError, "\U0001f534"},   // red circle
		{StateUnknown, "\u26ab"},   // black circle
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if tt.state.Icon() != tt.expected {
				t.Errorf("Icon() for %s = %q, want %q",
					tt.state, tt.state.Icon(), tt.expected)
			}
		})
	}
}

func TestErrorTypeMessage(t *testing.T) {
	tests := []struct {
		errType  ErrorType
		expected string
	}{
		{ErrorRateLimit, "Rate limited - too many requests"},
		{ErrorCrash, "Agent crashed"},
		{ErrorAuth, "Authentication error"},
		{ErrorConnection, "Connection error"},
		{ErrorGeneric, "Error detected"},
		{ErrorNone, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.errType), func(t *testing.T) {
			if tt.errType.Message() != tt.expected {
				t.Errorf("Message() for %s = %q, want %q",
					tt.errType, tt.errType.Message(), tt.expected)
			}
		})
	}
}

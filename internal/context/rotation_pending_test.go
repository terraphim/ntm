package context

import (
	"testing"
	"time"
)

func TestRotator_GetPendingRotations_Empty(t *testing.T) {
	t.Parallel()
	r := NewRotator(RotatorConfig{})

	got := r.GetPendingRotations()
	if len(got) != 0 {
		t.Errorf("GetPendingRotations() on empty rotator returned %d items; want 0", len(got))
	}
}

func TestRotator_GetPendingRotations_Multiple(t *testing.T) {
	t.Parallel()
	r := NewRotator(RotatorConfig{})

	r.pending["agent-1"] = &PendingRotation{
		AgentID:     "agent-1",
		SessionName: "session-a",
		PaneID:      "pane-1",
		CreatedAt:   time.Now(),
	}
	r.pending["agent-2"] = &PendingRotation{
		AgentID:     "agent-2",
		SessionName: "session-b",
		PaneID:      "pane-2",
		CreatedAt:   time.Now(),
	}

	got := r.GetPendingRotations()
	if len(got) != 2 {
		t.Fatalf("GetPendingRotations() returned %d items; want 2", len(got))
	}

	ids := map[string]bool{}
	for _, p := range got {
		ids[p.AgentID] = true
	}
	if !ids["agent-1"] || !ids["agent-2"] {
		t.Errorf("GetPendingRotations() missing expected agent IDs; got %v", ids)
	}
}

func TestRotator_GetPendingRotation_Exists(t *testing.T) {
	t.Parallel()
	r := NewRotator(RotatorConfig{})

	expected := &PendingRotation{
		AgentID:     "agent-1",
		SessionName: "test-session",
		PaneID:      "pane-5",
	}
	r.pending["agent-1"] = expected

	got := r.GetPendingRotation("agent-1")
	if got != expected {
		t.Errorf("GetPendingRotation(%q) returned different pointer", "agent-1")
	}
	if got.SessionName != "test-session" {
		t.Errorf("GetPendingRotation(%q).SessionName = %q; want %q", "agent-1", got.SessionName, "test-session")
	}
}

func TestRotator_GetPendingRotation_Missing(t *testing.T) {
	t.Parallel()
	r := NewRotator(RotatorConfig{})

	got := r.GetPendingRotation("nonexistent")
	if got != nil {
		t.Errorf("GetPendingRotation(%q) = %v; want nil", "nonexistent", got)
	}
}

func TestRotator_HasPendingRotation(t *testing.T) {
	t.Parallel()
	r := NewRotator(RotatorConfig{})

	r.pending["agent-1"] = &PendingRotation{AgentID: "agent-1"}

	tests := []struct {
		agentID string
		want    bool
	}{
		{"agent-1", true},
		{"agent-2", false},
		{"", false},
	}

	for _, tc := range tests {
		got := r.HasPendingRotation(tc.agentID)
		if got != tc.want {
			t.Errorf("HasPendingRotation(%q) = %v; want %v", tc.agentID, got, tc.want)
		}
	}
}

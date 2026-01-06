package coordinator

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/bv"
	"github.com/Dicklesworthstone/ntm/internal/robot"
)

func TestWorkAssignmentStruct(t *testing.T) {
	now := time.Now()
	wa := WorkAssignment{
		BeadID:         "ntm-1234",
		BeadTitle:      "Implement feature X",
		AgentPaneID:    "%0",
		AgentMailName:  "BlueFox",
		AgentType:      "cc",
		AssignedAt:     now,
		Priority:       1,
		Score:          0.85,
		FilesToReserve: []string{"internal/feature/*.go"},
	}

	if wa.BeadID != "ntm-1234" {
		t.Errorf("expected BeadID 'ntm-1234', got %q", wa.BeadID)
	}
	if wa.Score != 0.85 {
		t.Errorf("expected Score 0.85, got %f", wa.Score)
	}
	if len(wa.FilesToReserve) != 1 {
		t.Errorf("expected 1 file to reserve, got %d", len(wa.FilesToReserve))
	}
}

func TestAssignmentResultStruct(t *testing.T) {
	ar := AssignmentResult{
		Success:      true,
		MessageSent:  true,
		Reservations: []string{"internal/*.go"},
	}

	if !ar.Success {
		t.Error("expected Success to be true")
	}
	if !ar.MessageSent {
		t.Error("expected MessageSent to be true")
	}
	if ar.Error != "" {
		t.Error("expected empty error on success")
	}
}

func TestRemoveRecommendation(t *testing.T) {
	recs := []bv.TriageRecommendation{
		{ID: "ntm-001", Title: "First"},
		{ID: "ntm-002", Title: "Second"},
		{ID: "ntm-003", Title: "Third"},
	}

	result := removeRecommendation(recs, "ntm-002")

	if len(result) != 2 {
		t.Errorf("expected 2 recommendations after removal, got %d", len(result))
	}
	for _, r := range result {
		if r.ID == "ntm-002" {
			t.Error("expected ntm-002 to be removed")
		}
	}

	// Test removing non-existent ID
	result2 := removeRecommendation(recs, "ntm-999")
	if len(result2) != 3 {
		t.Errorf("expected 3 recommendations when removing non-existent, got %d", len(result2))
	}
}

func TestFindBestMatch(t *testing.T) {
	c := New("test-session", "/tmp/test", nil, "TestAgent")

	agent := &AgentState{
		PaneID:        "%0",
		AgentType:     "cc",
		AgentMailName: "BlueFox",
		Status:        robot.StateWaiting,
		Healthy:       true,
	}

	recs := []bv.TriageRecommendation{
		{ID: "ntm-001", Title: "Blocked Task", Status: "blocked", Score: 0.9},
		{ID: "ntm-002", Title: "Ready Task", Status: "open", Priority: 1, Score: 0.8},
		{ID: "ntm-003", Title: "Another Ready", Status: "open", Priority: 2, Score: 0.7},
	}

	assignment, rec := c.findBestMatch(agent, recs)

	if assignment == nil {
		t.Fatal("expected assignment, got nil")
	}
	if rec == nil {
		t.Fatal("expected recommendation, got nil")
	}
	if assignment.BeadID != "ntm-002" {
		t.Errorf("expected BeadID 'ntm-002' (first non-blocked), got %q", assignment.BeadID)
	}
	if assignment.AgentMailName != "BlueFox" {
		t.Errorf("expected AgentMailName 'BlueFox', got %q", assignment.AgentMailName)
	}
}

func TestFindBestMatchAllBlocked(t *testing.T) {
	c := New("test-session", "/tmp/test", nil, "TestAgent")

	agent := &AgentState{
		PaneID:    "%0",
		AgentType: "cc",
	}

	recs := []bv.TriageRecommendation{
		{ID: "ntm-001", Title: "Blocked 1", Status: "blocked"},
		{ID: "ntm-002", Title: "Blocked 2", Status: "blocked"},
	}

	assignment, rec := c.findBestMatch(agent, recs)

	if assignment != nil {
		t.Error("expected nil assignment when all are blocked")
	}
	if rec != nil {
		t.Error("expected nil recommendation when all are blocked")
	}
}

func TestFindBestMatchEmpty(t *testing.T) {
	c := New("test-session", "/tmp/test", nil, "TestAgent")

	agent := &AgentState{
		PaneID:    "%0",
		AgentType: "cc",
	}

	assignment, rec := c.findBestMatch(agent, nil)

	if assignment != nil || rec != nil {
		t.Error("expected nil for empty recommendations")
	}

	assignment, rec = c.findBestMatch(agent, []bv.TriageRecommendation{})

	if assignment != nil || rec != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestFormatAssignmentMessage(t *testing.T) {
	c := New("test-session", "/tmp/test", nil, "TestAgent")

	assignment := &WorkAssignment{
		BeadID:    "ntm-1234",
		BeadTitle: "Implement feature X",
		Priority:  1,
		Score:     0.85,
	}

	rec := &bv.TriageRecommendation{
		ID:         "ntm-1234",
		Title:      "Implement feature X",
		Reasons:    []string{"High impact", "Unblocks others"},
		UnblocksIDs: []string{"ntm-2000", "ntm-2001"},
	}

	body := c.formatAssignmentMessage(assignment, rec)

	if body == "" {
		t.Error("expected non-empty message body")
	}
	if !containsString(body, "# Work Assignment") {
		t.Error("expected markdown header in message")
	}
	if !containsString(body, "ntm-1234") {
		t.Error("expected bead ID in message")
	}
	if !containsString(body, "High impact") {
		t.Error("expected reasons in message")
	}
	if !containsString(body, "bd show") {
		t.Error("expected bd show instruction in message")
	}
}

// Helper function for string contains
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

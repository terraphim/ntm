// Package cost provides API cost tracking for AI agent sessions.
package cost

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"test", 1},
		{"hello world", 3},           // 11 chars -> 3 tokens
		{"This is a longer text", 6}, // 21 chars -> 6 tokens
		{"A very long sentence that should result in many tokens being counted", 17}, // 68 chars -> 17 tokens
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.expected)
			}
		})
	}
}

func TestGetModelPricing(t *testing.T) {
	tests := []struct {
		model       string
		wantInput   float64
		wantOutput  float64
	}{
		{"claude-opus", 0.015, 0.075},
		{"claude-sonnet", 0.003, 0.015},
		{"claude-haiku", 0.00025, 0.00125},
		{"gpt-4o", 0.005, 0.015},
		{"gpt-4o-mini", 0.00015, 0.0006},
		{"gemini-flash", 0.000075, 0.0003},
		{"unknown-model", 0.003, 0.015}, // Falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := GetModelPricing(tt.model)
			if pricing.InputPer1K != tt.wantInput {
				t.Errorf("GetModelPricing(%q).InputPer1K = %v, want %v", tt.model, pricing.InputPer1K, tt.wantInput)
			}
			if pricing.OutputPer1K != tt.wantOutput {
				t.Errorf("GetModelPricing(%q).OutputPer1K = %v, want %v", tt.model, pricing.OutputPer1K, tt.wantOutput)
			}
		})
	}
}

func TestAgentCost_Cost(t *testing.T) {
	tests := []struct {
		name         string
		inputTokens  int
		outputTokens int
		model        string
		wantMin      float64
		wantMax      float64
	}{
		{
			name:         "claude-opus 1k input 1k output",
			inputTokens:  1000,
			outputTokens: 1000,
			model:        "claude-opus",
			wantMin:      0.089, // 0.015 + 0.075 = 0.09
			wantMax:      0.091,
		},
		{
			name:         "claude-haiku cheap",
			inputTokens:  10000,
			outputTokens: 5000,
			model:        "claude-haiku",
			wantMin:      0.008, // 10*0.00025 + 5*0.00125 = 0.0025 + 0.00625 = 0.00875
			wantMax:      0.009,
		},
		{
			name:         "zero tokens",
			inputTokens:  0,
			outputTokens: 0,
			model:        "claude-opus",
			wantMin:      0,
			wantMax:      0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AgentCost{
				InputTokens:  tt.inputTokens,
				OutputTokens: tt.outputTokens,
				Model:        tt.model,
			}
			got := a.Cost()
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("AgentCost.Cost() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSessionCost_TotalCost(t *testing.T) {
	s := &SessionCost{
		Agents: map[string]*AgentCost{
			"agent1": {InputTokens: 1000, OutputTokens: 1000, Model: "claude-opus"},
			"agent2": {InputTokens: 1000, OutputTokens: 1000, Model: "claude-haiku"},
		},
	}

	total := s.TotalCost()
	// claude-opus: 0.015 + 0.075 = 0.09
	// claude-haiku: 0.00025 + 0.00125 = 0.00175
	// total: ~0.09175
	if total < 0.09 || total > 0.095 {
		t.Errorf("SessionCost.TotalCost() = %v, want ~0.09175", total)
	}
}

func TestSessionCost_TotalTokens(t *testing.T) {
	s := &SessionCost{
		Agents: map[string]*AgentCost{
			"agent1": {InputTokens: 1000, OutputTokens: 500},
			"agent2": {InputTokens: 2000, OutputTokens: 1500},
		},
	}

	input, output := s.TotalTokens()
	if input != 3000 {
		t.Errorf("TotalTokens() input = %d, want 3000", input)
	}
	if output != 2000 {
		t.Errorf("TotalTokens() output = %d, want 2000", output)
	}
}

func TestNewCostTracker(t *testing.T) {
	tracker := NewCostTracker("/tmp/test")
	if tracker == nil {
		t.Fatal("NewCostTracker returned nil")
	}
	if tracker.dataDir != "/tmp/test" {
		t.Errorf("dataDir = %q, want %q", tracker.dataDir, "/tmp/test")
	}
	if tracker.sessions == nil {
		t.Error("sessions map should not be nil")
	}
}

func TestCostTracker_RecordPrompt(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordPrompt("session1", "pane1", "claude-opus", "Hello world")

	s := tracker.GetSession("session1")
	if s == nil {
		t.Fatal("session not created")
	}
	agent, ok := s.Agents["pane1"]
	if !ok {
		t.Fatal("agent not created")
	}
	if agent.InputTokens == 0 {
		t.Error("InputTokens should be > 0")
	}
	if agent.Model != "claude-opus" {
		t.Errorf("Model = %q, want %q", agent.Model, "claude-opus")
	}
}

func TestCostTracker_RecordResponse(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordResponse("session1", "pane1", "claude-sonnet", "This is a response")

	s := tracker.GetSession("session1")
	if s == nil {
		t.Fatal("session not created")
	}
	agent := s.Agents["pane1"]
	if agent == nil {
		t.Fatal("agent not created")
	}
	if agent.OutputTokens == 0 {
		t.Error("OutputTokens should be > 0")
	}
}

func TestCostTracker_RecordTokens(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordTokens("session1", "pane1", "gpt-4o", 500, 200)

	s := tracker.GetSession("session1")
	if s == nil {
		t.Fatal("session not created")
	}
	agent := s.Agents["pane1"]
	if agent.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500", agent.InputTokens)
	}
	if agent.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", agent.OutputTokens)
	}
}

func TestCostTracker_GetSessionCost(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordTokens("session1", "pane1", "claude-opus", 1000, 1000)

	cost := tracker.GetSessionCost("session1")
	// 0.015 + 0.075 = 0.09
	if cost < 0.089 || cost > 0.091 {
		t.Errorf("GetSessionCost() = %v, want ~0.09", cost)
	}

	// Non-existent session
	cost = tracker.GetSessionCost("nonexistent")
	if cost != 0 {
		t.Errorf("GetSessionCost(nonexistent) = %v, want 0", cost)
	}
}

func TestCostTracker_GetAllSessions(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordPrompt("session1", "pane1", "claude-opus", "test")
	tracker.RecordPrompt("session2", "pane1", "claude-opus", "test")

	sessions := tracker.GetAllSessions()
	if len(sessions) != 2 {
		t.Errorf("GetAllSessions() returned %d sessions, want 2", len(sessions))
	}
}

func TestCostTracker_GetTotalCost(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordTokens("session1", "pane1", "claude-opus", 1000, 1000)
	tracker.RecordTokens("session2", "pane1", "claude-opus", 1000, 1000)

	total := tracker.GetTotalCost()
	// 2 sessions * 0.09 = 0.18
	if total < 0.17 || total > 0.19 {
		t.Errorf("GetTotalCost() = %v, want ~0.18", total)
	}
}

func TestCostTracker_ClearSession(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordPrompt("session1", "pane1", "claude-opus", "test")
	tracker.ClearSession("session1")

	s := tracker.GetSession("session1")
	if s != nil {
		t.Error("session should be nil after ClearSession")
	}
}

func TestCostTracker_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create tracker and record data
	tracker1 := NewCostTracker(tmpDir)
	tracker1.RecordTokens("session1", "pane1", "claude-opus", 1000, 500)
	tracker1.RecordTokens("session1", "pane2", "claude-sonnet", 2000, 1000)

	// Save
	if err := tracker1.SaveToDir(tmpDir); err != nil {
		t.Fatalf("SaveToDir failed: %v", err)
	}

	// Verify file exists
	costPath := filepath.Join(tmpDir, ".ntm", "costs.json")
	if _, err := os.Stat(costPath); os.IsNotExist(err) {
		t.Fatal("costs.json not created")
	}

	// Create new tracker and load
	tracker2 := NewCostTracker(tmpDir)
	if err := tracker2.LoadFromDir(tmpDir); err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Verify data was loaded correctly
	s := tracker2.GetSession("session1")
	if s == nil {
		t.Fatal("session not loaded")
	}
	if len(s.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(s.Agents))
	}

	agent1 := s.Agents["pane1"]
	if agent1.InputTokens != 1000 || agent1.OutputTokens != 500 {
		t.Errorf("pane1 tokens not loaded correctly: input=%d output=%d", agent1.InputTokens, agent1.OutputTokens)
	}
}

func TestCostTracker_LoadFromDir_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	tracker := NewCostTracker(tmpDir)

	// Should not error when file doesn't exist
	if err := tracker.LoadFromDir(tmpDir); err != nil {
		t.Errorf("LoadFromDir should not error for missing file: %v", err)
	}
}

func TestCostTracker_Concurrent(t *testing.T) {
	tracker := NewCostTracker("")
	var wg sync.WaitGroup

	// Simulate concurrent access from multiple agents
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pane := "pane" + string(rune('A'+id))
			for j := 0; j < 100; j++ {
				tracker.RecordPrompt("session1", pane, "claude-opus", "test prompt")
				tracker.RecordResponse("session1", pane, "claude-opus", "test response")
			}
		}(i)
	}

	wg.Wait()

	s := tracker.GetSession("session1")
	if s == nil {
		t.Fatal("session not created")
	}
	if len(s.Agents) != 10 {
		t.Errorf("expected 10 agents, got %d", len(s.Agents))
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		usd  float64
		want string
	}{
		{0.0001, "$0.0001"},
		{0.001, "$0.0010"},
		{0.01, "$0.010"},
		{0.1, "$0.100"},
		{1.0, "$1.00"},
		{10.5, "$10.50"},
		{100.99, "$100.99"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatCost(tt.usd)
			if got != tt.want {
				t.Errorf("FormatCost(%v) = %q, want %q", tt.usd, got, tt.want)
			}
		})
	}
}

func TestAgentCost_LastUpdated(t *testing.T) {
	tracker := NewCostTracker("")
	before := time.Now()
	tracker.RecordPrompt("session1", "pane1", "claude-opus", "test")
	after := time.Now()

	s := tracker.GetSession("session1")
	agent := s.Agents["pane1"]

	if agent.LastUpdated.Before(before) || agent.LastUpdated.After(after) {
		t.Errorf("LastUpdated = %v, want between %v and %v", agent.LastUpdated, before, after)
	}
}

func TestCostTracker_ModelUpdate(t *testing.T) {
	tracker := NewCostTracker("")

	// First record without model
	tracker.RecordPrompt("session1", "pane1", "", "test")

	s := tracker.GetSession("session1")
	if s.Agents["pane1"].Model != "" {
		t.Error("Model should be empty initially")
	}

	// Second record with model - should update
	tracker.RecordPrompt("session1", "pane1", "claude-opus", "test")

	s = tracker.GetSession("session1")
	if s.Agents["pane1"].Model != "claude-opus" {
		t.Errorf("Model should be updated to claude-opus, got %q", s.Agents["pane1"].Model)
	}
}

func TestCostTracker_GetSession_ReturnsCopy(t *testing.T) {
	tracker := NewCostTracker("")
	tracker.RecordTokens("session1", "pane1", "claude-opus", 1000, 500)

	// Get a copy
	s1 := tracker.GetSession("session1")

	// Modify the copy
	s1.Agents["pane1"].InputTokens = 9999

	// Get another copy - should not reflect the modification
	s2 := tracker.GetSession("session1")
	if s2.Agents["pane1"].InputTokens != 1000 {
		t.Error("GetSession should return a copy, not the original")
	}
}

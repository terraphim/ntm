package robot

import (
	"strings"
	"testing"
	"time"
)

func TestNewVelocityTracker(t *testing.T) {
	tracker := NewVelocityTracker("test-pane")

	if tracker.PaneID != "test-pane" {
		t.Errorf("expected PaneID 'test-pane', got %q", tracker.PaneID)
	}
	if tracker.MaxSamples != DefaultMaxSamples {
		t.Errorf("expected MaxSamples %d, got %d", DefaultMaxSamples, tracker.MaxSamples)
	}
	if len(tracker.Samples) != 0 {
		t.Errorf("expected empty samples, got %d", len(tracker.Samples))
	}
}

func TestNewVelocityTrackerWithSize(t *testing.T) {
	// Test with valid size
	tracker := NewVelocityTrackerWithSize("test-pane", 5)
	if tracker.MaxSamples != 5 {
		t.Errorf("expected MaxSamples 5, got %d", tracker.MaxSamples)
	}

	// Test with zero size (should default)
	tracker = NewVelocityTrackerWithSize("test-pane", 0)
	if tracker.MaxSamples != DefaultMaxSamples {
		t.Errorf("expected default MaxSamples %d, got %d", DefaultMaxSamples, tracker.MaxSamples)
	}

	// Test with negative size (should default)
	tracker = NewVelocityTrackerWithSize("test-pane", -5)
	if tracker.MaxSamples != DefaultMaxSamples {
		t.Errorf("expected default MaxSamples %d, got %d", DefaultMaxSamples, tracker.MaxSamples)
	}
}

func TestVelocityTracker_AddSample(t *testing.T) {
	tracker := NewVelocityTrackerWithSize("test", 3)

	// Add samples directly for testing
	samples := []VelocitySample{
		{Timestamp: time.Now(), CharsAdded: 10, Velocity: 5.0},
		{Timestamp: time.Now(), CharsAdded: 20, Velocity: 10.0},
		{Timestamp: time.Now(), CharsAdded: 30, Velocity: 15.0},
	}

	for _, s := range samples {
		tracker.mu.Lock()
		tracker.addSampleLocked(s)
		tracker.mu.Unlock()
	}

	if len(tracker.Samples) != 3 {
		t.Errorf("expected 3 samples, got %d", len(tracker.Samples))
	}

	// Add one more to test circular buffer behavior
	newSample := VelocitySample{Timestamp: time.Now(), CharsAdded: 40, Velocity: 20.0}
	tracker.mu.Lock()
	tracker.addSampleLocked(newSample)
	tracker.mu.Unlock()

	if len(tracker.Samples) != 3 {
		t.Errorf("expected 3 samples after overflow, got %d", len(tracker.Samples))
	}

	// First sample should now be the second original
	if tracker.Samples[0].Velocity != 10.0 {
		t.Errorf("expected first sample velocity 10.0, got %f", tracker.Samples[0].Velocity)
	}

	// Last sample should be the new one
	if tracker.Samples[2].Velocity != 20.0 {
		t.Errorf("expected last sample velocity 20.0, got %f", tracker.Samples[2].Velocity)
	}
}

func TestVelocityTracker_CurrentVelocity(t *testing.T) {
	tracker := NewVelocityTracker("test")

	// Empty tracker should return 0
	if v := tracker.CurrentVelocity(); v != 0 {
		t.Errorf("expected 0 velocity for empty tracker, got %f", v)
	}

	// Add samples
	tracker.mu.Lock()
	tracker.addSampleLocked(VelocitySample{Velocity: 5.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 10.0})
	tracker.mu.Unlock()

	// Should return last sample's velocity
	if v := tracker.CurrentVelocity(); v != 10.0 {
		t.Errorf("expected velocity 10.0, got %f", v)
	}
}

func TestVelocityTracker_AverageVelocity(t *testing.T) {
	tracker := NewVelocityTracker("test")

	// Empty tracker should return 0
	if v := tracker.AverageVelocity(); v != 0 {
		t.Errorf("expected 0 average for empty tracker, got %f", v)
	}

	// Add samples with known velocities
	tracker.mu.Lock()
	tracker.addSampleLocked(VelocitySample{Velocity: 5.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 10.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 15.0})
	tracker.mu.Unlock()

	// Average should be (5 + 10 + 15) / 3 = 10
	expected := 10.0
	if v := tracker.AverageVelocity(); v != expected {
		t.Errorf("expected average %f, got %f", expected, v)
	}
}

func TestVelocityTracker_RecentVelocity(t *testing.T) {
	tracker := NewVelocityTracker("test")

	// Empty tracker should return 0
	if v := tracker.RecentVelocity(2); v != 0 {
		t.Errorf("expected 0 for empty tracker, got %f", v)
	}

	// Add samples
	tracker.mu.Lock()
	tracker.addSampleLocked(VelocitySample{Velocity: 5.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 10.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 15.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 20.0})
	tracker.mu.Unlock()

	// Recent 2 should be (15 + 20) / 2 = 17.5
	if v := tracker.RecentVelocity(2); v != 17.5 {
		t.Errorf("expected 17.5, got %f", v)
	}

	// Recent 0 or negative should use all samples
	expected := (5.0 + 10.0 + 15.0 + 20.0) / 4.0
	if v := tracker.RecentVelocity(0); v != expected {
		t.Errorf("expected %f for n=0, got %f", expected, v)
	}

	// Recent larger than samples should use all
	if v := tracker.RecentVelocity(100); v != expected {
		t.Errorf("expected %f for n=100, got %f", expected, v)
	}
}

func TestVelocityTracker_SampleCount(t *testing.T) {
	tracker := NewVelocityTrackerWithSize("test", 5)

	if c := tracker.SampleCount(); c != 0 {
		t.Errorf("expected 0 samples, got %d", c)
	}

	tracker.mu.Lock()
	tracker.addSampleLocked(VelocitySample{Velocity: 1.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 2.0})
	tracker.mu.Unlock()

	if c := tracker.SampleCount(); c != 2 {
		t.Errorf("expected 2 samples, got %d", c)
	}
}

func TestVelocityTracker_GetSamples(t *testing.T) {
	tracker := NewVelocityTracker("test")

	tracker.mu.Lock()
	tracker.addSampleLocked(VelocitySample{Velocity: 5.0})
	tracker.addSampleLocked(VelocitySample{Velocity: 10.0})
	tracker.mu.Unlock()

	samples := tracker.GetSamples()

	// Should be a copy
	if len(samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(samples))
	}

	// Modifying copy shouldn't affect original
	samples[0].Velocity = 999.0
	if tracker.Samples[0].Velocity == 999.0 {
		t.Error("GetSamples should return a copy, not the original")
	}
}

func TestVelocityTracker_LastOutputAge(t *testing.T) {
	tracker := NewVelocityTracker("test")

	// Before any captures, should return 0
	if age := tracker.LastOutputAge(); age != 0 {
		t.Errorf("expected 0 age before captures, got %v", age)
	}

	// Add samples with no output (CharsAdded = 0)
	oldTime := time.Now().Add(-5 * time.Second)
	tracker.mu.Lock()
	tracker.Samples = append(tracker.Samples, VelocitySample{
		Timestamp:  oldTime,
		CharsAdded: 0,
		Velocity:   0,
	})
	tracker.LastCaptureAt = time.Now()
	tracker.mu.Unlock()

	// With no output, should return time since oldest sample (approx 5 seconds)
	age := tracker.LastOutputAge()
	if age < 4*time.Second || age > 6*time.Second {
		t.Errorf("expected ~5s age with no output, got %v", age)
	}

	// Now add a sample with output
	recentTime := time.Now().Add(-1 * time.Second)
	tracker.mu.Lock()
	tracker.Samples = append(tracker.Samples, VelocitySample{
		Timestamp:  recentTime,
		CharsAdded: 10,
		Velocity:   5.0,
	})
	tracker.mu.Unlock()

	// Should now return time since the sample with output (approx 1 second)
	age = tracker.LastOutputAge()
	if age > 2*time.Second {
		t.Errorf("expected ~1s age after output, got %v", age)
	}
}

func TestVelocityTracker_LastOutputTime(t *testing.T) {
	tracker := NewVelocityTracker("test")

	// Before any output, should return zero time
	if lt := tracker.LastOutputTime(); !lt.IsZero() {
		t.Errorf("expected zero time before output, got %v", lt)
	}

	// Add sample with output
	outputTime := time.Now().Add(-2 * time.Second)
	tracker.mu.Lock()
	tracker.Samples = append(tracker.Samples, VelocitySample{
		Timestamp:  outputTime,
		CharsAdded: 10,
		Velocity:   5.0,
	})
	tracker.mu.Unlock()

	// Should return the timestamp of that sample
	lt := tracker.LastOutputTime()
	if lt != outputTime {
		t.Errorf("expected %v, got %v", outputTime, lt)
	}
}

func TestVelocityTracker_Reset(t *testing.T) {
	tracker := NewVelocityTracker("test")

	// Add some state
	tracker.mu.Lock()
	tracker.addSampleLocked(VelocitySample{Velocity: 5.0})
	tracker.LastCapture = "some content"
	tracker.LastCaptureAt = time.Now()
	tracker.mu.Unlock()

	tracker.Reset()

	if len(tracker.Samples) != 0 {
		t.Errorf("expected empty samples after reset, got %d", len(tracker.Samples))
	}
	if tracker.LastCapture != "" {
		t.Errorf("expected empty LastCapture after reset")
	}
	if !tracker.LastCaptureAt.IsZero() {
		t.Errorf("expected zero LastCaptureAt after reset")
	}
}

func TestVelocityManager_GetOrCreate(t *testing.T) {
	vm := NewVelocityManager()

	// First call should create
	tracker1 := vm.GetOrCreate("pane1")
	if tracker1 == nil {
		t.Fatal("expected tracker, got nil")
	}
	if tracker1.PaneID != "pane1" {
		t.Errorf("expected pane1, got %s", tracker1.PaneID)
	}

	// Second call should return same tracker
	tracker2 := vm.GetOrCreate("pane1")
	if tracker1 != tracker2 {
		t.Error("expected same tracker instance")
	}

	// Different pane should create new tracker
	tracker3 := vm.GetOrCreate("pane2")
	if tracker3 == tracker1 {
		t.Error("expected different tracker for different pane")
	}
}

func TestVelocityManager_Get(t *testing.T) {
	vm := NewVelocityManager()

	// Non-existent should return nil
	if tracker := vm.Get("nonexistent"); tracker != nil {
		t.Error("expected nil for non-existent pane")
	}

	// Create and get
	vm.GetOrCreate("pane1")
	if tracker := vm.Get("pane1"); tracker == nil {
		t.Error("expected tracker, got nil")
	}
}

func TestVelocityManager_Remove(t *testing.T) {
	vm := NewVelocityManager()

	vm.GetOrCreate("pane1")
	vm.GetOrCreate("pane2")

	if vm.TrackerCount() != 2 {
		t.Errorf("expected 2 trackers, got %d", vm.TrackerCount())
	}

	vm.Remove("pane1")

	if vm.TrackerCount() != 1 {
		t.Errorf("expected 1 tracker after remove, got %d", vm.TrackerCount())
	}

	if vm.Get("pane1") != nil {
		t.Error("expected nil after remove")
	}

	if vm.Get("pane2") == nil {
		t.Error("expected pane2 to still exist")
	}
}

func TestVelocityManager_GetAllVelocities(t *testing.T) {
	vm := NewVelocityManager()

	tracker1 := vm.GetOrCreate("pane1")
	tracker2 := vm.GetOrCreate("pane2")

	// Add samples with known velocities
	tracker1.mu.Lock()
	tracker1.addSampleLocked(VelocitySample{Velocity: 5.0})
	tracker1.mu.Unlock()

	tracker2.mu.Lock()
	tracker2.addSampleLocked(VelocitySample{Velocity: 10.0})
	tracker2.mu.Unlock()

	velocities := vm.GetAllVelocities()

	if len(velocities) != 2 {
		t.Errorf("expected 2 velocities, got %d", len(velocities))
	}

	if velocities["pane1"] != 5.0 {
		t.Errorf("expected pane1 velocity 5.0, got %f", velocities["pane1"])
	}

	if velocities["pane2"] != 10.0 {
		t.Errorf("expected pane2 velocity 10.0, got %f", velocities["pane2"])
	}
}

func TestVelocityManager_Clear(t *testing.T) {
	vm := NewVelocityManager()

	vm.GetOrCreate("pane1")
	vm.GetOrCreate("pane2")
	vm.GetOrCreate("pane3")

	vm.Clear()

	if vm.TrackerCount() != 0 {
		t.Errorf("expected 0 trackers after clear, got %d", vm.TrackerCount())
	}
}

func TestVelocityManager_TrackerCount(t *testing.T) {
	vm := NewVelocityManager()

	if vm.TrackerCount() != 0 {
		t.Errorf("expected 0, got %d", vm.TrackerCount())
	}

	vm.GetOrCreate("pane1")
	if vm.TrackerCount() != 1 {
		t.Errorf("expected 1, got %d", vm.TrackerCount())
	}

	vm.GetOrCreate("pane2")
	vm.GetOrCreate("pane3")
	if vm.TrackerCount() != 3 {
		t.Errorf("expected 3, got %d", vm.TrackerCount())
	}
}

// =============================================================================
// State Classification Tests
// =============================================================================

func TestNewStateClassifier(t *testing.T) {
	sc := NewStateClassifier("test-pane", nil)

	if sc.velocityTracker == nil {
		t.Error("velocity tracker should be initialized")
	}
	if sc.patternLibrary == nil {
		t.Error("pattern library should be initialized")
	}
	if sc.currentState != StateUnknown {
		t.Errorf("expected initial state UNKNOWN, got %s", sc.currentState)
	}
	if sc.stallThreshold != DefaultStallThreshold {
		t.Errorf("expected default stall threshold")
	}
	if sc.hysteresisDuration != DefaultHysteresisDuration {
		t.Errorf("expected default hysteresis duration")
	}
}

func TestNewStateClassifierWithConfig(t *testing.T) {
	cfg := &ClassifierConfig{
		AgentType:          "claude",
		StallThreshold:     time.Minute,
		HysteresisDuration: 5 * time.Second,
	}

	sc := NewStateClassifier("test-pane", cfg)

	if sc.agentType != "claude" {
		t.Errorf("expected agent type 'claude', got %s", sc.agentType)
	}
	if sc.stallThreshold != time.Minute {
		t.Errorf("expected stall threshold 1m, got %v", sc.stallThreshold)
	}
	if sc.hysteresisDuration != 5*time.Second {
		t.Errorf("expected hysteresis 5s, got %v", sc.hysteresisDuration)
	}
}

func TestStateClassifier_classifyState(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	tests := []struct {
		name       string
		velocity   float64
		matches    []PatternMatch
		wantState  AgentState
		wantMinConf float64
	}{
		{
			name:       "error_pattern",
			velocity:   0,
			matches:    []PatternMatch{{Pattern: "rate_limit", Category: CategoryError}},
			wantState:  StateError,
			wantMinConf: 0.95,
		},
		{
			name:       "idle_prompt_low_velocity",
			velocity:   0.5,
			matches:    []PatternMatch{{Pattern: "claude_prompt", Category: CategoryIdle}},
			wantState:  StateWaiting,
			wantMinConf: 0.90,
		},
		{
			name:       "thinking_pattern",
			velocity:   0,
			matches:    []PatternMatch{{Pattern: "thinking_text", Category: CategoryThinking}},
			wantState:  StateThinking,
			wantMinConf: 0.80,
		},
		{
			name:       "high_velocity",
			velocity:   15.0,
			matches:    nil,
			wantState:  StateGenerating,
			wantMinConf: 0.85,
		},
		{
			name:       "medium_velocity",
			velocity:   5.0,
			matches:    nil,
			wantState:  StateGenerating,
			wantMinConf: 0.70,
		},
		{
			name:       "unknown_insufficient_signals",
			velocity:   0.5,
			matches:    nil,
			wantState:  StateUnknown,
			wantMinConf: 0.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, conf, _ := sc.classifyState(tt.velocity, tt.matches)

			if state != tt.wantState {
				t.Errorf("expected state %s, got %s", tt.wantState, state)
			}
			if conf < tt.wantMinConf {
				t.Errorf("expected confidence >= %f, got %f", tt.wantMinConf, conf)
			}
		})
	}
}

func TestStateClassifier_applyHysteresis_ErrorImmediate(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	// Set initial state
	sc.currentState = StateGenerating
	sc.stateSince = time.Now().Add(-time.Hour)

	// Error should transition immediately
	result := sc.applyHysteresis(StateError, 0.95, "test_error")

	if result != StateError {
		t.Errorf("expected immediate ERROR transition, got %s", result)
	}
	if sc.currentState != StateError {
		t.Errorf("current state should be ERROR")
	}
	if len(sc.stateHistory) != 1 {
		t.Errorf("expected 1 transition in history, got %d", len(sc.stateHistory))
	}
}

func TestStateClassifier_applyHysteresis_FirstClassificationImmediate(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	// First classification should transition immediately (except to UNKNOWN)
	// This ensures single-shot queries like PrintActivity get useful results
	result := sc.applyHysteresis(StateWaiting, 0.90, "idle_prompt")

	if result != StateWaiting {
		t.Errorf("expected immediate transition to WAITING on first classification, got %s", result)
	}
	if sc.currentState != StateWaiting {
		t.Errorf("current state should be WAITING, got %s", sc.currentState)
	}
	if len(sc.stateHistory) != 1 {
		t.Errorf("expected 1 transition in history, got %d", len(sc.stateHistory))
	}

	// After first classification, hysteresis should apply normally
	// Reset for second test
	sc2 := NewStateClassifier("test2", nil)

	// First call with GENERATING
	result = sc2.applyHysteresis(StateGenerating, 0.85, "high_velocity")
	if result != StateGenerating {
		t.Errorf("expected immediate GENERATING on first call, got %s", result)
	}

	// Second call with WAITING should NOT transition immediately (hysteresis)
	result = sc2.applyHysteresis(StateWaiting, 0.90, "idle_prompt")
	if result != StateGenerating {
		t.Errorf("expected hysteresis to keep GENERATING, got %s", result)
	}
	if sc2.pendingState != StateWaiting {
		t.Errorf("expected WAITING as pending state, got %s", sc2.pendingState)
	}
}

func TestStateClassifier_applyHysteresis_RequiresStability(t *testing.T) {
	sc := NewStateClassifier("test", &ClassifierConfig{
		HysteresisDuration: 2 * time.Second,
	})

	// Set initial state
	sc.currentState = StateGenerating

	// First call starts pending
	result := sc.applyHysteresis(StateWaiting, 0.90, "idle")
	if result != StateGenerating {
		t.Error("should stay in GENERATING until stable")
	}
	if sc.pendingState != StateWaiting {
		t.Error("should have WAITING as pending")
	}

	// Should still be GENERATING
	result = sc.applyHysteresis(StateWaiting, 0.90, "idle")
	if result != StateGenerating {
		t.Error("should still be GENERATING (hysteresis not elapsed)")
	}

	// Simulate time passing (set pendingSince to past)
	sc.pendingSince = time.Now().Add(-3 * time.Second)

	// Now should transition
	result = sc.applyHysteresis(StateWaiting, 0.90, "idle")
	if result != StateWaiting {
		t.Errorf("expected WAITING after hysteresis, got %s", result)
	}
}

func TestStateClassifier_recordTransition(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	// Record transitions up to max
	for i := 0; i < MaxStateHistory+5; i++ {
		sc.recordTransition(StateGenerating, StateWaiting, 0.90, "test")
	}

	// Should be capped at MaxStateHistory
	if len(sc.stateHistory) != MaxStateHistory {
		t.Errorf("expected %d transitions, got %d", MaxStateHistory, len(sc.stateHistory))
	}
}

func TestStateClassifier_CurrentState(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	if sc.CurrentState() != StateUnknown {
		t.Errorf("expected UNKNOWN, got %s", sc.CurrentState())
	}

	sc.currentState = StateGenerating
	if sc.CurrentState() != StateGenerating {
		t.Errorf("expected GENERATING, got %s", sc.CurrentState())
	}
}

func TestStateClassifier_Reset(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	// Add some state
	sc.currentState = StateGenerating
	sc.recordTransition(StateUnknown, StateGenerating, 0.85, "test")
	sc.pendingState = StateWaiting
	sc.lastPatterns = []string{"pattern1"}

	sc.Reset()

	if sc.currentState != StateUnknown {
		t.Errorf("expected UNKNOWN after reset")
	}
	if len(sc.stateHistory) != 0 {
		t.Error("expected empty history after reset")
	}
	if sc.pendingState != "" {
		t.Error("expected empty pending state after reset")
	}
	if sc.lastPatterns != nil {
		t.Error("expected nil patterns after reset")
	}
}

func TestStateClassifier_SetAgentType(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	sc.SetAgentType("codex")
	if sc.agentType != "codex" {
		t.Errorf("expected 'codex', got %s", sc.agentType)
	}
}

func TestStateClassifier_GetStateHistory(t *testing.T) {
	sc := NewStateClassifier("test", nil)

	sc.recordTransition(StateUnknown, StateGenerating, 0.85, "test1")
	sc.recordTransition(StateGenerating, StateWaiting, 0.90, "test2")

	history := sc.GetStateHistory()

	if len(history) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(history))
	}

	// Modifying copy shouldn't affect original
	history = nil
	if len(sc.stateHistory) != 2 {
		t.Error("GetStateHistory should return a copy")
	}
}

func TestStateClassifier_StateDuration(t *testing.T) {
	sc := NewStateClassifier("test", nil)
	sc.stateSince = time.Now().Add(-10 * time.Second)

	duration := sc.StateDuration()

	// Should be approximately 10 seconds (allow some margin)
	if duration < 9*time.Second || duration > 11*time.Second {
		t.Errorf("expected ~10s duration, got %v", duration)
	}
}

func TestActivityMonitor_GetOrCreate(t *testing.T) {
	am := NewActivityMonitor(nil)

	sc1 := am.GetOrCreate("pane1")
	if sc1 == nil {
		t.Fatal("expected classifier, got nil")
	}

	sc2 := am.GetOrCreate("pane1")
	if sc1 != sc2 {
		t.Error("should return same classifier")
	}

	sc3 := am.GetOrCreate("pane2")
	if sc3 == sc1 {
		t.Error("should create new classifier for different pane")
	}
}

func TestActivityMonitor_Get(t *testing.T) {
	am := NewActivityMonitor(nil)

	// Non-existent
	if am.Get("nonexistent") != nil {
		t.Error("expected nil for non-existent")
	}

	am.GetOrCreate("pane1")
	if am.Get("pane1") == nil {
		t.Error("expected classifier")
	}
}

func TestActivityMonitor_Remove(t *testing.T) {
	am := NewActivityMonitor(nil)

	am.GetOrCreate("pane1")
	am.GetOrCreate("pane2")

	am.Remove("pane1")

	if am.Get("pane1") != nil {
		t.Error("pane1 should be removed")
	}
	if am.Get("pane2") == nil {
		t.Error("pane2 should still exist")
	}
}

func TestActivityMonitor_GetAllStates(t *testing.T) {
	am := NewActivityMonitor(nil)

	sc1 := am.GetOrCreate("pane1")
	sc2 := am.GetOrCreate("pane2")

	sc1.mu.Lock()
	sc1.currentState = StateGenerating
	sc1.mu.Unlock()

	sc2.mu.Lock()
	sc2.currentState = StateWaiting
	sc2.mu.Unlock()

	states := am.GetAllStates()

	if states["pane1"] != StateGenerating {
		t.Errorf("expected GENERATING, got %s", states["pane1"])
	}
	if states["pane2"] != StateWaiting {
		t.Errorf("expected WAITING, got %s", states["pane2"])
	}
}

func TestActivityMonitor_Clear(t *testing.T) {
	am := NewActivityMonitor(nil)

	am.GetOrCreate("pane1")
	am.GetOrCreate("pane2")

	am.Clear()

	if am.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", am.Count())
	}
}

func TestActivityMonitor_Count(t *testing.T) {
	am := NewActivityMonitor(nil)

	if am.Count() != 0 {
		t.Errorf("expected 0, got %d", am.Count())
	}

	am.GetOrCreate("pane1")
	am.GetOrCreate("pane2")

	if am.Count() != 2 {
		t.Errorf("expected 2, got %d", am.Count())
	}
}

// =============================================================================
// Activity API Tests
// =============================================================================

func TestActivityOptions(t *testing.T) {
	opts := ActivityOptions{
		Session:    "test-session",
		Panes:      []string{"1", "2"},
		AgentTypes: []string{"claude", "codex"},
	}

	if opts.Session != "test-session" {
		t.Errorf("expected session test-session, got %s", opts.Session)
	}
	if len(opts.Panes) != 2 {
		t.Errorf("expected 2 panes, got %d", len(opts.Panes))
	}
	if len(opts.AgentTypes) != 2 {
		t.Errorf("expected 2 agent types, got %d", len(opts.AgentTypes))
	}
}

func TestActivityOutput(t *testing.T) {
	output := ActivityOutput{
		RobotResponse: NewRobotResponse(true),
		Session:       "test",
		Agents:        []AgentActivityInfo{},
		Summary: ActivitySummary{
			TotalAgents: 0,
			ByState:     make(map[string]int),
		},
	}

	if !output.Success {
		t.Error("expected success to be true")
	}
	if output.Session != "test" {
		t.Errorf("expected session test, got %s", output.Session)
	}
}

func TestAgentActivityInfo(t *testing.T) {
	info := AgentActivityInfo{
		Pane:             "1",
		PaneIdx:          1,
		AgentType:        "claude",
		State:            "WAITING",
		Confidence:       0.85,
		Velocity:         0.0,
		DetectedPatterns: []string{"claude_prompt"},
	}

	if info.Pane != "1" {
		t.Errorf("expected pane 1, got %s", info.Pane)
	}
	if info.State != "WAITING" {
		t.Errorf("expected state WAITING, got %s", info.State)
	}
	if info.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", info.Confidence)
	}
}

func TestActivitySummary(t *testing.T) {
	summary := ActivitySummary{
		TotalAgents: 4,
		ByState: map[string]int{
			"WAITING":    2,
			"GENERATING": 1,
			"ERROR":      1,
		},
	}

	if summary.TotalAgents != 4 {
		t.Errorf("expected 4 total agents, got %d", summary.TotalAgents)
	}
	if summary.ByState["WAITING"] != 2 {
		t.Errorf("expected 2 waiting, got %d", summary.ByState["WAITING"])
	}
}

func TestGenerateActivityHints(t *testing.T) {
	tests := []struct {
		name            string
		available       []string
		busy            []string
		problem         []string
		summary         ActivitySummary
		wantSummaryHas  string
		wantSuggestions int
	}{
		{
			name:      "no_agents",
			available: []string{},
			busy:      []string{},
			problem:   []string{},
			summary: ActivitySummary{
				TotalAgents: 0,
				ByState:     map[string]int{},
			},
			wantSummaryHas:  "No agents",
			wantSuggestions: 1,
		},
		{
			name:      "all_available",
			available: []string{"1", "2"},
			busy:      []string{},
			problem:   []string{},
			summary: ActivitySummary{
				TotalAgents: 2,
				ByState:     map[string]int{"WAITING": 2},
			},
			wantSummaryHas:  "2 available",
			wantSuggestions: 2, // "all idle" + "send work"
		},
		{
			name:      "all_busy",
			available: []string{},
			busy:      []string{"1", "2"},
			problem:   []string{},
			summary: ActivitySummary{
				TotalAgents: 2,
				ByState:     map[string]int{"GENERATING": 2},
			},
			wantSummaryHas:  "2 busy",
			wantSuggestions: 1, // "all busy"
		},
		{
			name:      "with_problems",
			available: []string{"1"},
			busy:      []string{"2"},
			problem:   []string{"3"},
			summary: ActivitySummary{
				TotalAgents: 3,
				ByState:     map[string]int{"WAITING": 1, "GENERATING": 1, "ERROR": 1},
			},
			wantSummaryHas:  "1 problems",
			wantSuggestions: 2, // "check error" + "send work"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := generateActivityHints(tt.available, tt.busy, tt.problem, tt.summary)

			if hints == nil {
				t.Fatal("expected hints, got nil")
			}

			if tt.wantSummaryHas != "" && !strings.Contains(hints.Summary, tt.wantSummaryHas) {
				t.Errorf("expected summary to contain %q, got %q", tt.wantSummaryHas, hints.Summary)
			}

			if len(hints.SuggestedActions) < tt.wantSuggestions {
				t.Errorf("expected at least %d suggestions, got %d: %v",
					tt.wantSuggestions, len(hints.SuggestedActions), hints.SuggestedActions)
			}
		})
	}
}

func TestNormalizeAgentType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude", "claude"},
		{"Claude", "claude"},
		{"cc", "claude"},
		{"claude-code", "claude"},
		{"codex", "codex"},
		{"cod", "codex"},
		{"codex-cli", "codex"},
		{"gemini", "gemini"},
		{"gmi", "gemini"},
		{"gemini-cli", "gemini"},
		{"cursor", "cursor"},
		{"Unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeAgentType(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAgentType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestActivityAgentHints(t *testing.T) {
	hints := &ActivityAgentHints{
		Summary:          "3 agents: 1 available, 1 busy, 1 problems",
		AvailableAgents:  []string{"1"},
		BusyAgents:       []string{"2"},
		ProblemAgents:    []string{"3"},
		SuggestedActions: []string{"Check error agents", "Send work"},
	}

	if hints.Summary == "" {
		t.Error("expected summary to be set")
	}
	if len(hints.AvailableAgents) != 1 {
		t.Errorf("expected 1 available agent, got %d", len(hints.AvailableAgents))
	}
	if len(hints.BusyAgents) != 1 {
		t.Errorf("expected 1 busy agent, got %d", len(hints.BusyAgents))
	}
	if len(hints.ProblemAgents) != 1 {
		t.Errorf("expected 1 problem agent, got %d", len(hints.ProblemAgents))
	}
}


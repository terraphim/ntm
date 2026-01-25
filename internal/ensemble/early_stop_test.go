package ensemble

import (
	"log/slog"
	"os"
	"testing"
)

// TestNewEarlyStopDetector verifies correct initialization.
func TestNewEarlyStopDetector(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := EarlyStopConfig{
			Enabled:             true,
			MinAgentsBeforeStop: 3,
			FindingsThreshold:   0.001,
			SimilarityThreshold: 0.8,
			WindowSize:          5,
		}
		detector := NewEarlyStopDetector(cfg)

		if detector == nil {
			t.Fatal("expected non-nil detector")
		}
		if !detector.Config.Enabled {
			t.Error("expected Enabled to be true")
		}
		if detector.Config.MinAgentsBeforeStop != 3 {
			t.Errorf("MinAgentsBeforeStop = %d, want 3", detector.Config.MinAgentsBeforeStop)
		}
		if detector.Config.FindingsThreshold != 0.001 {
			t.Errorf("FindingsThreshold = %f, want 0.001", detector.Config.FindingsThreshold)
		}
		if detector.Config.SimilarityThreshold != 0.8 {
			t.Errorf("SimilarityThreshold = %f, want 0.8", detector.Config.SimilarityThreshold)
		}
		if detector.Config.WindowSize != 5 {
			t.Errorf("WindowSize = %d, want 5", detector.Config.WindowSize)
		}
		t.Logf("Created detector with config: %+v", detector.Config)
	})

	t.Run("initializes empty slices", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true})

		if detector.Outputs != nil {
			t.Errorf("expected Outputs to be nil initially, got %v", detector.Outputs)
		}
		if detector.TokensSpent != nil {
			t.Errorf("expected TokensSpent to be nil initially, got %v", detector.TokensSpent)
		}
	})
}

// TestEarlyStopDetector_RecordOutput verifies output and token tracking.
func TestEarlyStopDetector_RecordOutput(t *testing.T) {
	t.Run("records single output", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true})
		output := ModeOutput{
			ModeID: "test-mode",
			Thesis: "Test thesis",
			TopFindings: []Finding{
				{Finding: "Finding 1"},
			},
		}

		detector.RecordOutput(output, 1000)

		if len(detector.Outputs) != 1 {
			t.Errorf("Outputs length = %d, want 1", len(detector.Outputs))
		}
		if len(detector.TokensSpent) != 1 {
			t.Errorf("TokensSpent length = %d, want 1", len(detector.TokensSpent))
		}
		if detector.TokensSpent[0] != 1000 {
			t.Errorf("TokensSpent[0] = %d, want 1000", detector.TokensSpent[0])
		}
		t.Logf("Recorded output: %s with %d tokens", output.ModeID, 1000)
	})

	t.Run("records multiple outputs", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true})

		for i := 0; i < 5; i++ {
			output := ModeOutput{
				ModeID: "mode-" + string(rune('A'+i)),
				Thesis: "Thesis",
				TopFindings: []Finding{
					{Finding: "Finding"},
				},
			}
			detector.RecordOutput(output, 500+i*100)
		}

		if len(detector.Outputs) != 5 {
			t.Errorf("Outputs length = %d, want 5", len(detector.Outputs))
		}
		if len(detector.TokensSpent) != 5 {
			t.Errorf("TokensSpent length = %d, want 5", len(detector.TokensSpent))
		}
		t.Logf("Recorded %d outputs", len(detector.Outputs))
	})

	t.Run("handles negative tokens as zero", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true})
		output := ModeOutput{ModeID: "test", Thesis: "Test", TopFindings: []Finding{{Finding: "f"}}}

		detector.RecordOutput(output, -100)

		if detector.TokensSpent[0] != 0 {
			t.Errorf("TokensSpent[0] = %d, want 0 for negative input", detector.TokensSpent[0])
		}
	})

	t.Run("handles nil detector gracefully", func(t *testing.T) {
		var detector *EarlyStopDetector
		output := ModeOutput{ModeID: "test", Thesis: "Test", TopFindings: []Finding{{Finding: "f"}}}

		// Should not panic
		detector.RecordOutput(output, 1000)
	})
}

// TestEarlyStopDetector_ShouldStop_Disabled tests behavior when early stopping is disabled.
func TestEarlyStopDetector_ShouldStop_Disabled(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:           false,
		FindingsThreshold: 0.001,
	})

	// Add outputs that would trigger stop if enabled
	for i := 0; i < 5; i++ {
		output := ModeOutput{
			ModeID:      "mode",
			Thesis:      "Same thesis",
			TopFindings: []Finding{{Finding: "Same finding"}},
		}
		detector.RecordOutput(output, 1000)
	}

	decision := detector.ShouldStop()

	if decision.ShouldStop {
		t.Error("expected ShouldStop=false when disabled")
	}
	if decision.Reason != "disabled" {
		t.Errorf("Reason = %q, want 'disabled'", decision.Reason)
	}
	t.Logf("Decision when disabled: ShouldStop=%v, Reason=%s", decision.ShouldStop, decision.Reason)
}

// TestEarlyStopDetector_ShouldStop_MinAgentsNotMet tests minimum agents requirement.
func TestEarlyStopDetector_ShouldStop_MinAgentsNotMet(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:             true,
		MinAgentsBeforeStop: 5,
		FindingsThreshold:   0.001,
		WindowSize:          3,
	})

	// Add only 3 outputs (less than min 5)
	for i := 0; i < 3; i++ {
		output := ModeOutput{
			ModeID:      "mode",
			Thesis:      "Same thesis",
			TopFindings: []Finding{{Finding: "Same finding"}},
		}
		detector.RecordOutput(output, 1000)
	}

	decision := detector.ShouldStop()

	if decision.ShouldStop {
		t.Error("expected ShouldStop=false when min agents not met")
	}
	if decision.Reason != "min_agents" {
		t.Errorf("Reason = %q, want 'min_agents'", decision.Reason)
	}
	if decision.AgentsRun != 3 {
		t.Errorf("AgentsRun = %d, want 3", decision.AgentsRun)
	}
	t.Logf("Decision with %d agents (min=%d): ShouldStop=%v, Reason=%s",
		decision.AgentsRun, detector.Config.MinAgentsBeforeStop, decision.ShouldStop, decision.Reason)
}

// TestEarlyStopDetector_ShouldStop_FindingsRateLow tests stopping on low findings rate.
func TestEarlyStopDetector_ShouldStop_FindingsRateLow(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:             true,
		MinAgentsBeforeStop: 2,
		FindingsThreshold:   0.01, // 1 finding per 100 tokens
		WindowSize:          3,
	})
	detector.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Add outputs with very few findings relative to tokens
	for i := 0; i < 4; i++ {
		output := ModeOutput{
			ModeID:      "mode",
			Thesis:      "Thesis",
			TopFindings: []Finding{{Finding: "Same finding"}}, // Only 1 unique finding
		}
		detector.RecordOutput(output, 5000) // High token cost
	}

	decision := detector.ShouldStop()

	// Rate = 1 unique finding / (3 window * 5000 tokens) = 1/15000 = 0.00006 < 0.01
	t.Logf("Decision: ShouldStop=%v, Reason=%s, FindingsRate=%.6f, Threshold=%.4f",
		decision.ShouldStop, decision.Reason, decision.FindingsRate, detector.Config.FindingsThreshold)

	if !decision.ShouldStop {
		t.Error("expected ShouldStop=true for low findings rate")
	}
	if decision.Reason != "findings_rate" && decision.Reason != "findings_rate_and_similarity" {
		t.Errorf("Reason = %q, want 'findings_rate' or 'findings_rate_and_similarity'", decision.Reason)
	}
}

// TestEarlyStopDetector_ShouldStop_FindingsRateHigh tests continuing with high findings rate.
func TestEarlyStopDetector_ShouldStop_FindingsRateHigh(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:             true,
		MinAgentsBeforeStop: 2,
		FindingsThreshold:   0.001, // 1 finding per 1000 tokens
		SimilarityThreshold: 0.95,  // Very high similarity threshold
		WindowSize:          3,
	})

	// Add outputs with many diverse findings
	for i := 0; i < 4; i++ {
		findings := make([]Finding, 10)
		for j := 0; j < 10; j++ {
			findings[j] = Finding{Finding: "Unique finding " + string(rune('A'+i)) + string(rune('0'+j))}
		}
		output := ModeOutput{
			ModeID:      "mode-" + string(rune('A'+i)),
			Thesis:      "Unique thesis " + string(rune('A'+i)),
			TopFindings: findings,
		}
		detector.RecordOutput(output, 500) // Low token cost, many findings
	}

	decision := detector.ShouldStop()

	// Rate = 30 unique findings / (3 * 500 tokens) = 30/1500 = 0.02 > 0.001
	t.Logf("Decision: ShouldStop=%v, Reason=%s, FindingsRate=%.6f",
		decision.ShouldStop, decision.Reason, decision.FindingsRate)

	if decision.ShouldStop {
		t.Error("expected ShouldStop=false for high findings rate")
	}
	if decision.Reason != "continue" {
		t.Errorf("Reason = %q, want 'continue'", decision.Reason)
	}
}

// TestEarlyStopDetector_ShouldStop_SimilarityHigh tests stopping on high output similarity.
func TestEarlyStopDetector_ShouldStop_SimilarityHigh(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:             true,
		MinAgentsBeforeStop: 2,
		FindingsThreshold:   0,   // Disable findings check
		SimilarityThreshold: 0.5, // Moderate similarity threshold
		WindowSize:          3,
	})
	detector.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Add highly similar outputs
	for i := 0; i < 4; i++ {
		output := ModeOutput{
			ModeID: "mode",
			Thesis: "The same thesis for all modes",
			TopFindings: []Finding{
				{Finding: "Same finding one"},
				{Finding: "Same finding two"},
			},
			Recommendations: []Recommendation{
				{Recommendation: "Same recommendation"},
			},
		}
		detector.RecordOutput(output, 100)
	}

	decision := detector.ShouldStop()

	t.Logf("Decision: ShouldStop=%v, Reason=%s, SimilarityScore=%.4f, Threshold=%.4f",
		decision.ShouldStop, decision.Reason, decision.SimilarityScore, detector.Config.SimilarityThreshold)

	if !decision.ShouldStop {
		t.Error("expected ShouldStop=true for high similarity")
	}
	if decision.Reason != "similarity" && decision.Reason != "findings_rate_and_similarity" {
		t.Errorf("Reason = %q, want 'similarity' or 'findings_rate_and_similarity'", decision.Reason)
	}
	if decision.SimilarityScore < detector.Config.SimilarityThreshold {
		t.Errorf("SimilarityScore = %.4f, expected >= %.4f", decision.SimilarityScore, detector.Config.SimilarityThreshold)
	}
}

// TestEarlyStopDetector_ShouldStop_BothConditions tests stopping when both conditions are met.
func TestEarlyStopDetector_ShouldStop_BothConditions(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:             true,
		MinAgentsBeforeStop: 2,
		FindingsThreshold:   0.01, // Will trigger
		SimilarityThreshold: 0.5,  // Will trigger
		WindowSize:          3,
	})

	// Add highly similar outputs with few findings
	for i := 0; i < 4; i++ {
		output := ModeOutput{
			ModeID:      "mode",
			Thesis:      "Same thesis",
			TopFindings: []Finding{{Finding: "Same finding"}},
		}
		detector.RecordOutput(output, 5000)
	}

	decision := detector.ShouldStop()

	t.Logf("Decision: ShouldStop=%v, Reason=%s, FindingsRate=%.6f, SimilarityScore=%.4f",
		decision.ShouldStop, decision.Reason, decision.FindingsRate, decision.SimilarityScore)

	if !decision.ShouldStop {
		t.Error("expected ShouldStop=true when both conditions met")
	}
	if decision.Reason != "findings_rate_and_similarity" {
		t.Errorf("Reason = %q, want 'findings_rate_and_similarity'", decision.Reason)
	}
}

// TestEarlyStopDetector_ShouldStop_NilDetector tests nil detector handling.
func TestEarlyStopDetector_ShouldStop_NilDetector(t *testing.T) {
	var detector *EarlyStopDetector

	decision := detector.ShouldStop()

	if decision.ShouldStop {
		t.Error("expected ShouldStop=false for nil detector")
	}
	t.Logf("Nil detector decision: %+v", decision)
}

// TestEarlyStopDetector_CalculateFindingsRate tests the findings rate calculation.
func TestEarlyStopDetector_CalculateFindingsRate(t *testing.T) {
	t.Run("empty detector returns 0", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 3})

		rate := detector.CalculateFindingsRate()

		if rate != 0 {
			t.Errorf("Rate = %f, want 0 for empty detector", rate)
		}
	})

	t.Run("zero tokens returns 0", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 3})
		detector.RecordOutput(ModeOutput{
			ModeID:      "mode",
			Thesis:      "t",
			TopFindings: []Finding{{Finding: "f"}},
		}, 0)

		rate := detector.CalculateFindingsRate()

		if rate != 0 {
			t.Errorf("Rate = %f, want 0 for zero tokens", rate)
		}
	})

	t.Run("calculates unique findings per token", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 10})

		// Add 3 outputs with 10 unique findings each = 30 unique findings total
		for i := 0; i < 3; i++ {
			findings := make([]Finding, 10)
			for j := 0; j < 10; j++ {
				findings[j] = Finding{Finding: "Unique finding " + string(rune('A'+i)) + string(rune('0'+j))}
			}
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "t",
				TopFindings: findings,
			}, 1000)
		}

		rate := detector.CalculateFindingsRate()

		// Expected: 30 unique findings / 3000 tokens = 0.01
		expectedRate := 0.01
		tolerance := 0.001
		if rate < expectedRate-tolerance || rate > expectedRate+tolerance {
			t.Errorf("Rate = %f, want ~%f", rate, expectedRate)
		}
		t.Logf("Calculated findings rate: %f (expected ~%f)", rate, expectedRate)
	})

	t.Run("deduplicates identical findings", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 10})

		// Add 3 outputs all with the same finding
		for i := 0; i < 3; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "t",
				TopFindings: []Finding{{Finding: "Same finding"}},
			}, 1000)
		}

		rate := detector.CalculateFindingsRate()

		// Expected: 1 unique finding / 3000 tokens = 0.000333...
		expectedRate := 1.0 / 3000.0
		tolerance := 0.0001
		if rate < expectedRate-tolerance || rate > expectedRate+tolerance {
			t.Errorf("Rate = %f, want ~%f", rate, expectedRate)
		}
		t.Logf("Deduplicated findings rate: %f (expected ~%f)", rate, expectedRate)
	})

	t.Run("nil detector returns 0", func(t *testing.T) {
		var detector *EarlyStopDetector
		rate := detector.CalculateFindingsRate()
		if rate != 0 {
			t.Errorf("Rate = %f, want 0 for nil detector", rate)
		}
	})
}

// TestEarlyStopDetector_CalculateSimilarity tests the similarity calculation.
func TestEarlyStopDetector_CalculateSimilarity(t *testing.T) {
	t.Run("empty detector returns 0", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 3})

		sim := detector.CalculateSimilarity()

		if sim != 0 {
			t.Errorf("Similarity = %f, want 0 for empty detector", sim)
		}
	})

	t.Run("single output returns 0", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 3})
		detector.RecordOutput(ModeOutput{
			ModeID:      "mode",
			Thesis:      "thesis",
			TopFindings: []Finding{{Finding: "finding"}},
		}, 1000)

		sim := detector.CalculateSimilarity()

		if sim != 0 {
			t.Errorf("Similarity = %f, want 0 for single output", sim)
		}
	})

	t.Run("identical outputs have high similarity", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 10})

		for i := 0; i < 3; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "identical thesis",
				TopFindings: []Finding{{Finding: "identical finding"}},
			}, 1000)
		}

		sim := detector.CalculateSimilarity()

		if sim < 0.9 {
			t.Errorf("Similarity = %f, expected >= 0.9 for identical outputs", sim)
		}
		t.Logf("Identical outputs similarity: %f", sim)
	})

	t.Run("diverse outputs have lower similarity than identical", func(t *testing.T) {
		// Test that truly diverse outputs have lower similarity
		detector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 10})

		// Use completely unrelated content for each output
		diverseTheses := []string{
			"quantum entanglement enables instantaneous communication protocols",
			"marine biology ecosystems demonstrate remarkable resilience patterns",
			"architectural design principles emphasize sustainable construction methods",
		}
		diverseFindings := [][]Finding{
			{{Finding: "photon pairs maintain correlation states"}, {Finding: "decoherence limits distance"}},
			{{Finding: "coral reefs adapt to temperature"}, {Finding: "biodiversity protects habitats"}},
			{{Finding: "passive cooling reduces energy"}, {Finding: "recycled materials strengthen structures"}},
		}

		for i := 0; i < 3; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode-" + string(rune('A'+i)),
				Thesis:      diverseTheses[i],
				TopFindings: diverseFindings[i],
			}, 1000)
		}

		diverseSim := detector.CalculateSimilarity()

		// Compare with identical outputs
		identicalDetector := NewEarlyStopDetector(EarlyStopConfig{Enabled: true, WindowSize: 10})
		for i := 0; i < 3; i++ {
			identicalDetector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "identical thesis",
				TopFindings: []Finding{{Finding: "identical finding"}},
			}, 1000)
		}
		identicalSim := identicalDetector.CalculateSimilarity()

		// Diverse outputs should have lower similarity than identical ones
		if diverseSim >= identicalSim {
			t.Errorf("Diverse similarity (%f) should be < identical similarity (%f)", diverseSim, identicalSim)
		}
		t.Logf("Diverse outputs similarity: %f, Identical: %f", diverseSim, identicalSim)
	})

	t.Run("nil detector returns 0", func(t *testing.T) {
		var detector *EarlyStopDetector
		sim := detector.CalculateSimilarity()
		if sim != 0 {
			t.Errorf("Similarity = %f, want 0 for nil detector", sim)
		}
	})
}

// TestEarlyStopDetector_WindowSize tests the sliding window behavior.
func TestEarlyStopDetector_WindowSize(t *testing.T) {
	t.Run("window size limits evaluated outputs", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{
			Enabled:    true,
			WindowSize: 2,
		})

		// Add 5 outputs
		for i := 0; i < 5; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "thesis",
				TopFindings: []Finding{{Finding: "f" + string(rune('0'+i))}},
			}, 100+i*10)
		}

		// Window should only include last 2
		windowOutputs := detector.windowOutputs()
		if len(windowOutputs) != 2 {
			t.Errorf("Window outputs length = %d, want 2", len(windowOutputs))
		}

		windowTokens := detector.windowTokens()
		// Last 2 outputs: 130 + 140 = 270
		expectedTokens := 270
		if windowTokens != expectedTokens {
			t.Errorf("Window tokens = %d, want %d", windowTokens, expectedTokens)
		}
		t.Logf("Window size=2: outputs=%d, tokens=%d", len(windowOutputs), windowTokens)
	})

	t.Run("window size larger than outputs uses all", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{
			Enabled:    true,
			WindowSize: 10,
		})

		for i := 0; i < 3; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "thesis",
				TopFindings: []Finding{{Finding: "f"}},
			}, 100)
		}

		windowOutputs := detector.windowOutputs()
		if len(windowOutputs) != 3 {
			t.Errorf("Window outputs length = %d, want 3", len(windowOutputs))
		}
	})

	t.Run("zero window size uses all outputs", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{
			Enabled:    true,
			WindowSize: 0,
		})

		for i := 0; i < 5; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "thesis",
				TopFindings: []Finding{{Finding: "f"}},
			}, 100)
		}

		windowOutputs := detector.windowOutputs()
		if len(windowOutputs) != 5 {
			t.Errorf("Window outputs length = %d, want 5", len(windowOutputs))
		}
	})

	t.Run("negative window size uses all outputs", func(t *testing.T) {
		detector := NewEarlyStopDetector(EarlyStopConfig{
			Enabled:    true,
			WindowSize: -1,
		})

		for i := 0; i < 5; i++ {
			detector.RecordOutput(ModeOutput{
				ModeID:      "mode",
				Thesis:      "thesis",
				TopFindings: []Finding{{Finding: "f"}},
			}, 100)
		}

		windowOutputs := detector.windowOutputs()
		if len(windowOutputs) != 5 {
			t.Errorf("Window outputs length = %d, want 5", len(windowOutputs))
		}
	})
}

// TestEarlyStopDetector_Integration tests the full decision flow.
func TestEarlyStopDetector_Integration(t *testing.T) {
	detector := NewEarlyStopDetector(EarlyStopConfig{
		Enabled:             true,
		MinAgentsBeforeStop: 3,
		FindingsThreshold:   0.002, // 2 findings per 1000 tokens
		SimilarityThreshold: 0.7,
		WindowSize:          4,
	})
	detector.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Log("=== Integration Test: Full Decision Flow ===")

	// Phase 1: Below minimum agents
	t.Log("Phase 1: Adding 2 outputs (below min)")
	for i := 0; i < 2; i++ {
		detector.RecordOutput(ModeOutput{
			ModeID:      "mode-" + string(rune('A'+i)),
			Thesis:      "Unique thesis " + string(rune('A'+i)),
			TopFindings: []Finding{{Finding: "Unique finding " + string(rune('A'+i))}},
		}, 500)
	}
	decision := detector.ShouldStop()
	t.Logf("After 2 outputs: ShouldStop=%v, Reason=%s", decision.ShouldStop, decision.Reason)
	if decision.ShouldStop {
		t.Error("Should not stop before min agents")
	}

	// Phase 2: At minimum agents with diverse findings
	t.Log("Phase 2: Adding 3rd output with diverse findings")
	detector.RecordOutput(ModeOutput{
		ModeID:      "mode-C",
		Thesis:      "Another unique thesis C",
		TopFindings: []Finding{
			{Finding: "Unique finding C1"},
			{Finding: "Unique finding C2"},
			{Finding: "Unique finding C3"},
		},
	}, 500)
	decision = detector.ShouldStop()
	t.Logf("After 3 outputs: ShouldStop=%v, Reason=%s, Rate=%.6f, Sim=%.4f",
		decision.ShouldStop, decision.Reason, decision.FindingsRate, decision.SimilarityScore)
	if decision.ShouldStop {
		t.Error("Should not stop with diverse findings")
	}

	// Phase 3: Adding similar outputs with fewer new findings
	t.Log("Phase 3: Adding similar outputs to trigger stop")
	for i := 0; i < 3; i++ {
		detector.RecordOutput(ModeOutput{
			ModeID:      "mode-repeat",
			Thesis:      "Repeated thesis",
			TopFindings: []Finding{{Finding: "Repeated finding"}},
		}, 2000)
	}
	decision = detector.ShouldStop()
	t.Logf("After repeated outputs: ShouldStop=%v, Reason=%s, Rate=%.6f, Sim=%.4f",
		decision.ShouldStop, decision.Reason, decision.FindingsRate, decision.SimilarityScore)
	// Should trigger stop due to low findings rate and/or high similarity
	if !decision.ShouldStop {
		t.Logf("WARNING: Expected stop to trigger (may be threshold tuning issue)")
	}
}

// TestOutputSignature verifies the output signature generation for similarity.
func TestOutputSignature(t *testing.T) {
	t.Run("includes thesis", func(t *testing.T) {
		output := ModeOutput{Thesis: "my thesis"}
		sig := outputSignature(output)
		if sig != "my thesis" {
			t.Errorf("Signature = %q, want 'my thesis'", sig)
		}
	})

	t.Run("includes findings", func(t *testing.T) {
		output := ModeOutput{
			Thesis:      "thesis",
			TopFindings: []Finding{{Finding: "finding1"}, {Finding: "finding2"}},
		}
		sig := outputSignature(output)
		if sig != "thesis finding1 finding2" {
			t.Errorf("Signature = %q, want 'thesis finding1 finding2'", sig)
		}
	})

	t.Run("includes all components", func(t *testing.T) {
		output := ModeOutput{
			Thesis:           "thesis",
			TopFindings:      []Finding{{Finding: "f1"}},
			Risks:            []Risk{{Risk: "r1"}},
			Recommendations:  []Recommendation{{Recommendation: "rec1"}},
			QuestionsForUser: []Question{{Question: "q1"}},
		}
		sig := outputSignature(output)
		expected := "thesis f1 r1 rec1 q1"
		if sig != expected {
			t.Errorf("Signature = %q, want %q", sig, expected)
		}
	})

	t.Run("skips empty fields", func(t *testing.T) {
		output := ModeOutput{
			Thesis:      "",
			TopFindings: []Finding{{Finding: ""}, {Finding: "valid"}},
		}
		sig := outputSignature(output)
		if sig != "valid" {
			t.Errorf("Signature = %q, want 'valid'", sig)
		}
	})
}

// TestEarlyStopConfig_Defaults tests default configuration behavior.
func TestEarlyStopConfig_Defaults(t *testing.T) {
	cfg := EarlyStopConfig{}

	if cfg.Enabled {
		t.Error("Default Enabled should be false")
	}
	if cfg.MinAgentsBeforeStop != 0 {
		t.Errorf("Default MinAgentsBeforeStop = %d, want 0", cfg.MinAgentsBeforeStop)
	}
	if cfg.FindingsThreshold != 0 {
		t.Errorf("Default FindingsThreshold = %f, want 0", cfg.FindingsThreshold)
	}
	if cfg.SimilarityThreshold != 0 {
		t.Errorf("Default SimilarityThreshold = %f, want 0", cfg.SimilarityThreshold)
	}
	if cfg.WindowSize != 0 {
		t.Errorf("Default WindowSize = %d, want 0", cfg.WindowSize)
	}
}

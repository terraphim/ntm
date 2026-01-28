package robot

import (
	"testing"
)

func TestGetEnsemblePresets_Basic(t *testing.T) {
	t.Log("TEST: TestGetEnsemblePresets_Basic - starting")

	output, err := GetEnsemblePresets()
	if err != nil {
		t.Fatalf("GetEnsemblePresets failed: %v", err)
	}

	if !output.Success {
		t.Errorf("expected success=true, got false: %s", output.Error)
	}

	if output.Action != "ensemble_presets" {
		t.Errorf("expected action=ensemble_presets, got %s", output.Action)
	}

	// Should have embedded presets
	if output.Count == 0 {
		t.Error("expected at least one preset")
	}

	if output.Count != len(output.Presets) {
		t.Errorf("count mismatch: count=%d, presets=%d", output.Count, len(output.Presets))
	}

	t.Logf("TEST: TestGetEnsemblePresets_Basic - found %d presets", output.Count)
	t.Log("TEST: TestGetEnsemblePresets_Basic - assertion: presets loaded correctly")
}

func TestGetEnsemblePresets_PresetInfo(t *testing.T) {
	t.Log("TEST: TestGetEnsemblePresets_PresetInfo - starting")

	output, _ := GetEnsemblePresets()
	if len(output.Presets) == 0 {
		t.Fatal("expected at least one preset")
	}

	preset := output.Presets[0]

	// Verify required fields
	if preset.Name == "" {
		t.Error("preset Name is empty")
	}
	if preset.Description == "" {
		t.Error("preset Description is empty")
	}
	if len(preset.Modes) == 0 {
		t.Error("preset has no modes")
	}
	if preset.ModeCount != len(preset.Modes) {
		t.Errorf("mode count mismatch: count=%d, modes=%d", preset.ModeCount, len(preset.Modes))
	}
	if preset.Synthesis.Strategy == "" {
		t.Error("preset synthesis strategy is empty")
	}
	if preset.Budget.Total == 0 {
		t.Error("preset budget is zero")
	}

	// Tags should not be nil (even if empty)
	if preset.Tags == nil {
		t.Error("preset tags is nil, should be empty array")
	}

	t.Logf("TEST: TestGetEnsemblePresets_PresetInfo - preset: %s, modes: %d, budget: %d",
		preset.Name, preset.ModeCount, preset.Budget.Total)
	t.Log("TEST: TestGetEnsemblePresets_PresetInfo - assertion: preset info is complete")
}

func TestGetEnsemblePresets_KnownPresets(t *testing.T) {
	t.Log("TEST: TestGetEnsemblePresets_KnownPresets - starting")

	output, _ := GetEnsemblePresets()

	// Should have known embedded presets
	knownPresets := []string{"project-diagnosis", "bug-hunt", "idea-forge"}
	found := make(map[string]bool)

	for _, preset := range output.Presets {
		found[preset.Name] = true
	}

	for _, name := range knownPresets {
		if !found[name] {
			t.Errorf("expected known preset '%s' not found", name)
		}
	}

	t.Logf("TEST: TestGetEnsemblePresets_KnownPresets - found presets: %v", found)
	t.Log("TEST: TestGetEnsemblePresets_KnownPresets - assertion: known presets present")
}

func TestGetEnsemblePresets_AgentHints(t *testing.T) {
	t.Log("TEST: TestGetEnsemblePresets_AgentHints - starting")

	output, _ := GetEnsemblePresets()

	if output.AgentHints == nil {
		t.Log("TEST: no agent hints (may be expected if empty)")
		return
	}

	// If hints present, should have useful content
	if output.AgentHints.Summary == "" && len(output.AgentHints.SuggestedActions) == 0 {
		t.Error("agent hints present but empty")
	}

	if output.AgentHints.Summary != "" {
		t.Logf("TEST: hint summary: %s", output.AgentHints.Summary)
	}

	t.Log("TEST: TestGetEnsemblePresets_AgentHints - assertion: agent hints work")
}

func TestGetEnsemblePresets_ModeCodes(t *testing.T) {
	t.Log("TEST: TestGetEnsemblePresets_ModeCodes - starting")

	output, _ := GetEnsemblePresets()
	if len(output.Presets) == 0 {
		t.Fatal("expected at least one preset")
	}

	// Find a preset with modes
	var presetWithModes *PresetInfo
	for i := range output.Presets {
		if len(output.Presets[i].Modes) > 0 {
			presetWithModes = &output.Presets[i]
			break
		}
	}

	if presetWithModes == nil {
		t.Fatal("no preset with modes found")
	}

	// Mode codes should be present (either as codes like A1 or IDs)
	for _, modeCode := range presetWithModes.Modes {
		if modeCode == "" {
			t.Error("empty mode code in preset")
		}
	}

	t.Logf("TEST: TestGetEnsemblePresets_ModeCodes - preset %s modes: %v",
		presetWithModes.Name, presetWithModes.Modes)
	t.Log("TEST: TestGetEnsemblePresets_ModeCodes - assertion: mode codes present")
}

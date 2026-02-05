package ensemble

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// fallbackModeOutputText — missing branches
// ---------------------------------------------------------------------------

func TestFallbackModeOutputText_Nil(t *testing.T) {
	t.Parallel()

	got := fallbackModeOutputText(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestFallbackModeOutputText_FailureModes(t *testing.T) {
	t.Parallel()

	output := &ModeOutput{
		FailureModesToWatch: []FailureModeWarning{
			{Mode: "anchoring", Description: "over-reliance on first data"},
			{Mode: "groupthink", Description: "conformity pressure"},
		},
	}

	got := fallbackModeOutputText(output)
	if !strings.Contains(got, "Failure modes:") {
		t.Errorf("expected 'Failure modes:' header, got %q", got)
	}
	if !strings.Contains(got, "anchoring: over-reliance on first data") {
		t.Errorf("expected first failure mode, got %q", got)
	}
	if !strings.Contains(got, "groupthink: conformity pressure") {
		t.Errorf("expected second failure mode, got %q", got)
	}
}

func TestFallbackModeOutputText_Empty(t *testing.T) {
	t.Parallel()

	got := fallbackModeOutputText(&ModeOutput{})
	if got != "" {
		t.Errorf("expected empty for empty output, got %q", got)
	}
}

func TestFallbackModeOutputText_FindingWithoutReasoning(t *testing.T) {
	t.Parallel()

	output := &ModeOutput{
		TopFindings: []Finding{
			{Finding: "bare finding"},
		},
	}
	got := fallbackModeOutputText(output)
	if !strings.Contains(got, "- bare finding\n") {
		t.Errorf("expected finding without reasoning parens, got %q", got)
	}
	if strings.Contains(got, "(") {
		t.Errorf("expected no parentheses for empty reasoning, got %q", got)
	}
}

func TestFallbackModeOutputText_RiskWithoutMitigation(t *testing.T) {
	t.Parallel()

	output := &ModeOutput{
		Risks: []Risk{
			{Risk: "data loss"},
		},
	}
	got := fallbackModeOutputText(output)
	if !strings.Contains(got, "- data loss\n") {
		t.Errorf("expected risk without mitigation, got %q", got)
	}
	if strings.Contains(got, "mitigation") {
		t.Errorf("expected no mitigation text, got %q", got)
	}
}

func TestFallbackModeOutputText_RiskWithMitigation(t *testing.T) {
	t.Parallel()

	output := &ModeOutput{
		Risks: []Risk{
			{Risk: "data loss", Mitigation: "backups"},
		},
	}
	got := fallbackModeOutputText(output)
	if !strings.Contains(got, "| mitigation: backups") {
		t.Errorf("expected mitigation text, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// EstimateOutputTokens — branch coverage
// ---------------------------------------------------------------------------

func TestEstimateOutputTokens_Empty(t *testing.T) {
	t.Parallel()

	if got := EstimateOutputTokens(""); got != 0 {
		t.Errorf("expected 0 for empty string, got %d", got)
	}
	if got := EstimateOutputTokens("   "); got != 0 {
		t.Errorf("expected 0 for whitespace-only, got %d", got)
	}
}

func TestEstimateOutputTokens_NonEmpty(t *testing.T) {
	t.Parallel()

	got := EstimateOutputTokens("This is a test with several words that should produce some tokens.")
	if got <= 0 {
		t.Errorf("expected positive token count, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// EstimateModeOutputTokens — branch coverage
// ---------------------------------------------------------------------------

func TestEstimateModeOutputTokens_Nil(t *testing.T) {
	t.Parallel()

	if got := EstimateModeOutputTokens(nil); got != 0 {
		t.Errorf("expected 0 for nil, got %d", got)
	}
}

func TestEstimateModeOutputTokens_WithRaw(t *testing.T) {
	t.Parallel()

	output := &ModeOutput{
		RawOutput: "This is a test of raw output estimation.",
	}
	got := EstimateModeOutputTokens(output)
	if got <= 0 {
		t.Errorf("expected positive tokens for raw output, got %d", got)
	}
}

func TestEstimateModeOutputTokens_WithoutRaw(t *testing.T) {
	t.Parallel()

	output := &ModeOutput{
		ModeID: "deductive",
		Thesis: "The main conclusion is strong.",
		TopFindings: []Finding{
			{Finding: "key insight"},
		},
	}
	got := EstimateModeOutputTokens(output)
	if got <= 0 {
		t.Errorf("expected positive tokens for structured output, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// EstimateModeOutputsTokens — sums across outputs
// ---------------------------------------------------------------------------

func TestEstimateModeOutputsTokens(t *testing.T) {
	t.Parallel()

	outputs := []ModeOutput{
		{RawOutput: "first output text here"},
		{RawOutput: "second output text here with more words"},
	}
	total := EstimateModeOutputsTokens(outputs)
	if total <= 0 {
		t.Errorf("expected positive total, got %d", total)
	}

	single := EstimateModeOutputTokens(&outputs[0])
	if total <= single {
		t.Errorf("expected total (%d) > single (%d)", total, single)
	}
}

package cli

import (
	"strings"
	"testing"

	"github.com/Dicklesworthstone/ntm/internal/tmux"
)

// =============================================================================
// activity.go: detectAgentTypeFromPane
// =============================================================================

func TestDetectAgentTypeFromPane(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pane tmux.Pane
		want string
	}{
		{"claude", tmux.Pane{Type: tmux.AgentClaude}, "claude"},
		{"codex", tmux.Pane{Type: tmux.AgentCodex}, "codex"},
		{"gemini", tmux.Pane{Type: tmux.AgentGemini}, "gemini"},
		{"user", tmux.Pane{Type: tmux.AgentUser}, "user"},
		{"unknown", tmux.Pane{Type: tmux.AgentUnknown}, "unknown"},
		{"empty type", tmux.Pane{Type: ""}, "unknown"},
		{"arbitrary", tmux.Pane{Type: "something-else"}, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := detectAgentTypeFromPane(tc.pane)
			if got != tc.want {
				t.Errorf("detectAgentTypeFromPane(%v) = %q, want %q", tc.pane.Type, got, tc.want)
			}
		})
	}
}

// =============================================================================
// analytics.go: updateAgentStats
// =============================================================================

func TestUpdateAgentStats(t *testing.T) {
	t.Parallel()

	breakdown := make(map[string]AgentStats)

	// First update creates entry
	updateAgentStats(breakdown, "claude", 1, 5, 100)
	if s := breakdown["claude"]; s.Count != 1 || s.Prompts != 5 || s.TokensEst != 100 {
		t.Errorf("after first update: %+v", s)
	}

	// Second update accumulates
	updateAgentStats(breakdown, "claude", 2, 3, 200)
	if s := breakdown["claude"]; s.Count != 3 || s.Prompts != 8 || s.TokensEst != 300 {
		t.Errorf("after second update: %+v", s)
	}

	// Different agent type
	updateAgentStats(breakdown, "codex", 1, 1, 50)
	if s := breakdown["codex"]; s.Count != 1 || s.Prompts != 1 || s.TokensEst != 50 {
		t.Errorf("codex entry: %+v", s)
	}

	// Original unchanged
	if s := breakdown["claude"]; s.Count != 3 {
		t.Errorf("claude changed unexpectedly: %+v", s)
	}
}

// =============================================================================
// redaction_io.go: formatRedactionCategoryCounts
// =============================================================================

func TestFormatRedactionCategoryCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		categories map[string]int
		want       string
	}{
		{"nil", nil, ""},
		{"empty", map[string]int{}, ""},
		{"single", map[string]int{"PASSWORD": 3}, "PASSWORD=3"},
		{"multiple sorted", map[string]int{"TOKEN": 2, "API_KEY": 1, "PASSWORD": 3}, "API_KEY=1, PASSWORD=3, TOKEN=2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatRedactionCategoryCounts(tc.categories)
			if got != tc.want {
				t.Errorf("formatRedactionCategoryCounts(%v) = %q, want %q", tc.categories, got, tc.want)
			}
		})
	}
}

// =============================================================================
// redaction_io.go: redactionBlockedError.Error()
// =============================================================================

func TestRedactionBlockedError(t *testing.T) {
	t.Parallel()

	// With categories
	err := redactionBlockedError{
		summary: RedactionSummary{
			Categories: map[string]int{"PASSWORD": 2, "API_KEY": 1},
		},
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	if !strings.Contains(msg, "PASSWORD") {
		t.Errorf("expected error to mention PASSWORD, got %q", msg)
	}
	if !strings.Contains(msg, "API_KEY") {
		t.Errorf("expected error to mention API_KEY, got %q", msg)
	}

	// Without categories
	errEmpty := redactionBlockedError{
		summary: RedactionSummary{},
	}
	msgEmpty := errEmpty.Error()
	if !strings.Contains(msgEmpty, "refusing to proceed") {
		t.Errorf("expected 'refusing to proceed', got %q", msgEmpty)
	}
}

// =============================================================================
// health.go: truncateString
// =============================================================================

func TestTruncateStringHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hell…"},
		{"maxLen 1", "hello", 1, "h"},
		{"maxLen 0", "hello", 0, ""},
		{"empty string", "", 5, ""},
		{"unicode", "日本語テスト", 4, "日本語…"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateString(tc.s, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tc.s, tc.maxLen, got, tc.want)
			}
		})
	}
}

// =============================================================================
// agent_spec.go: AgentSpecsValue.Set, Type
// =============================================================================

func TestAgentSpecsValue_SetAndType(t *testing.T) {
	t.Parallel()

	var specs AgentSpecs
	val := NewAgentSpecsValue(AgentTypeClaude, &specs)

	if val.Type() != "N[:model]" {
		t.Errorf("Type() = %q, want %q", val.Type(), "N[:model]")
	}

	if err := val.Set("3"); err != nil {
		t.Fatalf("Set(3) error: %v", err)
	}
	if len(specs) != 1 || specs[0].Count != 3 {
		t.Errorf("after Set(3): specs = %+v", specs)
	}

	if err := val.Set("2:opus"); err != nil {
		t.Fatalf("Set(2:opus) error: %v", err)
	}
	if len(specs) != 2 || specs[1].Count != 2 || specs[1].Model != "opus" {
		t.Errorf("after Set(2:opus): specs = %+v", specs)
	}
}

// =============================================================================
// agent_spec.go: AgentSpecs.String
// =============================================================================

func TestAgentSpecsStringFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		specs AgentSpecs
		want  string
	}{
		{"empty", AgentSpecs{}, ""},
		{"single", AgentSpecs{{Type: AgentTypeClaude, Count: 1}}, "1"},
		{"with model", AgentSpecs{{Type: AgentTypeClaude, Count: 2, Model: "opus"}}, "2:opus"},
		{"multiple", AgentSpecs{
			{Type: AgentTypeClaude, Count: 1},
			{Type: AgentTypeCodex, Count: 3, Model: "gpt4"},
		}, "1,3:gpt4"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.specs.String()
			if got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

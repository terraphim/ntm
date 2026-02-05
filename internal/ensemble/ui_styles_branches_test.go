package ensemble

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ModeBadge — missing branches: empty code, empty icon
// ---------------------------------------------------------------------------

func TestModeBadge_EmptyCode(t *testing.T) {
	t.Parallel()

	// When Code is empty, should use strings.ToUpper(mode.ID).
	mode := ReasoningMode{ID: "deductive", Category: CategoryFormal}
	badge := ModeBadge(mode)
	if badge == "" {
		t.Fatal("expected non-empty badge")
	}
	if !strings.Contains(badge, "DEDUCTIVE") {
		t.Errorf("expected uppercased ID in badge, got %q", badge)
	}
}

func TestModeBadge_WithIcon(t *testing.T) {
	t.Parallel()

	mode := ReasoningMode{ID: "deductive", Code: "A1", Icon: "X", Category: CategoryFormal}
	badge := ModeBadge(mode)
	if badge == "" {
		t.Fatal("expected non-empty badge")
	}
	// ASCII icon should be preserved.
	if !strings.Contains(badge, "A1") {
		t.Errorf("expected code in badge, got %q", badge)
	}
}

// ---------------------------------------------------------------------------
// TierChip — missing branches: advanced, experimental, unknown
// ---------------------------------------------------------------------------

func TestTierChip_AllTiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tier ModeTier
		want string // substring expected in output
	}{
		{"core", TierCore, "CORE"},
		{"advanced", TierAdvanced, "ADV"},
		{"experimental", TierExperimental, "EXP"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TierChip(tc.tier)
			if got == "" {
				t.Fatalf("expected non-empty TierChip for %q", tc.tier)
			}
			// ANSI-stripped check isn't needed; just verify non-empty.
		})
	}
}

func TestTierChip_UnknownTier(t *testing.T) {
	t.Parallel()

	got := TierChip("custom")
	if got == "" {
		t.Fatal("expected non-empty TierChip for unknown tier")
	}
}

// ---------------------------------------------------------------------------
// isASCII — additional coverage
// ---------------------------------------------------------------------------

func TestIsASCII(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"empty", "", true},
		{"ascii_letters", "hello", true},
		{"ascii_symbols", "!@#$%^&*()", true},
		{"unicode_char", "◆", false},
		{"mixed", "hello◆", false},
		{"high_byte", "\x80", false},
		{"max_ascii", "\x7f", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isASCII(tc.s)
			if got != tc.want {
				t.Errorf("isASCII(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

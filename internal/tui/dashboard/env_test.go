package dashboard

import (
	"testing"
)

// =============================================================================
// envPositiveInt tests
// =============================================================================

func TestEnvPositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantVal int
		wantOK  bool
	}{
		{"valid positive", "42", 42, true},
		{"valid one", "1", 1, true},
		{"large number", "100000", 100000, true},
		{"zero", "0", 0, false},
		{"negative", "-5", 0, false},
		{"empty", "", 0, false},
		{"whitespace only", "   ", 0, false},
		{"not a number", "abc", 0, false},
		{"float", "3.14", 0, false},
		{"whitespace padded", "  42  ", 42, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envName := "NTM_TEST_POSITIVE_INT"
			if tc.value != "" || tc.name == "empty" {
				t.Setenv(envName, tc.value)
			}
			got, ok := envPositiveInt(envName)
			if ok != tc.wantOK {
				t.Errorf("envPositiveInt(%q) ok = %v, want %v", tc.value, ok, tc.wantOK)
			}
			if got != tc.wantVal {
				t.Errorf("envPositiveInt(%q) = %d, want %d", tc.value, got, tc.wantVal)
			}
		})
	}

	t.Run("unset env", func(t *testing.T) {
		// Don't set env var at all
		got, ok := envPositiveInt("NTM_TEST_UNSET_POSITIVE")
		if ok {
			t.Error("expected false for unset env var")
		}
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})
}

// =============================================================================
// envNonNegativeInt tests
// =============================================================================

func TestEnvNonNegativeInt(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantVal int
		wantOK  bool
	}{
		{"valid positive", "42", 42, true},
		{"zero", "0", 0, true},
		{"negative", "-5", 0, false},
		{"empty", "", 0, false},
		{"not a number", "abc", 0, false},
		{"whitespace padded", "  0  ", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envName := "NTM_TEST_NONNEG_INT"
			if tc.value != "" || tc.name == "empty" {
				t.Setenv(envName, tc.value)
			}
			got, ok := envNonNegativeInt(envName)
			if ok != tc.wantOK {
				t.Errorf("envNonNegativeInt(%q) ok = %v, want %v", tc.value, ok, tc.wantOK)
			}
			if got != tc.wantVal {
				t.Errorf("envNonNegativeInt(%q) = %d, want %d", tc.value, got, tc.wantVal)
			}
		})
	}
}

// =============================================================================
// envNonNegativeFloat tests
// =============================================================================

func TestEnvNonNegativeFloat(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantVal float64
		wantOK  bool
	}{
		{"valid float", "3.14", 3.14, true},
		{"valid integer", "42", 42.0, true},
		{"zero", "0", 0.0, true},
		{"zero float", "0.0", 0.0, true},
		{"negative", "-1.5", 0, false},
		{"empty", "", 0, false},
		{"not a number", "abc", 0, false},
		{"whitespace padded", "  2.5  ", 2.5, true},
		{"very small", "0.001", 0.001, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envName := "NTM_TEST_NONNEG_FLOAT"
			if tc.value != "" || tc.name == "empty" {
				t.Setenv(envName, tc.value)
			}
			got, ok := envNonNegativeFloat(envName)
			if ok != tc.wantOK {
				t.Errorf("envNonNegativeFloat(%q) ok = %v, want %v", tc.value, ok, tc.wantOK)
			}
			if got != tc.wantVal {
				t.Errorf("envNonNegativeFloat(%q) = %f, want %f", tc.value, got, tc.wantVal)
			}
		})
	}
}

// =============================================================================
// LayoutForWidth tests
// =============================================================================

func TestLayoutForWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		width int
		want  LayoutMode
	}{
		{"very narrow", 30, LayoutMobile},
		{"below mobile threshold", 59, LayoutMobile},
		{"at mobile threshold", MobileThreshold, LayoutCompact},
		{"between mobile and tablet", 80, LayoutCompact},
		{"at tablet threshold", TabletThreshold, LayoutSplit},
		{"between tablet and desktop", 120, LayoutSplit},
		{"at desktop threshold", DesktopThreshold, LayoutWide},
		{"between desktop and ultra", 160, LayoutWide},
		{"at ultrawide threshold", UltraWideThreshold, LayoutUltraWide},
		{"beyond ultrawide", 300, LayoutUltraWide},
		{"zero width", 0, LayoutMobile},
		{"negative width", -10, LayoutMobile},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := LayoutForWidth(tc.width)
			if got != tc.want {
				t.Errorf("LayoutForWidth(%d) = %v, want %v", tc.width, got, tc.want)
			}
		})
	}
}

// =============================================================================
// CalculateLayout tests
// =============================================================================

func TestCalculateLayout(t *testing.T) {
	t.Parallel()

	t.Run("mobile layout", func(t *testing.T) {
		t.Parallel()
		dims := CalculateLayout(50, 30)
		if dims.Mode != LayoutMobile {
			t.Errorf("Mode = %v, want LayoutMobile", dims.Mode)
		}
		if dims.CardWidth != 46 { // width - 4
			t.Errorf("CardWidth = %d, want 46", dims.CardWidth)
		}
		if dims.CardsPerRow != 1 {
			t.Errorf("CardsPerRow = %d, want 1", dims.CardsPerRow)
		}
	})

	t.Run("compact layout", func(t *testing.T) {
		t.Parallel()
		dims := CalculateLayout(80, 30)
		if dims.Mode != LayoutCompact {
			t.Errorf("Mode = %v, want LayoutCompact", dims.Mode)
		}
		if dims.CardsPerRow < 1 {
			t.Errorf("CardsPerRow = %d, should be >= 1", dims.CardsPerRow)
		}
	})

	t.Run("split layout proportions", func(t *testing.T) {
		t.Parallel()
		dims := CalculateLayout(120, 30)
		if dims.Mode != LayoutSplit {
			t.Errorf("Mode = %v, want LayoutSplit", dims.Mode)
		}
		if dims.ListWidth == 0 || dims.DetailWidth == 0 {
			t.Error("split layout should have non-zero list and detail widths")
		}
		// 40% list : 60% detail
		if dims.ListWidth >= dims.DetailWidth {
			t.Errorf("list (%d) should be narrower than detail (%d)", dims.ListWidth, dims.DetailWidth)
		}
	})

	t.Run("body height calculation", func(t *testing.T) {
		t.Parallel()
		dims := CalculateLayout(120, 40)
		if dims.BodyHeight != 30 { // height - 10
			t.Errorf("BodyHeight = %d, want 30", dims.BodyHeight)
		}
	})
}

// =============================================================================
// WorkingSpinnerFrame tests
// =============================================================================

func TestWorkingSpinnerFrame(t *testing.T) {
	t.Parallel()

	expected := []string{"◐", "◓", "◑", "◒"}
	for i, want := range expected {
		got := WorkingSpinnerFrame(i)
		if got != want {
			t.Errorf("WorkingSpinnerFrame(%d) = %q, want %q", i, got, want)
		}
	}

	// Verify wrapping
	if got := WorkingSpinnerFrame(4); got != "◐" {
		t.Errorf("WorkingSpinnerFrame(4) = %q, want %q (should wrap)", got, "◐")
	}
	if got := WorkingSpinnerFrame(7); got != "◒" {
		t.Errorf("WorkingSpinnerFrame(7) = %q, want %q", got, "◒")
	}
}

// =============================================================================
// ViewportPosition.EnsureVisible edge cases
// =============================================================================

func TestEnsureVisibleEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("visible larger than total", func(t *testing.T) {
		t.Parallel()
		vp := &ViewportPosition{
			Offset:   5,
			Visible:  20,
			Total:    10,
			Selected: 3,
		}
		vp.EnsureVisible()
		if vp.Offset != 0 {
			t.Errorf("Offset = %d, want 0 when visible > total", vp.Offset)
		}
	})

	t.Run("selected at boundary", func(t *testing.T) {
		t.Parallel()
		vp := &ViewportPosition{
			Offset:   0,
			Visible:  5,
			Total:    10,
			Selected: 4, // last visible item
		}
		vp.EnsureVisible()
		if vp.Offset != 0 {
			t.Errorf("Offset = %d, want 0 for item at last visible position", vp.Offset)
		}
	})

	t.Run("selected just past visible", func(t *testing.T) {
		t.Parallel()
		vp := &ViewportPosition{
			Offset:   0,
			Visible:  5,
			Total:    10,
			Selected: 5, // one past visible
		}
		vp.EnsureVisible()
		if vp.Offset != 1 {
			t.Errorf("Offset = %d, want 1", vp.Offset)
		}
	})
}

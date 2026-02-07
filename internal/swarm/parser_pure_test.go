package swarm

import (
	"testing"
)

// =============================================================================
// agent_launcher.go: parsePrimaryWindowIndex
// =============================================================================

func TestParsePrimaryWindowIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"empty string", "", 0, true},
		{"whitespace only", "   \n  \n  ", 0, true},
		{"single value", "5", 5, false},
		{"single value 1", "1", 1, false},
		{"multiple values min", "3\n5\n2\n7", 2, false},
		{"window 1 preferred", "3\n1\n5", 1, false},
		{"window 1 not min", "0\n1\n5", 1, false},
		{"no valid integers", "abc\ndef", 0, true},
		{"mixed valid invalid", "abc\n3\ndef\n7", 3, false},
		{"blank lines ignored", "\n\n4\n\n6\n\n", 4, false},
		{"leading trailing spaces", "  3  \n  5  ", 3, false},
		{"zero is valid", "0\n2\n3", 0, false},
		{"negative values", "-1\n2\n3", -1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePrimaryWindowIndex(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parsePrimaryWindowIndex(%q) expected error, got %d", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePrimaryWindowIndex(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parsePrimaryWindowIndex(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// agent_launcher.go: parseMinIntOutput
// =============================================================================

func TestParseMinIntOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"empty string", "", 0, true},
		{"whitespace only", "  \n  ", 0, true},
		{"single value", "42", 42, false},
		{"multiple values", "10\n5\n20\n3\n15", 3, false},
		{"no valid integers", "abc\nxyz", 0, true},
		{"mixed with invalid lines", "abc\n7\ndef\n3\nghi", 3, false},
		{"blank lines ignored", "\n\n8\n\n", 8, false},
		{"all same value", "5\n5\n5", 5, false},
		{"zero is min", "3\n0\n1", 0, false},
		{"negative min", "-2\n1\n5", -2, false},
		{"spaces in lines", "  4  \n  2  \n  9  ", 2, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseMinIntOutput(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseMinIntOutput(%q) expected error, got %d", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMinIntOutput(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseMinIntOutput(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// agent_launcher.go: swarmTmuxPaneIndex
// =============================================================================

func TestSwarmTmuxPaneIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		base      int
		planIndex int
		want      int
		wantErr   bool
	}{
		{"plan 0 returns base", 5, 0, 5, false},
		{"plan 1 returns base", 5, 1, 5, false},
		{"plan 2 returns base+1", 5, 2, 6, false},
		{"plan 3 returns base+2", 5, 3, 7, false},
		{"negative plan index", 5, -1, 0, true},
		{"base 0 plan 0", 0, 0, 0, false},
		{"base 0 plan 1", 0, 1, 0, false},
		{"base 0 plan 2", 0, 2, 1, false},
		{"base 1 plan 0", 1, 0, 1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := swarmTmuxPaneIndex(tc.base, tc.planIndex)
			if tc.wantErr {
				if err == nil {
					t.Errorf("swarmTmuxPaneIndex(%d, %d) expected error, got %d", tc.base, tc.planIndex, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("swarmTmuxPaneIndex(%d, %d) unexpected error: %v", tc.base, tc.planIndex, err)
			}
			if got != tc.want {
				t.Errorf("swarmTmuxPaneIndex(%d, %d) = %d, want %d", tc.base, tc.planIndex, got, tc.want)
			}
		})
	}
}

// =============================================================================
// agent_launcher.go: formatPaneTargetWithWindow
// =============================================================================

func TestFormatPaneTargetWithWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		session string
		window  int
		pane    int
		want    string
	}{
		{"basic", "myproj", 0, 1, "myproj:0.1"},
		{"window 1", "sess", 1, 0, "sess:1.0"},
		{"large indices", "test", 10, 25, "test:10.25"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatPaneTargetWithWindow(tc.session, tc.window, tc.pane)
			if got != tc.want {
				t.Errorf("formatPaneTargetWithWindow(%q, %d, %d) = %q, want %q",
					tc.session, tc.window, tc.pane, got, tc.want)
			}
		})
	}
}

// =============================================================================
// agent_launcher.go: swarmPaneTargetFromPlanIndex
// =============================================================================

func TestSwarmPaneTargetFromPlanIndexEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("valid plan index", func(t *testing.T) {
		t.Parallel()
		targeting := swarmSessionTargeting{WindowIndex: 1, BasePaneIndex: 3}
		got, err := swarmPaneTargetFromPlanIndex("mysess", targeting, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// planIndex=2 -> tmuxPaneIndex = 3 + (2-1) = 4
		want := "mysess:1.4"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("plan index 0 returns base", func(t *testing.T) {
		t.Parallel()
		targeting := swarmSessionTargeting{WindowIndex: 0, BasePaneIndex: 5}
		got, err := swarmPaneTargetFromPlanIndex("sess", targeting, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "sess:0.5"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("negative plan index errors", func(t *testing.T) {
		t.Parallel()
		targeting := swarmSessionTargeting{WindowIndex: 0, BasePaneIndex: 0}
		_, err := swarmPaneTargetFromPlanIndex("sess", targeting, -1)
		if err == nil {
			t.Error("expected error for negative plan index")
		}
	})
}

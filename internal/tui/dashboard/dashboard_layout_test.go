package dashboard

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dicklesworthstone/ntm/internal/cass"
	"github.com/Dicklesworthstone/ntm/internal/status"
	"github.com/Dicklesworthstone/ntm/internal/tmux"
	"github.com/Dicklesworthstone/ntm/internal/tracker"
	"github.com/Dicklesworthstone/ntm/internal/tui/layout"
)

func newTestModel(width int) Model {
	m := New("test", "")
	m.width = width
	m.height = 30
	m.tier = layout.TierForWidth(width)
	m.panes = []tmux.Pane{
		{
			ID:      "1",
			Index:   1,
			Title:   "codex-long-title-for-wrap-check",
			Type:    tmux.AgentCodex,
			Variant: "VARIANT",
			Command: "run --flag",
		},
	}
	m.cursor = 0
	m.paneStatus[1] = PaneStatus{
		State:          "working",
		ContextPercent: 50,
		ContextLimit:   1000,
	}
	return m
}

func TestPaneListColumnsByWidthTiers(t *testing.T) {
	t.Parallel()

	// Test that renderPaneList produces output for various widths without panicking.
	// The layout dimensions affect column visibility (ShowContextCol, ShowModelCol, etc.)
	// but we don't strictly verify header content since it depends on theme/style rendering.
	cases := []struct {
		width int
		name  string
	}{
		{width: 80, name: "narrow"},
		{width: 120, name: "tablet-threshold"},
		{width: 160, name: "desktop-threshold"},
		{width: 200, name: "wide"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := newTestModel(tc.width)
			// Use the same width for layout calculations
			list := m.renderPaneList(tc.width)

			// Basic sanity checks
			if list == "" {
				t.Fatalf("width %d: renderPaneList returned empty string", tc.width)
			}

			lines := strings.Split(list, "\n")
			if len(lines) < 2 {
				t.Fatalf("width %d: expected at least 2 lines (header + row), got %d", tc.width, len(lines))
			}

			// Verify CalculateLayout produces expected column visibility flags
			dims := CalculateLayout(tc.width, 1)
			if tc.width >= TabletThreshold && !dims.ShowContextCol {
				t.Errorf("width %d: ShowContextCol should be true for width >= %d", tc.width, TabletThreshold)
			}
			if tc.width >= DesktopThreshold && !dims.ShowModelCol {
				t.Errorf("width %d: ShowModelCol should be true for width >= %d", tc.width, DesktopThreshold)
			}
			if tc.width >= UltraWideThreshold && !dims.ShowCmdCol {
				t.Errorf("width %d: ShowCmdCol should be true for width >= %d", tc.width, UltraWideThreshold)
			}
		})
	}
}

func TestPaneRowSelectionStyling_NoWrapAcrossWidths(t *testing.T) {
	t.Parallel()

	widths := []int{80, 120, 160, 200}
	for _, w := range widths {
		w := w
		t.Run(fmt.Sprintf("width_%d", w), func(t *testing.T) {
			t.Parallel()

			m := newTestModel(w)
			m.cursor = 0 // selected row
			// Use same width for layout calculation
			dims := CalculateLayout(w, 1)
			row := PaneTableRow{
				Index:        m.panes[0].Index,
				Type:         string(m.panes[0].Type),
				Title:        m.panes[0].Title,
				Status:       m.paneStatus[m.panes[0].Index].State,
				IsSelected:   true,
				ContextPct:   m.paneStatus[m.panes[0].Index].ContextPercent,
				ModelVariant: m.panes[0].Variant,
			}
			rendered := RenderPaneRow(row, dims, m.theme)
			clean := status.StripANSI(rendered)

			// Row should be rendered and not empty
			if len(clean) == 0 {
				t.Fatalf("width %d: rendered row is empty", w)
			}

			// Row should not contain unexpected newlines (single line output for basic mode)
			// Note: Wide layouts may include second line for rich content, so only check
			// if layout mode is not wide enough for multi-line output
			if dims.Mode < LayoutWide && strings.Contains(clean, "\n") {
				t.Fatalf("width %d: row contained unexpected newline in non-wide mode", w)
			}
		})
	}
}

func TestSplitViewLayouts_ByWidthTiers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		width        int
		expectList   bool
		expectDetail bool
	}{
		{width: 120, expectList: true, expectDetail: true},
		{width: 160, expectList: true, expectDetail: true},
		{width: 200, expectList: true, expectDetail: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("width_%d", tc.width), func(t *testing.T) {
			t.Parallel()

			m := newTestModel(tc.width)
			m.height = 30
			if m.tier < layout.TierSplit {
				t.Skip("split view not used below split threshold")
			}
			out := m.renderSplitView()
			plain := status.StripANSI(out)

			// Ensure we always render the list panel
			if !strings.Contains(plain, "TITLE") {
				t.Fatalf("width %d: expected list header 'TITLE' in split view", tc.width)
			}

			if tc.expectDetail {
				if !strings.Contains(plain, "Context Usage") && m.tier >= layout.TierWide {
					t.Fatalf("width %d: expected detail pane content (Context Usage) at wide tier", tc.width)
				}
			} else {
				// For narrow widths we shouldn't render split view; ensure single-panel fallback
				if strings.Contains(plain, "Context Usage") && tc.width < layout.SplitViewThreshold {
					t.Fatalf("width %d: unexpected detail content for narrow layout", tc.width)
				}
			}
		})
	}
}

func TestSplitProportionsAcrossThresholds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		total         int
		expectSplit   bool
		expectNonZero bool
		name          string
	}{
		{total: 80, expectSplit: false, expectNonZero: false, name: "narrow"},
		{total: 120, expectSplit: true, expectNonZero: true, name: "split-threshold"},
		{total: 160, expectSplit: true, expectNonZero: true, name: "mid-split"},
		{total: 200, expectSplit: true, expectNonZero: true, name: "wide"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			left, right := layout.SplitProportions(tc.total)

			if left+right > tc.total {
				t.Fatalf("total %d: left+right=%d exceeds total width", tc.total, left+right)
			}

			if tc.expectSplit {
				if right == 0 {
					t.Fatalf("total %d: expected split view to allocate right panel", tc.total)
				}
			} else if right != 0 {
				t.Fatalf("total %d: expected single column layout, got right=%d", tc.total, right)
			}

			if tc.expectNonZero && (left == 0 || right == 0) {
				t.Fatalf("total %d: both panels should be non-zero (left=%d right=%d)", tc.total, left, right)
			}
		})
	}
}

func TestSidebarRendersCASSContext(t *testing.T) {
	t.Parallel()

	m := newTestModel(layout.UltraWideViewThreshold)
	now := time.Now().Add(-2 * time.Hour).Unix()
	hits := []cass.SearchHit{
		{
			Title:     "Session: auth refactor",
			Score:     0.90,
			CreatedAt: &now,
		},
	}

	updated, _ := m.Update(CASSContextMsg{Hits: hits})
	m = updated.(Model)

	out := status.StripANSI(m.renderSidebar(60, 25))
	if !strings.Contains(out, "auth refactor") {
		t.Fatalf("expected sidebar to include CASS hit title; got:\n%s", out)
	}
}

func TestSidebarRendersFileChanges(t *testing.T) {
	t.Parallel()

	m := newTestModel(layout.UltraWideViewThreshold)
	now := time.Now().Add(-2 * time.Minute)

	changes := []tracker.RecordedFileChange{
		{
			Timestamp: now,
			Session:   "test",
			Agents:    []string{"BluePond"},
			Change: tracker.FileChange{
				Path: "/src/main.go",
				Type: tracker.FileModified,
			},
		},
	}

	updated, _ := m.Update(FileChangeMsg{Changes: changes})
	m = updated.(Model)

	out := status.StripANSI(m.renderSidebar(60, 25))
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expected sidebar to include file change; got:\n%s", out)
	}
}

func TestHelpOverlayToggle(t *testing.T) {
	t.Parallel()

	t.Run("pressing_?_opens_help", func(t *testing.T) {
		t.Parallel()

		m := newTestModel(120)
		if m.showHelp {
			t.Fatal("showHelp should be false initially")
		}

		// Press '?' to open help
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
		updated, _ := m.Update(msg)
		m = updated.(Model)

		if !m.showHelp {
			t.Error("showHelp should be true after pressing '?'")
		}
	})

	t.Run("pressing_?_again_closes_help", func(t *testing.T) {
		t.Parallel()

		m := newTestModel(120)
		m.showHelp = true

		// Press '?' to close help
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
		updated, _ := m.Update(msg)
		m = updated.(Model)

		if m.showHelp {
			t.Error("showHelp should be false after pressing '?' while open")
		}
	})

	t.Run("pressing_esc_closes_help", func(t *testing.T) {
		t.Parallel()

		m := newTestModel(120)
		m.showHelp = true

		// Press Esc to close help
		msg := tea.KeyMsg{Type: tea.KeyEsc}
		updated, _ := m.Update(msg)
		m = updated.(Model)

		if m.showHelp {
			t.Error("showHelp should be false after pressing Esc while open")
		}
	})

	t.Run("help_overlay_blocks_other_keys", func(t *testing.T) {
		t.Parallel()

		m := newTestModel(120)
		m.showHelp = true
		initialCursor := m.cursor

		// Try to move cursor down while help is open
		msg := tea.KeyMsg{Type: tea.KeyDown}
		updated, _ := m.Update(msg)
		m = updated.(Model)

		if m.cursor != initialCursor {
			t.Error("cursor should not change when help overlay is open")
		}
		if !m.showHelp {
			t.Error("help should still be open after pressing unrelated key")
		}
	})
}

func TestHelpBarIncludesHelpHint(t *testing.T) {
	t.Parallel()

	m := newTestModel(120)
	helpBar := m.renderHelpBar()

	if !strings.Contains(helpBar, "?") {
		t.Error("help bar should include '?' hint")
	}
	if !strings.Contains(helpBar, "help") {
		t.Error("help bar should include 'help' description")
	}
}

func TestViewRendersHelpOverlayWhenOpen(t *testing.T) {
	t.Parallel()

	m := newTestModel(120)
	m.showHelp = true

	view := m.View()

	if !strings.Contains(view, "Shortcuts") || !strings.Contains(view, "Navigation") {
		t.Error("view should render help overlay content when showHelp is true")
	}
}

func TestQuickActionsBarWidthGated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		width       int
		shouldShow  bool
		description string
	}{
		{width: 80, shouldShow: false, description: "narrow"},
		{width: 120, shouldShow: false, description: "split"},
		{width: 180, shouldShow: false, description: "below wide"},
		{width: 200, shouldShow: true, description: "wide threshold"},
		{width: 240, shouldShow: true, description: "ultra"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			m := newTestModel(tc.width)
			quickActions := m.renderQuickActions()
			plain := status.StripANSI(quickActions)

			hasContent := len(plain) > 0

			if tc.shouldShow && !hasContent {
				t.Errorf("width %d: expected quick actions to be visible at wide tier", tc.width)
			}
			if !tc.shouldShow && hasContent {
				t.Errorf("width %d: expected quick actions to be hidden in narrow mode", tc.width)
			}
		})
	}
}

func TestQuickActionsBarContainsExpectedActions(t *testing.T) {
	t.Parallel()

	m := newTestModel(200) // Wide enough to show quick actions
	quickActions := m.renderQuickActions()
	plain := status.StripANSI(quickActions)

	expectedItems := []string{"Palette", "Send", "Copy", "Zoom"}
	for _, item := range expectedItems {
		if !strings.Contains(plain, item) {
			t.Errorf("quick actions bar should contain '%s', got: %s", item, plain)
		}
	}

	// Verify the "Actions" label is present
	if !strings.Contains(plain, "Actions") {
		t.Error("quick actions bar should contain 'Actions' label")
	}
}

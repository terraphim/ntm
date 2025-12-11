package panels

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/Dicklesworthstone/ntm/internal/history"
	"github.com/Dicklesworthstone/ntm/internal/tui/theme"
)

// historyConfig returns the configuration for the history panel
func historyConfig() PanelConfig {
	return PanelConfig{
		ID:              "history",
		Title:           "Command History",
		Priority:        PriorityNormal,
		RefreshInterval: 30 * time.Second, // Slow refresh, history doesn't change often
		MinWidth:        35,
		MinHeight:       8,
		Collapsible:     true,
	}
}

// HistoryPanel displays command history
type HistoryPanel struct {
	PanelBase
	entries []history.HistoryEntry
	cursor  int
	offset  int
	theme   theme.Theme
}

// NewHistoryPanel creates a new history panel
func NewHistoryPanel() *HistoryPanel {
	return &HistoryPanel{
		PanelBase: NewPanelBase(historyConfig()),
		theme:     theme.Current(),
	}
}

// Init implements tea.Model
func (m *HistoryPanel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *HistoryPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.IsFocused() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.contentHeight() {
					m.offset = m.cursor - m.contentHeight() + 1
				}
			}
		}
	}
	return m, nil
}

// SetEntries updates the history entries
func (m *HistoryPanel) SetEntries(entries []history.HistoryEntry) {
	m.entries = entries
	// Keep cursor within bounds
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Keybindings returns history panel specific shortcuts
func (m *HistoryPanel) Keybindings() []Keybinding {
	return []Keybinding{
		{
			Key:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "replay")),
			Description: "Replay selected command",
			Action:      "replay",
		},
		{
			Key:         key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
			Description: "Copy command to clipboard",
			Action:      "copy",
		},
		{
			Key:         key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
			Description: "Move cursor down",
			Action:      "down",
		},
		{
			Key:         key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "up")),
			Description: "Move cursor up",
			Action:      "up",
		},
	}
}

func (m *HistoryPanel) contentHeight() int {
	return m.Height() - 4 // borders + header
}

// View renders the panel
func (m *HistoryPanel) View() string {
	t := m.theme
	w, h := m.Width(), m.Height()

	borderColor := t.Surface1
	if m.IsFocused() {
		borderColor = t.Primary
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w-2).
		Height(h-2).
		Padding(0, 1)

	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Lavender).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(t.Surface1).
		Width(w - 4).
		Align(lipgloss.Center)

	content.WriteString(headerStyle.Render(m.Config().Title) + "\n")

	if len(m.entries) == 0 {
		content.WriteString("\n" + lipgloss.NewStyle().Foreground(t.Overlay).Italic(true).Render("No history"))
		return boxStyle.Render(content.String())
	}

	visibleHeight := m.contentHeight()
	end := m.offset + visibleHeight
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		entry := m.entries[i]
		selected := i == m.cursor

		var lineStyle lipgloss.Style
		if selected {
			lineStyle = lipgloss.NewStyle().Background(t.Surface0).Bold(true)
		} else {
			lineStyle = lipgloss.NewStyle()
		}

		// ID
		id := lipgloss.NewStyle().Foreground(t.Overlay).Render(entry.ID[:4])

		// Targets
		targets := "all"
		if len(entry.Targets) > 0 {
			targets = strings.Join(entry.Targets, ",")
		}
		if len(targets) > 10 {
			targets = targets[:9] + "…"
		}
		targetStyle := lipgloss.NewStyle().Foreground(t.Blue).Width(10).Render(targets)

		// Prompt
		prompt := strings.ReplaceAll(entry.Prompt, "\n", " ")
		maxPrompt := w - 20
		if maxPrompt < 10 {
			maxPrompt = 10
		}
		if len(prompt) > maxPrompt {
			prompt = prompt[:maxPrompt-1] + "…"
		}
		promptStyle := lipgloss.NewStyle().Foreground(t.Text).Render(prompt)

		// Status
		status := "✓"
		statusColor := t.Green
		if !entry.Success {
			status = "✗"
			statusColor = t.Red
		}
		statusStyle := lipgloss.NewStyle().Foreground(statusColor).Render(status)

		line := fmt.Sprintf("%s %s %s %s", statusStyle, id, targetStyle, promptStyle)
		content.WriteString(lineStyle.Render(line) + "\n")
	}

	return boxStyle.Render(content.String())
}

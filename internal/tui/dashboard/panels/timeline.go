package panels

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Dicklesworthstone/ntm/internal/state"
	"github.com/Dicklesworthstone/ntm/internal/tui/components"
	"github.com/Dicklesworthstone/ntm/internal/tui/theme"
)

// timelineConfig returns the configuration for the timeline panel
func timelineConfig() PanelConfig {
	return PanelConfig{
		ID:              "timeline",
		Title:           "Agent Timeline",
		Priority:        PriorityNormal,
		RefreshInterval: 5 * time.Second,
		MinWidth:        40,
		MinHeight:       10,
		Collapsible:     true,
	}
}

// TimelineData holds the data for the timeline panel
type TimelineData struct {
	Events []state.AgentEvent
	Stats  state.TimelineStats
}

// TimelinePanel displays agent state changes as horizontal timeline bars
type TimelinePanel struct {
	PanelBase
	data   TimelineData
	theme  theme.Theme
	err    error
	cursor int // Currently selected agent index

	// View parameters
	timeWindow   time.Duration // Total time span visible
	timeOffset   time.Duration // Offset from now (negative = past)
	zoomLevel    int           // Zoom level (0 = default, positive = zoomed in)
	selectedTime time.Time     // Time position for details
}

// NewTimelinePanel creates a new timeline panel
func NewTimelinePanel() *TimelinePanel {
	return &TimelinePanel{
		PanelBase:  NewPanelBase(timelineConfig()),
		theme:      theme.Current(),
		timeWindow: 30 * time.Minute, // Default 30 minute window
		timeOffset: 0,
		zoomLevel:  0,
	}
}

// HasError returns true if there's an active error
func (m *TimelinePanel) HasError() bool {
	return m.err != nil
}

// Init implements tea.Model
func (m *TimelinePanel) Init() tea.Cmd {
	return nil
}

// TimelineSelectMsg is sent when user selects a timeline position
type TimelineSelectMsg struct {
	AgentID string
	Time    time.Time
	State   state.TimelineState
}

// Update implements tea.Model
func (m *TimelinePanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.IsFocused() {
		return m, nil
	}

	agents := m.getAgentList()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(agents)-1 {
				m.cursor++
			}
		case "left", "h":
			// Scroll back in time
			m.timeOffset -= m.scrollStep()
		case "right", "l":
			// Scroll forward in time (but not past now)
			if m.timeOffset < 0 {
				m.timeOffset += m.scrollStep()
				if m.timeOffset > 0 {
					m.timeOffset = 0
				}
			}
		case "+", "=":
			// Zoom in (smaller time window)
			if m.zoomLevel < 5 {
				m.zoomLevel++
				m.timeWindow = m.windowForZoom()
			}
		case "-", "_":
			// Zoom out (larger time window)
			if m.zoomLevel > -3 {
				m.zoomLevel--
				m.timeWindow = m.windowForZoom()
			}
		case "n":
			// Jump to now
			m.timeOffset = 0
		case "enter":
			// Select current position for details
			if len(agents) > 0 && m.cursor >= 0 && m.cursor < len(agents) {
				agentID := agents[m.cursor]
				now := time.Now()
				selectedTime := now.Add(m.timeOffset)
				eventState := m.getStateAtTime(agentID, selectedTime)
				return m, func() tea.Msg {
					return TimelineSelectMsg{
						AgentID: agentID,
						Time:    selectedTime,
						State:   eventState,
					}
				}
			}
		}
	}
	return m, nil
}

// SetData updates the timeline data
func (m *TimelinePanel) SetData(data TimelineData, err error) {
	m.data = data
	m.err = err
	if err == nil {
		m.SetLastUpdate(time.Now())
	}
	// Keep cursor within bounds
	agents := m.getAgentList()
	if m.cursor >= len(agents) {
		m.cursor = len(agents) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Keybindings returns timeline panel specific shortcuts
func (m *TimelinePanel) Keybindings() []Keybinding {
	return []Keybinding{
		{
			Key:         key.NewBinding(key.WithKeys("+"), key.WithHelp("+", "zoom in")),
			Description: "Zoom in timeline",
			Action:      "zoom_in",
		},
		{
			Key:         key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "zoom out")),
			Description: "Zoom out timeline",
			Action:      "zoom_out",
		},
		{
			Key:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "scroll back")),
			Description: "Scroll back in time",
			Action:      "scroll_back",
		},
		{
			Key:         key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "scroll forward")),
			Description: "Scroll forward",
			Action:      "scroll_forward",
		},
		{
			Key:         key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "now")),
			Description: "Jump to now",
			Action:      "jump_now",
		},
		{
			Key:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
			Description: "View event details",
			Action:      "details",
		},
	}
}

// View renders the panel
func (m *TimelinePanel) View() string {
	t := m.theme
	w, h := m.Width(), m.Height()

	borderColor := t.Surface1
	bgColor := t.Base
	if m.IsFocused() {
		borderColor = t.Primary
		bgColor = t.Surface0
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(bgColor).
		Width(w - 2).
		Height(h - 2).
		Padding(0, 1)

	var content strings.Builder

	// Build header with error badge if needed
	title := m.Config().Title
	if m.err != nil {
		errorBadge := lipgloss.NewStyle().
			Background(t.Red).
			Foreground(t.Base).
			Bold(true).
			Padding(0, 1).
			Render("⚠ Error")
		title = title + " " + errorBadge
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Lavender).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(t.Surface1).
		Width(w - 4).
		Align(lipgloss.Center)

	content.WriteString(headerStyle.Render(title) + "\n")

	// Show error message if present
	if m.err != nil {
		content.WriteString(components.ErrorState(m.err.Error(), "Press r to retry", w-4) + "\n")
	}

	agents := m.getAgentList()
	if len(agents) == 0 {
		content.WriteString("\n" + components.RenderEmptyState(components.EmptyStateOptions{
			Icon:        components.IconWaiting,
			Title:       "No agent activity",
			Description: "Timeline will populate as agents change state",
			Action:      "Spawn agents with 'ntm spawn'",
			Width:       w - 4,
			Centered:    true,
		}))
		return boxStyle.Render(FitToHeight(content.String(), h-4))
	}

	// Calculate layout
	labelWidth := m.maxAgentLabelWidth(agents) + 2
	barWidth := w - labelWidth - 8
	if barWidth < 10 {
		barWidth = 10
	}

	// Render agent tracks
	contentHeight := h - 6 // Leave room for header, time axis, borders
	visibleAgents := contentHeight - 2
	if visibleAgents < 1 {
		visibleAgents = 1
	}

	now := time.Now()
	windowEnd := now.Add(m.timeOffset)
	windowStart := windowEnd.Add(-m.timeWindow)

	for i, agentID := range agents {
		if i >= visibleAgents {
			break
		}

		selected := i == m.cursor
		var lineStyle lipgloss.Style
		if selected && m.IsFocused() {
			lineStyle = lipgloss.NewStyle().Background(t.Surface0).Bold(true)
		} else {
			lineStyle = lipgloss.NewStyle()
		}

		// Agent label
		label := m.formatAgentLabel(agentID, labelWidth)
		labelStyle := lipgloss.NewStyle().
			Foreground(m.agentColor(agentID)).
			Width(labelWidth)

		// Render timeline bar
		bar := m.renderTimelineBar(agentID, windowStart, windowEnd, barWidth)

		line := labelStyle.Render(label) + " " + bar
		content.WriteString(lineStyle.Render(line) + "\n")
	}

	// Render time axis
	timeAxis := m.renderTimeAxis(windowStart, windowEnd, labelWidth, barWidth)
	content.WriteString(timeAxis + "\n")

	// Render zoom/offset indicator
	indicator := m.renderIndicator(windowStart, windowEnd, w-4)
	content.WriteString(indicator)

	return boxStyle.Render(FitToHeight(content.String(), h-4))
}

// Helper methods

func (m *TimelinePanel) getAgentList() []string {
	agentSet := make(map[string]struct{})
	for _, e := range m.data.Events {
		agentSet[e.AgentID] = struct{}{}
	}
	agents := make([]string, 0, len(agentSet))
	for id := range agentSet {
		agents = append(agents, id)
	}
	sort.Strings(agents)
	return agents
}

func (m *TimelinePanel) maxAgentLabelWidth(agents []string) int {
	maxWidth := 8 // minimum
	for _, id := range agents {
		if len(id) > maxWidth {
			maxWidth = len(id)
		}
	}
	if maxWidth > 15 {
		maxWidth = 15
	}
	return maxWidth
}

func (m *TimelinePanel) formatAgentLabel(agentID string, width int) string {
	if len(agentID) > width {
		return agentID[:width-1] + "…"
	}
	return fmt.Sprintf("%-*s", width, agentID)
}

func (m *TimelinePanel) agentColor(agentID string) lipgloss.Color {
	t := m.theme
	// Color based on agent type prefix
	if strings.HasPrefix(agentID, "cc") {
		return t.Claude
	}
	if strings.HasPrefix(agentID, "cod") {
		return t.Codex
	}
	if strings.HasPrefix(agentID, "gmi") {
		return t.Gemini
	}
	return t.Text
}

func (m *TimelinePanel) stateColor(s state.TimelineState) lipgloss.Color {
	t := m.theme
	switch s {
	case state.TimelineIdle:
		return t.Overlay // Gray
	case state.TimelineWorking:
		return t.Green
	case state.TimelineWaiting:
		return t.Yellow
	case state.TimelineError:
		return t.Red
	case state.TimelineStopped:
		return t.Surface2 // Dark gray
	default:
		return t.Overlay
	}
}

func (m *TimelinePanel) stateChar(s state.TimelineState) string {
	switch s {
	case state.TimelineWorking:
		return "█"
	case state.TimelineWaiting:
		return "▓"
	case state.TimelineError:
		return "▒"
	case state.TimelineIdle:
		return "░"
	case state.TimelineStopped:
		return "·"
	default:
		return " "
	}
}

func (m *TimelinePanel) renderTimelineBar(agentID string, start, end time.Time, width int) string {
	t := m.theme
	var bar strings.Builder

	// Opening bracket
	bar.WriteString(lipgloss.NewStyle().Foreground(t.Surface1).Render("["))

	// Get events for this agent
	events := m.getEventsForAgent(agentID)
	if len(events) == 0 {
		// No events, render empty bar
		emptyStyle := lipgloss.NewStyle().Foreground(t.Surface1)
		for i := 0; i < width-2; i++ {
			bar.WriteString(emptyStyle.Render("·"))
		}
		bar.WriteString(lipgloss.NewStyle().Foreground(t.Surface1).Render("]"))
		return bar.String()
	}

	// Calculate time per character
	duration := end.Sub(start)
	charDuration := duration / time.Duration(width-2)

	// Render each time slot
	for i := 0; i < width-2; i++ {
		slotStart := start.Add(time.Duration(i) * charDuration)
		slotEnd := slotStart.Add(charDuration)
		slotState := m.getStateInRange(events, slotStart, slotEnd)
		char := m.stateChar(slotState)
		color := m.stateColor(slotState)
		bar.WriteString(lipgloss.NewStyle().Foreground(color).Render(char))
	}

	// Closing bracket
	bar.WriteString(lipgloss.NewStyle().Foreground(t.Surface1).Render("]"))
	return bar.String()
}

func (m *TimelinePanel) getEventsForAgent(agentID string) []state.AgentEvent {
	var events []state.AgentEvent
	for _, e := range m.data.Events {
		if e.AgentID == agentID {
			events = append(events, e)
		}
	}
	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
	return events
}

func (m *TimelinePanel) getStateInRange(events []state.AgentEvent, start, end time.Time) state.TimelineState {
	// Find the state that was active during this time range
	// We look for the most recent event before or during this range
	var activeState state.TimelineState = state.TimelineIdle
	for _, e := range events {
		if e.Timestamp.Before(end) || e.Timestamp.Equal(end) {
			activeState = e.State
		} else {
			break
		}
	}
	return activeState
}

func (m *TimelinePanel) getStateAtTime(agentID string, t time.Time) state.TimelineState {
	events := m.getEventsForAgent(agentID)
	var activeState state.TimelineState = state.TimelineIdle
	for _, e := range events {
		if e.Timestamp.Before(t) || e.Timestamp.Equal(t) {
			activeState = e.State
		} else {
			break
		}
	}
	return activeState
}

func (m *TimelinePanel) renderTimeAxis(start, end time.Time, labelWidth, barWidth int) string {
	t := m.theme
	var axis strings.Builder

	// Padding to align with bars
	axis.WriteString(strings.Repeat(" ", labelWidth+1))

	// Determine tick count and interval
	duration := end.Sub(start)
	tickCount := 6
	if barWidth < 30 {
		tickCount = 3
	}
	tickInterval := barWidth / tickCount
	timeInterval := duration / time.Duration(tickCount)

	// Render tick marks
	axis.WriteString("|")
	for i := 0; i < tickCount; i++ {
		for j := 0; j < tickInterval-1; j++ {
			axis.WriteString("-")
		}
		if i < tickCount-1 {
			axis.WriteString("|")
		}
	}
	// Fill remaining space
	remaining := barWidth - (tickCount*tickInterval + 1)
	for i := 0; i < remaining; i++ {
		axis.WriteString("-")
	}
	axis.WriteString("|\n")

	// Render time labels
	axis.WriteString(strings.Repeat(" ", labelWidth+1))
	axisStyle := lipgloss.NewStyle().Foreground(t.Overlay)
	for i := 0; i <= tickCount; i++ {
		tickTime := start.Add(time.Duration(i) * timeInterval)
		label := tickTime.Format("15:04")
		if i == 0 {
			axis.WriteString(axisStyle.Render(label))
		} else {
			// Add spacing
			spacing := tickInterval - len(label)
			if spacing < 0 {
				spacing = 0
			}
			axis.WriteString(strings.Repeat(" ", spacing) + axisStyle.Render(label))
		}
	}

	return axis.String()
}

func (m *TimelinePanel) renderIndicator(start, end time.Time, width int) string {
	t := m.theme
	indicatorStyle := lipgloss.NewStyle().Foreground(t.Overlay).Italic(true)

	// Show time range and zoom level
	zoomLabel := ""
	switch m.zoomLevel {
	case -3:
		zoomLabel = "2h"
	case -2:
		zoomLabel = "1h"
	case -1:
		zoomLabel = "45m"
	case 0:
		zoomLabel = "30m"
	case 1:
		zoomLabel = "15m"
	case 2:
		zoomLabel = "10m"
	case 3:
		zoomLabel = "5m"
	case 4:
		zoomLabel = "2m"
	case 5:
		zoomLabel = "1m"
	}

	indicator := fmt.Sprintf("Window: %s | %s - %s",
		zoomLabel,
		start.Format("15:04:05"),
		end.Format("15:04:05"))

	// Add "LIVE" indicator if viewing current time
	if m.timeOffset == 0 {
		liveStyle := lipgloss.NewStyle().Foreground(t.Green).Bold(true)
		indicator = liveStyle.Render("● LIVE") + " " + indicatorStyle.Render(indicator)
	}

	return indicatorStyle.Render(indicator)
}

func (m *TimelinePanel) windowForZoom() time.Duration {
	switch m.zoomLevel {
	case -3:
		return 2 * time.Hour
	case -2:
		return 1 * time.Hour
	case -1:
		return 45 * time.Minute
	case 0:
		return 30 * time.Minute
	case 1:
		return 15 * time.Minute
	case 2:
		return 10 * time.Minute
	case 3:
		return 5 * time.Minute
	case 4:
		return 2 * time.Minute
	case 5:
		return 1 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func (m *TimelinePanel) scrollStep() time.Duration {
	// Scroll by 1/6 of the current window
	return m.timeWindow / 6
}

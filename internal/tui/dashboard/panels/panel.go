package panels

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// PanelPriority defines panel display/update priority levels.
type PanelPriority int

const (
	// PriorityCritical panels always visible and update frequently (alerts, errors)
	PriorityCritical PanelPriority = 1
	// PriorityHigh panels are important for workflow (agents, status)
	PriorityHigh PanelPriority = 2
	// PriorityNormal panels provide useful context (metrics, history)
	PriorityNormal PanelPriority = 3
	// PriorityLow panels are supplementary (tips, help)
	PriorityLow PanelPriority = 4
)

// Keybinding represents a panel-specific keyboard shortcut.
type Keybinding struct {
	Key         key.Binding // The key binding
	Description string      // Human-readable description
	Action      string      // Action identifier for dispatch
}

// PanelConfig holds configuration for panel behavior and display.
type PanelConfig struct {
	// ID is a unique identifier for the panel (e.g., "metrics", "alerts")
	ID string

	// Title is the display title for the panel header
	Title string

	// Priority determines display order and update frequency
	Priority PanelPriority

	// RefreshInterval is how often the panel should poll for data updates.
	// Zero means no automatic refresh (manual only).
	RefreshInterval time.Duration

	// MinWidth is the minimum width the panel needs to render properly
	MinWidth int

	// MinHeight is the minimum height the panel needs to render properly
	MinHeight int

	// Collapsible indicates whether the panel can be collapsed
	Collapsible bool

	// DefaultCollapsed is the initial collapsed state
	DefaultCollapsed bool

	// ShowInTiers specifies which layout tiers should display this panel.
	// Empty means show in all tiers.
	ShowInTiers []string
}

// DefaultPanelConfig returns a PanelConfig with sensible defaults.
func DefaultPanelConfig(id, title string) PanelConfig {
	return PanelConfig{
		ID:               id,
		Title:            title,
		Priority:         PriorityNormal,
		RefreshInterval:  5 * time.Second,
		MinWidth:         20,
		MinHeight:        5,
		Collapsible:      true,
		DefaultCollapsed: false,
	}
}

// Panel defines a dashboard panel component.
// Embeds tea.Model for Bubble Tea integration and adds panel-specific methods.
type Panel interface {
	tea.Model

	// SetSize sets the panel dimensions for rendering
	SetSize(width, height int)

	// Focus marks the panel as focused (receives keyboard input)
	Focus()

	// Blur marks the panel as unfocused
	Blur()

	// Config returns the panel's configuration
	Config() PanelConfig

	// Keybindings returns panel-specific keyboard shortcuts.
	// These are active when the panel is focused.
	Keybindings() []Keybinding
}

// PanelBase provides common functionality for panel implementations.
// Embed this in concrete panel types to get default implementations.
type PanelBase struct {
	config  PanelConfig
	width   int
	height  int
	focused bool
}

// NewPanelBase creates a new PanelBase with the given config.
func NewPanelBase(cfg PanelConfig) PanelBase {
	return PanelBase{config: cfg}
}

// SetSize implements Panel.SetSize
func (b *PanelBase) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// Focus implements Panel.Focus
func (b *PanelBase) Focus() {
	b.focused = true
}

// Blur implements Panel.Blur
func (b *PanelBase) Blur() {
	b.focused = false
}

// Config implements Panel.Config
func (b *PanelBase) Config() PanelConfig {
	return b.config
}

// Keybindings returns empty keybindings by default.
// Override in concrete panels for panel-specific shortcuts.
func (b *PanelBase) Keybindings() []Keybinding {
	return nil
}

// IsFocused returns whether the panel is focused
func (b *PanelBase) IsFocused() bool {
	return b.focused
}

// Width returns the current panel width
func (b *PanelBase) Width() int {
	return b.width
}

// Height returns the current panel height
func (b *PanelBase) Height() int {
	return b.height
}

// PadToHeight pads content with empty lines to fill the specified height.
// This prevents layout jitter when content varies in length.
func PadToHeight(content string, targetHeight int) string {
	if targetHeight <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	currentHeight := len(lines)
	if currentHeight >= targetHeight {
		return content
	}
	// Add empty lines to fill remaining space
	for i := currentHeight; i < targetHeight; i++ {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// TruncateToHeight truncates content to fit within targetHeight lines.
// Returns the truncated content.
func TruncateToHeight(content string, targetHeight int) string {
	if targetHeight <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= targetHeight {
		return content
	}
	return strings.Join(lines[:targetHeight], "\n")
}

// FitToHeight ensures content exactly fills targetHeight lines,
// truncating if too long or padding if too short.
func FitToHeight(content string, targetHeight int) string {
	if targetHeight <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")

	// Truncate if too long
	if len(lines) > targetHeight {
		lines = lines[:targetHeight]
	}

	// Pad if too short
	for len(lines) < targetHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

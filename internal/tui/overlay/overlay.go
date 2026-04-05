// Package overlay provides modal overlay components for the ClawGo TUI.
// Overlays are drawn on top of the main REPL content and intercept key events
// when active. The OverlayManager supports stacking multiple overlays.
package overlay

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
)

// Overlay is the interface that all modal overlay types must implement.
// An overlay captures key input when active and renders on top of the main view.
type Overlay interface {
	// Update processes input messages while the overlay is active.
	Update(msg tea.Msg) (Overlay, tea.Cmd)

	// View renders the overlay content within the given dimensions.
	View(width, height int) string

	// IsActive returns true if the overlay is still open.
	IsActive() bool

	// Dismiss closes the overlay, causing it to report inactive.
	Dismiss()
}

// OverlayResult carries the outcome when an overlay completes.
type OverlayResult struct {
	Action string // "select", "cancel", "toggle", etc.
	Value  string // Selected text or relevant value
	Index  int    // Selected item index (-1 if not applicable)
}

// OverlayResultMsg is the Bubble Tea message emitted when an overlay completes.
type OverlayResultMsg struct {
	Source string        // Overlay type that produced this result
	Result OverlayResult // The result data
}

// OverlayManager manages a stack of overlays. The topmost overlay receives
// all input. When an overlay is dismissed, it is popped from the stack.
type OverlayManager struct {
	stack []Overlay
}

// NewOverlayManager creates an empty overlay manager.
func NewOverlayManager() *OverlayManager {
	return &OverlayManager{}
}

// Push adds an overlay to the top of the stack.
func (m *OverlayManager) Push(o Overlay) {
	m.stack = append(m.stack, o)
}

// Pop removes and returns the topmost overlay. Returns nil if empty.
func (m *OverlayManager) Pop() Overlay {
	if len(m.stack) == 0 {
		return nil
	}
	top := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return top
}

// Current returns the topmost overlay without removing it. Returns nil if empty.
func (m *OverlayManager) Current() Overlay {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

// IsActive returns true if there is at least one active overlay on the stack.
func (m *OverlayManager) IsActive() bool {
	return len(m.stack) > 0 && m.stack[len(m.stack)-1].IsActive()
}

// Update delegates the message to the topmost overlay and auto-pops dismissed overlays.
func (m *OverlayManager) Update(msg tea.Msg) tea.Cmd {
	if len(m.stack) == 0 {
		return nil
	}

	top := m.stack[len(m.stack)-1]
	updated, cmd := top.Update(msg)
	m.stack[len(m.stack)-1] = updated

	// Auto-pop dismissed overlays
	if !updated.IsActive() {
		m.stack = m.stack[:len(m.stack)-1]
	}

	return cmd
}

// View renders the topmost overlay centered within the given dimensions.
// A dimmed background is drawn behind the overlay content.
func (m *OverlayManager) View(width, height int) string {
	if len(m.stack) == 0 {
		return ""
	}

	top := m.stack[len(m.stack)-1]
	content := top.View(width, height)

	// Build dimmed background overlay
	return renderOverlayFrame(content, width, height)
}

// Size returns the number of overlays on the stack.
func (m *OverlayManager) Size() int {
	return len(m.stack)
}

// Overlay styling
var (
	overlayBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#5C6370")).
				Padding(0, 1)

	overlayTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#61AFEF"))

	overlayDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3E4451"))

	overlayHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3E4451")).
				Foreground(lipgloss.Color("#ABB2BF"))

	overlayMatchStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5C07B")).
				Bold(true)
)

// renderOverlayFrame renders content centered with a dimmed background.
func renderOverlayFrame(content string, width, height int) string {
	contentLines := strings.Split(content, "\n")
	contentHeight := len(contentLines)

	// Calculate vertical centering
	topPad := (height - contentHeight) / 2
	if topPad < 1 {
		topPad = 1
	}

	var sb strings.Builder
	// Top dim area
	dimLine := strings.Repeat(" ", width)
	for i := 0; i < topPad; i++ {
		sb.WriteString(dimLine)
		sb.WriteString("\n")
	}

	// Content
	for _, line := range contentLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// truncateText truncates s to maxLen characters, appending "..." if truncated.
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatTitle renders a title bar for overlays.
func formatTitle(title string, width int) string {
	styled := overlayTitleStyle.Render(title)
	line := strings.Repeat("─", max(0, width-lipgloss.Width(styled)-2))
	return fmt.Sprintf("─%s─%s", styled, line)
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

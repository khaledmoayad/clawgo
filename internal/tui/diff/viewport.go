package diff

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// ViewportModel wraps the bubbles viewport for scrollable content display.
// It provides a simplified interface for the TUI to present large content
// (diffs, tool output) in a scrollable view with keyboard navigation.
type ViewportModel struct {
	inner viewport.Model
}

// NewViewportModel creates a new viewport with the given dimensions.
// The viewport inherits default key bindings from bubbles/v2/viewport:
// up/down arrows, j/k, page up/down, home/end for navigation.
func NewViewportModel(width, height int) ViewportModel {
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
	return ViewportModel{inner: vp}
}

// SetContent sets the scrollable content text.
func (m *ViewportModel) SetContent(content string) {
	m.inner.SetContent(content)
}

// SetSize updates the viewport dimensions (e.g., on terminal resize).
func (m *ViewportModel) SetSize(width, height int) {
	m.inner.SetWidth(width)
	m.inner.SetHeight(height)
}

// Width returns the viewport width.
func (m ViewportModel) Width() int {
	return m.inner.Width()
}

// Height returns the viewport height.
func (m ViewportModel) Height() int {
	return m.inner.Height()
}

// Update delegates message handling to the inner viewport model.
// This processes key presses for scrolling (arrows, j/k, page up/down, etc.).
func (m ViewportModel) Update(msg tea.Msg) (ViewportModel, tea.Cmd) {
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

// View renders the visible portion of the content.
func (m ViewportModel) View() string {
	return m.inner.View()
}

// AtTop returns true when the viewport is scrolled to the top.
func (m ViewportModel) AtTop() bool {
	return m.inner.AtTop()
}

// AtBottom returns true when the viewport is scrolled to the bottom.
func (m ViewportModel) AtBottom() bool {
	return m.inner.AtBottom()
}

// ScrollPercent returns the current scroll position as 0.0-1.0.
func (m ViewportModel) ScrollPercent() float64 {
	return m.inner.ScrollPercent()
}

// TotalLines returns the total number of lines in the content.
func (m ViewportModel) TotalLines() int {
	return m.inner.TotalLineCount()
}

// NeedsViewport returns true if contentLines exceeds the viewport height,
// meaning scrolling is needed to view all content.
func (m ViewportModel) NeedsViewport(contentLines int) bool {
	return contentLines > m.inner.Height()
}

// ScrollDown moves the viewport down by n lines.
func (m *ViewportModel) ScrollDown(n int) {
	m.inner.ScrollDown(n)
}

// ScrollUp moves the viewport up by n lines.
func (m *ViewportModel) ScrollUp(n int) {
	m.inner.ScrollUp(n)
}

// GotoTop scrolls to the top of the content.
func (m *ViewportModel) GotoTop() {
	m.inner.GotoTop()
}

// GotoBottom scrolls to the bottom of the content.
func (m *ViewportModel) GotoBottom() {
	m.inner.GotoBottom()
}

// ContentHeight returns the number of lines in the content,
// counting newlines in whatever was last set via SetContent.
func (m ViewportModel) ContentHeight(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

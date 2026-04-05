package overlay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// FullscreenOverlay implements the Ctrl+O fullscreen toggle.
// It displays content in a scrollable full-terminal view with a
// title bar at the top and a status bar showing scroll position at the bottom.
type FullscreenOverlay struct {
	content  string
	viewport viewport.Model
	title    string
	active   bool
}

// NewFullscreen creates a fullscreen overlay with scrollable content.
func NewFullscreen(title, content string, width, height int) *FullscreenOverlay {
	// Reserve 2 lines for title bar and status bar
	vpHeight := max(1, height-2)
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(vpHeight),
	)
	vp.SetContent(content)

	return &FullscreenOverlay{
		content:  content,
		viewport: vp,
		title:    title,
		active:   true,
	}
}

// Update processes key events for the fullscreen overlay.
// Escape or Ctrl+O dismisses it (toggle behavior).
// Scroll via arrows, j/k, Page Up/Down.
func (m *FullscreenOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		k := keyMsg.Key()
		switch {
		case k.Code == tea.KeyEscape:
			m.active = false
			return m, func() tea.Msg {
				return OverlayResultMsg{
					Source: "fullscreen",
					Result: OverlayResult{Action: "toggle", Index: -1},
				}
			}

		case k.Code == 'o' && k.Mod == tea.ModCtrl:
			// Ctrl+O toggles back
			m.active = false
			return m, func() tea.Msg {
				return OverlayResultMsg{
					Source: "fullscreen",
					Result: OverlayResult{Action: "toggle", Index: -1},
				}
			}
		}
	}

	// Delegate all other keys to viewport for scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the fullscreen overlay filling the entire terminal.
func (m *FullscreenOverlay) View(width, height int) string {
	// Update viewport dimensions if terminal was resized
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(max(1, height-2))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ABB2BF")).
		Background(lipgloss.Color("#3E4451")).
		Width(width)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370")).
		Background(lipgloss.Color("#282C34")).
		Width(width)

	// Title bar
	titleBar := titleStyle.Render(fmt.Sprintf(" %s", m.title))

	// Status bar with scroll position
	scrollPct := m.viewport.ScrollPercent()
	totalLines := m.viewport.TotalLineCount()
	statusText := fmt.Sprintf(" Scroll: arrows/j/k/PgUp/PgDn  Exit: Esc/Ctrl+O  %d lines  %.0f%%", totalLines, scrollPct*100)
	statusBar := statusStyle.Render(statusText)

	var sb strings.Builder
	sb.WriteString(titleBar)
	sb.WriteString("\n")
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n")
	sb.WriteString(statusBar)

	return sb.String()
}

// IsActive returns true if the fullscreen overlay is still open.
func (m *FullscreenOverlay) IsActive() bool {
	return m.active
}

// Dismiss closes the fullscreen overlay.
func (m *FullscreenOverlay) Dismiss() {
	m.active = false
}

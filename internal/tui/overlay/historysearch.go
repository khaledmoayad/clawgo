package overlay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// HistoryItem represents a past session prompt for the history search overlay.
type HistoryItem struct {
	Text      string // The prompt text
	Date      string // Human-readable date (e.g. "2024-01-15 14:30")
	SessionID string // Session identifier for resumption
}

// HistorySearchOverlay implements the Ctrl+R history search.
// It shows a filterable list of past session prompts, allowing
// the user to select one to inject into the current input.
type HistorySearchOverlay struct {
	items     []HistoryItem
	filtered  []HistoryItem
	cursor    int
	textinput textinput.Model
	active    bool
}

// NewHistorySearch creates a history search overlay from a list of past prompts.
func NewHistorySearch(history []HistoryItem) *HistorySearchOverlay {
	ti := textinput.New()
	ti.Placeholder = "Search history..."
	ti.Focus()
	ti.CharLimit = 200

	m := &HistorySearchOverlay{
		items:     history,
		filtered:  make([]HistoryItem, len(history)),
		textinput: ti,
		active:    true,
	}
	copy(m.filtered, history)
	return m
}

// Update processes key events for the history search overlay.
func (m *HistorySearchOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
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
					Source: "history",
					Result: OverlayResult{Action: "cancel", Index: -1},
				}
			}

		case k.Code == tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				selected := m.filtered[m.cursor]
				m.active = false
				return m, func() tea.Msg {
					return OverlayResultMsg{
						Source: "history",
						Result: OverlayResult{
							Action: "select",
							Value:  selected.Text,
							Index:  m.cursor,
						},
					}
				}
			}
			return m, nil

		case k.Code == tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case k.Code == tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	// Update text input and re-filter
	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	m.applyFilter()
	return m, cmd
}

// View renders the history search overlay.
func (m *HistorySearchOverlay) View(width, height int) string {
	boxWidth := min(width-4, 80)
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Title
	title := formatTitle("History Search (Ctrl+R)", boxWidth-2)

	// Text input
	inputView := m.textinput.View()

	// List area
	listHeight := min(height-8, len(m.filtered))
	if listHeight < 1 {
		listHeight = 1
	}
	if listHeight > 20 {
		listHeight = 20
	}

	// Render visible items
	var listLines []string
	startIdx := 0
	if m.cursor >= listHeight {
		startIdx = m.cursor - listHeight + 1
	}
	endIdx := min(startIdx+listHeight, len(m.filtered))

	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	for i := startIdx; i < endIdx; i++ {
		item := m.filtered[i]
		prompt := strings.ReplaceAll(item.Text, "\n", " ")
		prompt = truncateText(prompt, boxWidth-len(item.Date)-6)
		datePart := dateStyle.Render(item.Date)
		line := fmt.Sprintf("%s  %s", prompt, datePart)
		if i == m.cursor {
			line = overlayHighlightStyle.Render(fmt.Sprintf("▸ %s", line))
		} else {
			line = fmt.Sprintf("  %s", line)
		}
		listLines = append(listLines, line)
	}

	if len(listLines) == 0 {
		listLines = append(listLines, lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render("  No matching history entries"))
	}

	// Status bar
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render(
		fmt.Sprintf(" %d/%d entries  Enter:select  Esc:cancel", len(m.filtered), len(m.items)),
	)

	// Assemble content
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(inputView)
	sb.WriteString("\n")
	sb.WriteString(strings.Join(listLines, "\n"))
	sb.WriteString("\n")
	sb.WriteString(status)

	return overlayBorderStyle.Width(boxWidth).Render(sb.String())
}

// IsActive returns true if the history search is still open.
func (m *HistorySearchOverlay) IsActive() bool {
	return m.active
}

// Dismiss closes the history search.
func (m *HistorySearchOverlay) Dismiss() {
	m.active = false
}

// applyFilter filters history items by case-insensitive substring match on prompt text.
func (m *HistorySearchOverlay) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.textinput.Value()))
	if query == "" {
		m.filtered = make([]HistoryItem, len(m.items))
		copy(m.filtered, m.items)
	} else {
		m.filtered = m.filtered[:0]
		for _, item := range m.items {
			if strings.Contains(strings.ToLower(item.Text), query) {
				m.filtered = append(m.filtered, item)
			}
		}
	}

	// Reset cursor if out of range
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

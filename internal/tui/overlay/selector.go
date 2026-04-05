package overlay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// SelectorItem represents a single item in the message selector list.
type SelectorItem struct {
	Role    string // "user", "assistant", "tool_use", etc.
	Preview string // First ~60 chars of message content
	Index   int    // Original message index
}

// MessageSelectorOverlay implements the Ctrl+K message selector.
// It shows a filterable list of all conversation messages, allowing
// the user to jump to a specific message by selecting it.
type MessageSelectorOverlay struct {
	items     []SelectorItem
	filtered  []SelectorItem
	cursor    int
	textinput textinput.Model
	active    bool
}

// NewMessageSelector creates a message selector overlay from display messages.
// Each message is represented by its role and a content preview (first 60 chars).
func NewMessageSelector(messages []DisplayMessage) *MessageSelectorOverlay {
	items := make([]SelectorItem, 0, len(messages))
	for i, msg := range messages {
		preview := strings.ReplaceAll(msg.Content, "\n", " ")
		preview = truncateText(preview, 60)
		role := msg.Role
		if msg.ToolName != "" {
			role = msg.ToolName
		}
		items = append(items, SelectorItem{
			Role:    role,
			Preview: preview,
			Index:   i,
		})
	}

	ti := textinput.New()
	ti.Placeholder = "Filter messages..."
	ti.Focus()
	ti.CharLimit = 100

	m := &MessageSelectorOverlay{
		items:     items,
		filtered:  make([]SelectorItem, len(items)),
		textinput: ti,
		active:    true,
	}
	copy(m.filtered, items)
	return m
}

// Update processes key events for the message selector.
func (m *MessageSelectorOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
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
					Source: "selector",
					Result: OverlayResult{Action: "cancel", Index: -1},
				}
			}

		case k.Code == tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				selected := m.filtered[m.cursor]
				m.active = false
				return m, func() tea.Msg {
					return OverlayResultMsg{
						Source: "selector",
						Result: OverlayResult{
							Action: "select",
							Value:  selected.Preview,
							Index:  selected.Index,
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

// View renders the message selector overlay.
func (m *MessageSelectorOverlay) View(width, height int) string {
	boxWidth := min(width-4, 80)
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Title
	title := formatTitle("Message Selector (Ctrl+K)", boxWidth-2)

	// Text input
	inputView := m.textinput.View()

	// List area: reserve lines for title, input, status bar, borders
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
	for i := startIdx; i < endIdx; i++ {
		item := m.filtered[i]
		roleLabel := formatRole(item.Role)
		preview := truncateText(item.Preview, boxWidth-len(item.Role)-6)
		line := fmt.Sprintf("%s %s", roleLabel, preview)
		if i == m.cursor {
			line = overlayHighlightStyle.Render(fmt.Sprintf("▸ %s", line))
		} else {
			line = fmt.Sprintf("  %s", line)
		}
		listLines = append(listLines, line)
	}

	// Status bar
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render(
		fmt.Sprintf(" %d/%d messages  Enter:select  Esc:cancel", len(m.filtered), len(m.items)),
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

// IsActive returns true if the selector is still open.
func (m *MessageSelectorOverlay) IsActive() bool {
	return m.active
}

// Dismiss closes the selector.
func (m *MessageSelectorOverlay) Dismiss() {
	m.active = false
}

// applyFilter filters items by case-insensitive substring match on content preview.
func (m *MessageSelectorOverlay) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.textinput.Value()))
	if query == "" {
		m.filtered = make([]SelectorItem, len(m.items))
		copy(m.filtered, m.items)
	} else {
		m.filtered = m.filtered[:0]
		for _, item := range m.items {
			if strings.Contains(strings.ToLower(item.Preview), query) ||
				strings.Contains(strings.ToLower(item.Role), query) {
				m.filtered = append(m.filtered, item)
			}
		}
	}

	// Reset cursor if out of range
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// formatRole returns a styled role label for the selector list.
func formatRole(role string) string {
	switch role {
	case "user":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B9BD2")).Bold(true).Render("user")
	case "assistant":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E8B86D")).Bold(true).Render("assistant")
	case "thinking":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD")).Italic(true).Render("thinking")
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")).Render("error")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Render(role)
	}
}

// DisplayMessage mirrors the tui.DisplayMessage type to avoid circular imports.
// The overlay package defines its own version; callers convert when creating overlays.
type DisplayMessage struct {
	Role     string
	Content  string
	ToolName string
}

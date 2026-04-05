package overlay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// SearchMatch represents a single match found during transcript search.
type SearchMatch struct {
	MsgIndex   int // Index into the messages slice
	LineOffset int // Line number within the message content
	Start      int // Character start position of match on that line
	End        int // Character end position (exclusive)
}

// TranscriptSearchOverlay implements the Ctrl+F transcript search.
// It scans all messages for a query string, collecting match positions,
// and allows navigation between matches with highlighting.
type TranscriptSearchOverlay struct {
	messages     []DisplayMessage
	matches      []SearchMatch
	currentMatch int
	query        string
	textinput    textinput.Model
	active       bool
	scrollOffset int // Vertical scroll offset for the results view
}

// NewTranscriptSearch creates a transcript search overlay.
func NewTranscriptSearch(messages []DisplayMessage) *TranscriptSearchOverlay {
	ti := textinput.New()
	ti.Placeholder = "Search transcript..."
	ti.Focus()
	ti.CharLimit = 200

	return &TranscriptSearchOverlay{
		messages:  messages,
		textinput: ti,
		active:    true,
	}
}

// Update processes key events for the transcript search.
func (m *TranscriptSearchOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
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
					Source: "search",
					Result: OverlayResult{Action: "cancel", Index: -1},
				}
			}

		case k.Code == tea.KeyEnter, k.Code == tea.KeyDown:
			// Next match
			if len(m.matches) > 0 {
				m.currentMatch = (m.currentMatch + 1) % len(m.matches)
			}
			return m, nil

		case k.Code == tea.KeyUp:
			// Previous match (Shift+Up or just Up when navigating)
			if len(m.matches) > 0 {
				m.currentMatch--
				if m.currentMatch < 0 {
					m.currentMatch = len(m.matches) - 1
				}
			}
			return m, nil
		}
	}

	// Update text input
	prevQuery := m.textinput.Value()
	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)

	// Re-scan if query changed
	newQuery := m.textinput.Value()
	if newQuery != prevQuery {
		m.query = newQuery
		m.scanMatches()
		m.currentMatch = 0
	}

	return m, cmd
}

// View renders the transcript search overlay.
func (m *TranscriptSearchOverlay) View(width, height int) string {
	boxWidth := min(width-4, 90)
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Title
	title := formatTitle("Transcript Search (Ctrl+F)", boxWidth-2)

	// Text input with match count
	inputView := m.textinput.View()
	matchInfo := ""
	if m.query != "" {
		if len(m.matches) > 0 {
			matchInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379")).Render(
				fmt.Sprintf(" %d/%d matches", m.currentMatch+1, len(m.matches)),
			)
		} else {
			matchInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")).Render(" No matches")
		}
	}

	// Build message preview area with highlighted matches
	contentHeight := max(1, min(height-8, 20))
	var contentLines []string

	if m.query != "" && len(m.matches) > 0 {
		// Show context around current match
		curMatch := m.matches[m.currentMatch]
		contentLines = m.renderMatchContext(curMatch, boxWidth-4, contentHeight)
	} else if m.query != "" {
		contentLines = append(contentLines, lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render("  No matches found"))
	} else {
		contentLines = append(contentLines, lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render("  Type to search across all messages"))
	}

	// Status bar
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Render(
		" Enter/Down:next  Up:prev  Esc:close",
	)

	// Assemble
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(inputView)
	sb.WriteString(matchInfo)
	sb.WriteString("\n")
	sb.WriteString(strings.Join(contentLines, "\n"))
	sb.WriteString("\n")
	sb.WriteString(status)

	return overlayBorderStyle.Width(boxWidth).Render(sb.String())
}

// IsActive returns true if the search overlay is still open.
func (m *TranscriptSearchOverlay) IsActive() bool {
	return m.active
}

// Dismiss closes the search overlay.
func (m *TranscriptSearchOverlay) Dismiss() {
	m.active = false
}

// Matches returns the collected search matches (for testing).
func (m *TranscriptSearchOverlay) Matches() []SearchMatch {
	return m.matches
}

// scanMatches finds all case-insensitive substring matches across all messages.
func (m *TranscriptSearchOverlay) scanMatches() {
	m.matches = nil
	if m.query == "" {
		return
	}

	queryLower := strings.ToLower(m.query)

	for msgIdx, msg := range m.messages {
		lines := strings.Split(msg.Content, "\n")
		for lineIdx, line := range lines {
			lineLower := strings.ToLower(line)
			searchFrom := 0
			for {
				idx := strings.Index(lineLower[searchFrom:], queryLower)
				if idx == -1 {
					break
				}
				absStart := searchFrom + idx
				m.matches = append(m.matches, SearchMatch{
					MsgIndex:   msgIdx,
					LineOffset: lineIdx,
					Start:      absStart,
					End:        absStart + len(m.query),
				})
				searchFrom = absStart + len(m.query)
			}
		}
	}
}

// renderMatchContext renders the message containing the current match
// with the match text highlighted.
func (m *TranscriptSearchOverlay) renderMatchContext(match SearchMatch, maxWidth, maxLines int) []string {
	if match.MsgIndex >= len(m.messages) {
		return nil
	}

	msg := m.messages[match.MsgIndex]
	lines := strings.Split(msg.Content, "\n")

	var result []string

	// Message header
	roleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#61AFEF"))
	header := fmt.Sprintf("  %s [message %d]:", roleStyle.Render(msg.Role), match.MsgIndex+1)
	result = append(result, header)

	// Show lines around the match with highlighting
	startLine := max(0, match.LineOffset-2)
	endLine := min(len(lines), match.LineOffset+maxLines-2)

	for i := startLine; i < endLine; i++ {
		line := lines[i]
		displayed := truncateText(line, maxWidth)
		if m.query != "" {
			displayed = HighlightMatches(displayed, m.query)
		}
		prefix := "  "
		if i == match.LineOffset {
			prefix = "> "
		}
		result = append(result, prefix+displayed)
	}

	return result
}

// HighlightMatches wraps all case-insensitive occurrences of query in text
// with ANSI yellow background highlighting.
func HighlightMatches(text, query string) string {
	if query == "" {
		return text
	}

	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)

	var sb strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(textLower[lastEnd:], queryLower)
		if idx == -1 {
			break
		}
		absStart := lastEnd + idx
		absEnd := absStart + len(query)

		// Non-matching prefix
		sb.WriteString(text[lastEnd:absStart])
		// Highlighted match
		sb.WriteString(overlayMatchStyle.Render(text[absStart:absEnd]))

		lastEnd = absEnd
	}

	// Remainder
	sb.WriteString(text[lastEnd:])
	return sb.String()
}

// Package suggest provides a typeahead suggestion overlay for the ClawGo TUI.
// It supports pluggable suggestion providers (commands, file paths, shell history)
// and renders a dropdown below the input area with keyboard navigation.
package suggest

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// maxVisibleItems is the maximum number of suggestions shown at once.
const maxVisibleItems = 8

// defaultDebounce is the debounce duration for filesystem-backed lookups.
const defaultDebounce = 50 * time.Millisecond

// Suggestion represents a single typeahead suggestion item.
type Suggestion struct {
	Text        string // The text to insert on acceptance
	Description string // Brief description shown beside the text
	Icon        string // Prefix icon/glyph (e.g., "/", "@", "!")
	Provider    string // Name of the provider that generated this
}

// SuggestionAcceptMsg is sent when the user accepts a suggestion.
type SuggestionAcceptMsg struct {
	Text string
}

// SuggestionsReadyMsg carries new suggestions after a debounced lookup.
type SuggestionsReadyMsg struct {
	Items []Suggestion
}

// SuggestModel manages the suggestion dropdown UI and provider dispatch.
type SuggestModel struct {
	active        bool
	items         []Suggestion
	cursor        int
	providers     []SuggestionProvider
	debounceTimer *time.Timer
	lastInput     string
	width         int
	scrollOffset  int
}

// NewSuggestModel creates a new suggestion model with the given providers.
func NewSuggestModel(providers ...SuggestionProvider) SuggestModel {
	return SuggestModel{
		providers: providers,
	}
}

// OnInputChange triggers suggestion refresh after debounce.
// It evaluates all providers, selects the first matching one, and returns
// a command that will deliver SuggestionsReadyMsg after the debounce period.
func (m *SuggestModel) OnInputChange(text string, cursorPos int) tea.Cmd {
	m.lastInput = text

	// Find the first matching provider
	var matched SuggestionProvider
	for _, p := range m.providers {
		if p.Match(text, cursorPos) {
			matched = p
			break
		}
	}

	if matched == nil {
		// No provider matches -- dismiss suggestions
		m.active = false
		m.items = nil
		m.cursor = 0
		m.scrollOffset = 0
		return nil
	}

	// Capture provider for the closure
	provider := matched
	input := text
	pos := cursorPos

	// Cancel existing timer
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}

	// Return a command that debounces the lookup
	return func() tea.Msg {
		time.Sleep(defaultDebounce)
		items := provider.Suggest(input, pos)
		return SuggestionsReadyMsg{Items: items}
	}
}

// IsActive returns true if suggestions are currently visible.
func (m *SuggestModel) IsActive() bool {
	return m.active
}

// SelectedText returns the text of the currently selected suggestion,
// or empty string if no suggestion is selected.
func (m *SuggestModel) SelectedText() string {
	if !m.active || m.cursor < 0 || m.cursor >= len(m.items) {
		return ""
	}
	return m.items[m.cursor].Text
}

// Items returns the current suggestion list.
func (m *SuggestModel) Items() []Suggestion {
	return m.items
}

// Cursor returns the current cursor position.
func (m *SuggestModel) Cursor() int {
	return m.cursor
}

// Update processes key events for the suggestion dropdown.
func (m SuggestModel) Update(msg tea.Msg) (SuggestModel, tea.Cmd) {
	switch msg := msg.(type) {
	case SuggestionsReadyMsg:
		if len(msg.Items) > 0 {
			m.active = true
			m.items = msg.Items
			m.cursor = 0
			m.scrollOffset = 0
		} else {
			m.active = false
			m.items = nil
			m.cursor = 0
			m.scrollOffset = 0
		}
		return m, nil

	case tea.KeyPressMsg:
		if !m.active {
			return m, nil
		}

		k := msg.Key()
		switch {
		case k.Code == tea.KeyEscape:
			m.active = false
			m.items = nil
			m.cursor = 0
			m.scrollOffset = 0
			return m, nil

		case k.Code == tea.KeyEnter:
			if len(m.items) > 0 && m.cursor < len(m.items) {
				text := m.items[m.cursor].Text
				m.active = false
				m.items = nil
				m.cursor = 0
				m.scrollOffset = 0
				return m, func() tea.Msg { return SuggestionAcceptMsg{Text: text} }
			}
			return m, nil

		case k.Code == tea.KeyTab || k.Code == tea.KeyDown:
			if len(m.items) > 0 {
				m.cursor = (m.cursor + 1) % len(m.items)
				m.adjustScroll()
			}
			return m, nil

		case (k.Code == tea.KeyTab && k.Mod&tea.ModShift != 0) || k.Code == tea.KeyUp:
			if len(m.items) > 0 {
				m.cursor = (m.cursor + len(m.items) - 1) % len(m.items)
				m.adjustScroll()
			}
			return m, nil
		}
	}

	return m, nil
}

// adjustScroll ensures the cursor is visible within the scroll window.
func (m *SuggestModel) adjustScroll() {
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+maxVisibleItems {
		m.scrollOffset = m.cursor - maxVisibleItems + 1
	}
}

// Suggestion styles
var (
	suggestBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#5C6370")).
				Padding(0, 1)

	suggestActiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3E4451")).
				Foreground(lipgloss.Color("#ABB2BF"))

	suggestNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ABB2BF"))

	suggestDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5C6370"))

	suggestIconStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5C07B")).
				Bold(true)
)

// View renders the suggestion dropdown.
func (m SuggestModel) View() string {
	if !m.active || len(m.items) == 0 {
		return ""
	}

	w := m.width
	if w < 30 {
		w = 40
	}
	if w > 60 {
		w = 60
	}

	endIdx := m.scrollOffset + maxVisibleItems
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	var lines []string
	for i := m.scrollOffset; i < endIdx; i++ {
		item := m.items[i]
		icon := suggestIconStyle.Render(item.Icon)
		text := item.Text
		desc := ""
		if item.Description != "" {
			desc = " " + suggestDescStyle.Render(item.Description)
		}
		line := fmt.Sprintf(" %s %s%s", icon, text, desc)
		if i == m.cursor {
			line = suggestActiveStyle.Render(line)
		} else {
			line = suggestNormalStyle.Render(line)
		}
		lines = append(lines, line)
	}

	// Scroll indicator
	if len(m.items) > maxVisibleItems {
		indicator := suggestDescStyle.Render(
			fmt.Sprintf(" %d/%d", m.cursor+1, len(m.items)),
		)
		lines = append(lines, indicator)
	}

	content := strings.Join(lines, "\n")
	return suggestBorderStyle.Width(w).Render(content)
}

// SetWidth sets the available width for the suggestion dropdown.
func (m *SuggestModel) SetWidth(w int) {
	m.width = w
}

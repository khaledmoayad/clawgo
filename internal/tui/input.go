package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
)

// InputModel manages the multi-line prompt input area.
type InputModel struct {
	textarea     textarea.Model
	keys         KeyMap
	focused      bool
	vimNormal    bool     // true when vim is in Normal mode (blocks typing)
	commandNames []string // available slash command names for tab completion
}

// NewInputModel creates an input sub-model with a multi-line textarea.
func NewInputModel() InputModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Shift+Enter for newline)"
	ta.CharLimit = 0 // unlimited
	ta.ShowLineNumbers = false
	ta.SetHeight(3) // initial 3 lines
	ta.Focus()
	return InputModel{textarea: ta, keys: DefaultKeyMap(), focused: true}
}

// Update processes key events for the input area.
// Enter (without Shift) sends a SubmitMsg.
// Tab completes slash commands when input starts with "/".
// Shift+Enter inserts a newline (handled by textarea).
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	// Handle key messages for submit and tab completion
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		k := keyMsg.Key()

		// Submit on Enter (without Shift)
		if m.keys.IsSubmit(k) && !m.keys.IsNewLine(k) {
			text := m.textarea.Value()
			if strings.TrimSpace(text) != "" {
				return m, func() tea.Msg { return SubmitMsg{Text: text} }
			}
			return m, nil
		}

		// Tab completion for slash commands
		if k.Code == tea.KeyTab && len(m.commandNames) > 0 {
			text := m.textarea.Value()
			if completed, ok := m.tryCompleteCommand(text); ok {
				m.textarea.SetValue(completed)
				// Move cursor to end
				m.textarea.CursorEnd()
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// tryCompleteCommand attempts to complete a partial slash command.
// Returns the completed text and true if exactly one match is found.
func (m InputModel) tryCompleteCommand(text string) (string, bool) {
	if !strings.HasPrefix(text, "/") {
		return "", false
	}

	// Extract the partial command name (everything after "/" before first space)
	partial := text[1:]
	if strings.Contains(partial, " ") {
		// Already has arguments, no completion
		return "", false
	}

	partial = strings.ToLower(partial)
	if partial == "" {
		return "", false
	}

	var matches []string
	for _, name := range m.commandNames {
		if strings.HasPrefix(name, partial) {
			matches = append(matches, name)
		}
	}

	if len(matches) == 1 {
		return "/" + matches[0], true
	}

	return "", false
}

// View renders the input textarea.
func (m InputModel) View() string {
	return m.textarea.View()
}

// Value returns the current input text.
func (m InputModel) Value() string { return m.textarea.Value() }

// Reset clears the input area.
func (m *InputModel) Reset() {
	m.textarea.Reset()
}

// Focus gives input focus to the textarea.
func (m *InputModel) Focus() tea.Cmd {
	m.focused = true
	return m.textarea.Focus()
}

// Blur removes input focus from the textarea.
func (m *InputModel) Blur() {
	m.textarea.Blur()
	m.focused = false
}

// SetCommandNames sets the list of available slash command names for tab completion.
func (m *InputModel) SetCommandNames(names []string) {
	m.commandNames = names
}

// SetVimNormal sets vim normal mode state. When true, the textarea is blurred
// (no cursor, no editing). When false, the textarea is focused for text input.
func (m *InputModel) SetVimNormal(normal bool) {
	m.vimNormal = normal
	if normal {
		m.textarea.Blur()
	} else if m.focused {
		m.textarea.Focus()
	}
}

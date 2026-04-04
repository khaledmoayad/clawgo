package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
)

// InputModel manages the multi-line prompt input area.
type InputModel struct {
	textarea  textarea.Model
	keys      KeyMap
	focused   bool
	vimNormal bool // true when vim is in Normal mode (blocks typing)
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
// Shift+Enter inserts a newline (handled by textarea).
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	// Handle key messages for submit
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		k := keyMsg.Key()
		if m.keys.IsSubmit(k) && !m.keys.IsNewLine(k) {
			text := m.textarea.Value()
			if strings.TrimSpace(text) != "" {
				return m, func() tea.Msg { return SubmitMsg{Text: text} }
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
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

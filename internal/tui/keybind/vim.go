package keybind

import tea "charm.land/bubbletea/v2"

// VimMode represents the current vim editing mode.
type VimMode int

const (
	// ModeInsert is the default text editing mode.
	ModeInsert VimMode = iota
	// ModeNormal is the vim navigation mode.
	ModeNormal
	// ModeVisual is a stub for future visual selection mode.
	ModeVisual
)

// VimModel manages vim-style keybinding state.
// It tracks the current mode and pending multi-key commands like "dd" or "gg".
type VimModel struct {
	mode    VimMode
	enabled bool
	pending string // stores partial commands like first "d" of "dd"
}

// NewVimModel creates a new VimModel, starting disabled in Insert mode.
func NewVimModel() VimModel {
	return VimModel{
		mode:    ModeInsert,
		enabled: false,
	}
}

// Toggle switches the enabled state. When enabling, starts in Normal mode.
// When disabling, resets to Insert mode.
func (m *VimModel) Toggle() {
	m.enabled = !m.enabled
	if m.enabled {
		m.mode = ModeNormal
	} else {
		m.mode = ModeInsert
	}
	m.pending = ""
}

// SetEnabled sets the enabled state explicitly.
func (m *VimModel) SetEnabled(enabled bool) {
	m.enabled = enabled
	if enabled {
		m.mode = ModeNormal
	} else {
		m.mode = ModeInsert
	}
	m.pending = ""
}

// IsEnabled returns whether vim mode is active.
func (m VimModel) IsEnabled() bool { return m.enabled }

// Mode returns the current vim mode.
func (m VimModel) Mode() VimMode { return m.mode }

// IsNormal returns true if in Normal mode.
func (m VimModel) IsNormal() bool { return m.mode == ModeNormal }

// IsInsert returns true if in Insert mode.
func (m VimModel) IsInsert() bool { return m.mode == ModeInsert }

// ModeString returns a display string for the current mode ("NORMAL", "INSERT", "VISUAL").
func (m VimModel) ModeString() string {
	switch m.mode {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	default:
		return "UNKNOWN"
	}
}

// HandleKey processes a key press through vim mode logic.
// Returns the semantic action and whether the key was consumed.
// If not enabled, all keys pass through (consumed=false).
func (m *VimModel) HandleKey(k tea.Key) (action Action, consumed bool) {
	if !m.enabled {
		return ActionNone, false
	}

	switch m.mode {
	case ModeInsert:
		return m.handleInsert(k)
	case ModeNormal:
		return m.handleNormal(k)
	default:
		return ActionNone, false
	}
}

// handleInsert processes keys in Insert mode.
// Only Escape is intercepted to switch to Normal mode.
func (m *VimModel) handleInsert(k tea.Key) (Action, bool) {
	if k.Code == tea.KeyEscape {
		m.mode = ModeNormal
		m.pending = ""
		return ActionNone, true
	}
	return ActionNone, false
}

// handleNormal processes keys in Normal mode with vim navigation.
func (m *VimModel) handleNormal(k tea.Key) (Action, bool) {
	// Check for pending multi-key sequences first
	if m.pending != "" {
		return m.handlePending(k)
	}

	switch k.Code {
	case 'i':
		m.mode = ModeInsert
		return ActionNone, true
	case 'h':
		// Left movement (consumed, caller handles cursor)
		return ActionNone, true
	case 'j':
		return ActionScrollDown, true
	case 'k':
		return ActionScrollUp, true
	case 'l':
		// Right movement (consumed, caller handles cursor)
		return ActionNone, true
	case '0':
		return ActionHome, true
	case '$':
		return ActionEnd, true
	case 'G':
		return ActionEnd, true
	case 'g':
		m.pending = "g"
		return ActionNone, true
	case 'd':
		m.pending = "d"
		return ActionNone, true
	case '/':
		// Search stub for future implementation
		return ActionNone, true
	case tea.KeyEscape:
		// Stay in normal mode, consume the key
		return ActionNone, true
	default:
		// Unknown keys in normal mode pass through
		return ActionNone, false
	}
}

// handlePending processes the second key of a multi-key sequence.
func (m *VimModel) handlePending(k tea.Key) (Action, bool) {
	pending := m.pending
	m.pending = ""

	switch pending {
	case "g":
		if k.Code == 'g' {
			return ActionHome, true
		}
		// Invalid sequence, discard
		return ActionNone, true
	case "d":
		if k.Code == 'd' {
			return ActionDeleteLine, true
		}
		// Invalid sequence, discard
		return ActionNone, true
	default:
		return ActionNone, true
	}
}

package keybind

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// VimMode represents the current vim editing mode.
type VimMode int

const (
	// ModeInsert is the default text editing mode.
	ModeInsert VimMode = iota
	// ModeNormal is the vim navigation mode.
	ModeNormal
	// ModeVisual is a stub for future visual selection mode.
	ModeVisual
	// ModeOperatorPending waits for a motion/text-object after an operator key.
	ModeOperatorPending
	// ModeSearch is active when the user is typing a / search query.
	ModeSearch
)

// MaxVimCount caps numeric prefix to prevent huge allocations.
const MaxVimCount = 10000

// UndoEntry captures a snapshot for undo/redo.
type UndoEntry struct {
	Text   string
	Cursor int
}

// VimModel manages vim-style keybinding state with full operator/motion/text-object
// composition, search, undo/redo, and scroll keybindings.
type VimModel struct {
	mode    VimMode
	enabled bool

	// Operator pending state
	pendingOp    Operator
	pendingCount int
	countAccum   string
	pending      string // stores partial commands like first "g" of "gg"

	// Register for yank/delete/paste
	register Register

	// Search state
	searchQuery string
	searchDir   int // 1=forward, -1=backward

	// Undo/redo stacks
	undoStack []UndoEntry
	redoStack []UndoEntry
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
	m.resetPending()
}

// SetEnabled sets the enabled state explicitly.
func (m *VimModel) SetEnabled(enabled bool) {
	m.enabled = enabled
	if enabled {
		m.mode = ModeNormal
	} else {
		m.mode = ModeInsert
	}
	m.resetPending()
}

// IsEnabled returns whether vim mode is active.
func (m VimModel) IsEnabled() bool { return m.enabled }

// Mode returns the current vim mode.
func (m VimModel) Mode() VimMode { return m.mode }

// IsNormal returns true if in Normal mode.
func (m VimModel) IsNormal() bool { return m.mode == ModeNormal }

// IsInsert returns true if in Insert mode.
func (m VimModel) IsInsert() bool { return m.mode == ModeInsert }

// IsSearch returns true if in Search mode.
func (m VimModel) IsSearch() bool { return m.mode == ModeSearch }

// SearchQuery returns the current search query string.
func (m VimModel) SearchQuery() string { return m.searchQuery }

// ModeString returns a display string for the current mode.
func (m VimModel) ModeString() string {
	switch m.mode {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeOperatorPending:
		return "OP-PENDING"
	case ModeSearch:
		return "SEARCH"
	default:
		return "UNKNOWN"
	}
}

// PushUndo saves a snapshot to the undo stack. Call before text modifications.
func (m *VimModel) PushUndo(text string, cursor int) {
	m.undoStack = append(m.undoStack, UndoEntry{Text: text, Cursor: cursor})
	// Clear redo stack on new edit
	m.redoStack = m.redoStack[:0]
}

// Undo pops the last undo entry. Returns the entry and true, or zero value and false.
func (m *VimModel) Undo() (UndoEntry, bool) {
	if len(m.undoStack) == 0 {
		return UndoEntry{}, false
	}
	entry := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	return entry, true
}

// PushRedo saves a snapshot to the redo stack.
func (m *VimModel) PushRedo(text string, cursor int) {
	m.redoStack = append(m.redoStack, UndoEntry{Text: text, Cursor: cursor})
}

// Redo pops the last redo entry. Returns the entry and true, or zero value and false.
func (m *VimModel) Redo() (UndoEntry, bool) {
	if len(m.redoStack) == 0 {
		return UndoEntry{}, false
	}
	entry := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	return entry, true
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
	case ModeNormal, ModeOperatorPending:
		return m.handleNormal(k)
	case ModeSearch:
		return m.handleSearch(k)
	default:
		return ActionNone, false
	}
}

// handleInsert processes keys in Insert mode.
// Only Escape is intercepted to switch to Normal mode.
func (m *VimModel) handleInsert(k tea.Key) (Action, bool) {
	if k.Code == tea.KeyEscape {
		m.mode = ModeNormal
		m.resetPending()
		return ActionNone, true
	}
	return ActionNone, false
}

// handleNormal processes keys in Normal mode with full vim command parsing.
func (m *VimModel) handleNormal(k tea.Key) (Action, bool) {
	// Handle Ctrl key combinations first (work in both vim and non-vim modes)
	if k.Mod&tea.ModCtrl != 0 {
		return m.handleCtrl(k)
	}

	// Escape cancels any pending state
	if k.Code == tea.KeyEscape {
		m.resetPending()
		m.mode = ModeNormal
		return ActionNone, true
	}

	ch := k.Code

	// If we're in operator-pending and waiting for a text object scope/type
	if m.pending != "" {
		return m.handlePending(k)
	}

	// Accumulate count prefix (1-9 start, 0 continues if digits already present)
	if ch >= '1' && ch <= '9' || (ch == '0' && m.countAccum != "") {
		m.countAccum += string(ch)
		return ActionNone, true
	}

	count := m.effectiveCount()

	// Operators: d, c, y
	switch ch {
	case 'd':
		if m.pendingOp == OpDelete {
			// dd = line operation
			m.resetPending()
			return ActionDeleteLine, true
		}
		m.pendingOp = OpDelete
		m.pendingCount = count
		m.countAccum = ""
		m.mode = ModeOperatorPending
		return ActionNone, true
	case 'c':
		if m.pendingOp == OpChange {
			// cc = change line
			m.resetPending()
			return ActionChangeRange, true
		}
		m.pendingOp = OpChange
		m.pendingCount = count
		m.countAccum = ""
		m.mode = ModeOperatorPending
		return ActionNone, true
	case 'y':
		if m.pendingOp == OpYank {
			// yy = yank line
			m.resetPending()
			return ActionYankRange, true
		}
		m.pendingOp = OpYank
		m.pendingCount = count
		m.countAccum = ""
		m.mode = ModeOperatorPending
		return ActionNone, true
	}

	// Motions - if an operator is pending, these complete the operator
	if m.pendingOp != OpNone {
		return m.handleOperatorMotion(ch, count)
	}

	// Simple motions (no operator pending)
	switch ch {
	case 'h':
		m.resetPending()
		return ActionNone, true // cursor left - consumed, caller handles
	case 'j':
		m.resetPending()
		return ActionScrollDown, true
	case 'k':
		m.resetPending()
		return ActionScrollUp, true
	case 'l':
		m.resetPending()
		return ActionNone, true // cursor right - consumed, caller handles
	case 'w', 'W', 'b', 'B', 'e', 'E':
		m.resetPending()
		return ActionNone, true // word motions consumed, caller moves cursor
	case '0':
		m.resetPending()
		return ActionHome, true
	case '$':
		m.resetPending()
		return ActionEnd, true
	case '^':
		m.resetPending()
		return ActionHome, true // first non-blank
	case 'G':
		m.resetPending()
		return ActionScrollToBottom, true
	case 'g':
		m.pending = "g"
		return ActionNone, true
	case 'i':
		m.resetPending()
		m.mode = ModeInsert
		return ActionNone, true
	case 'I':
		m.resetPending()
		m.mode = ModeInsert
		return ActionHome, true
	case 'a':
		m.resetPending()
		m.mode = ModeInsert
		return ActionNone, true // caller positions cursor after current char
	case 'A':
		m.resetPending()
		m.mode = ModeInsert
		return ActionEnd, true
	case 'o':
		m.resetPending()
		m.mode = ModeInsert
		return ActionNone, true // caller opens line below
	case 'O':
		m.resetPending()
		m.mode = ModeInsert
		return ActionNone, true // caller opens line above
	case 'p':
		m.resetPending()
		return ActionPaste, true
	case 'P':
		m.resetPending()
		return ActionPasteBefore, true
	case 'u':
		m.resetPending()
		return ActionUndo, true
	case 'x':
		m.resetPending()
		return ActionDeleteRange, true // delete char under cursor
	case '/':
		m.resetPending()
		m.mode = ModeSearch
		m.searchQuery = ""
		m.searchDir = 1
		return ActionSearchStart, true
	case '?':
		m.resetPending()
		m.mode = ModeSearch
		m.searchQuery = ""
		m.searchDir = -1
		return ActionSearchStart, true
	case 'n':
		m.resetPending()
		return ActionSearchNext, true
	case 'N':
		m.resetPending()
		return ActionSearchPrev, true
	}

	// Unknown keys pass through
	m.resetPending()
	return ActionNone, false
}

// handleCtrl processes ctrl-key combinations for both vim and non-vim scroll keys.
func (m *VimModel) handleCtrl(k tea.Key) (Action, bool) {
	switch k.Code {
	case 'd':
		m.resetPending()
		return ActionHalfPageDown, true
	case 'u':
		m.resetPending()
		return ActionHalfPageUp, true
	case 'r':
		m.resetPending()
		return ActionRedo, true
	case 'f':
		m.resetPending()
		return ActionPageDown, true
	case 'b':
		m.resetPending()
		return ActionPageUp, true
	default:
		return ActionNone, false
	}
}

// handleOperatorMotion handles a motion or text-object key when an operator is pending.
func (m *VimModel) handleOperatorMotion(ch rune, count int) (Action, bool) {
	op := m.pendingOp
	opCount := m.pendingCount
	if opCount < 1 {
		opCount = 1
	}

	switch ch {
	case 'w', 'W', 'b', 'B', 'e', 'E',
		'h', 'l', 'j', 'k',
		'0', '$', '^':
		// Motion completes operator
		m.resetPending()
		switch op {
		case OpDelete:
			return ActionDeleteRange, true
		case OpChange:
			return ActionChangeRange, true
		case OpYank:
			return ActionYankRange, true
		}
	case 'i', 'a':
		// Start text object (e.g., diw, ci", yaw)
		m.pending = string(ch)
		return ActionNone, true
	case 'G':
		m.resetPending()
		switch op {
		case OpDelete:
			return ActionDeleteRange, true
		case OpChange:
			return ActionChangeRange, true
		case OpYank:
			return ActionYankRange, true
		}
	case 'g':
		m.pending = "g"
		return ActionNone, true
	case 'f', 'F', 't', 'T':
		// Find motion - need one more char
		m.pending = string(ch)
		return ActionNone, true
	}

	// Invalid sequence - cancel
	m.resetPending()
	return ActionNone, true
}

// handlePending processes the next key of a multi-key sequence.
func (m *VimModel) handlePending(k tea.Key) (Action, bool) {
	pending := m.pending
	m.pending = ""
	ch := k.Code

	switch pending {
	case "g":
		if ch == 'g' {
			if m.pendingOp != OpNone {
				// Operator + gg (e.g., dgg)
				m.resetPending()
				return ActionDeleteRange, true
			}
			m.resetPending()
			return ActionScrollToTop, true
		}
		// Invalid g-sequence
		m.resetPending()
		return ActionNone, true

	case "i", "a":
		// Text object type key (e.g., w, ", (, etc.)
		obj := TextObjectFromKeys(pending[0], byte(ch))
		if obj == ObjNone {
			m.resetPending()
			return ActionNone, true
		}
		op := m.pendingOp
		m.resetPending()
		switch op {
		case OpDelete:
			return ActionDeleteRange, true
		case OpChange:
			return ActionChangeRange, true
		case OpYank:
			return ActionYankRange, true
		default:
			return ActionNone, true
		}

	case "f", "F", "t", "T":
		// Find char motion
		op := m.pendingOp
		m.resetPending()
		if op != OpNone {
			switch op {
			case OpDelete:
				return ActionDeleteRange, true
			case OpChange:
				return ActionChangeRange, true
			case OpYank:
				return ActionYankRange, true
			}
		}
		return ActionNone, true
	}

	m.resetPending()
	return ActionNone, true
}

// handleSearch processes keys in Search mode.
func (m *VimModel) handleSearch(k tea.Key) (Action, bool) {
	switch k.Code {
	case tea.KeyEscape:
		// Cancel search
		m.mode = ModeNormal
		m.searchQuery = ""
		return ActionNone, true
	case tea.KeyEnter:
		// Execute search
		m.mode = ModeNormal
		return ActionSearchNext, true
	case tea.KeyBackspace:
		// Delete last char of search query
		if len(m.searchQuery) > 0 {
			_, sz := utf8.DecodeLastRuneInString(m.searchQuery)
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-sz]
		}
		return ActionNone, true
	default:
		// Append character to search query
		if k.Code >= ' ' {
			m.searchQuery += string(k.Code)
		}
		return ActionNone, true
	}
}

// effectiveCount returns the accumulated count, defaulting to 1.
func (m *VimModel) effectiveCount() int {
	if m.countAccum == "" {
		return 1
	}
	n, err := strconv.Atoi(m.countAccum)
	if err != nil || n <= 0 {
		return 1
	}
	if n > MaxVimCount {
		return MaxVimCount
	}
	return n
}

// resetPending clears all pending state back to idle normal mode.
func (m *VimModel) resetPending() {
	m.pendingOp = OpNone
	m.pendingCount = 0
	m.countAccum = ""
	m.pending = ""
	if m.mode == ModeOperatorPending {
		m.mode = ModeNormal
	}
}

// FindInText searches for query in text starting from offset.
// Returns a list of match start positions.
func FindInText(text, query string, forward bool) []int {
	if query == "" {
		return nil
	}
	var matches []int
	lower := strings.ToLower(text)
	q := strings.ToLower(query)
	start := 0
	for {
		idx := strings.Index(lower[start:], q)
		if idx < 0 {
			break
		}
		matches = append(matches, start+idx)
		start += idx + 1
	}
	return matches
}

// NextMatch finds the next match position after cursor (wrapping).
func NextMatch(matches []int, cursor int) int {
	if len(matches) == 0 {
		return -1
	}
	for _, m := range matches {
		if m > cursor {
			return m
		}
	}
	return matches[0] // wrap
}

// PrevMatch finds the previous match position before cursor (wrapping).
func PrevMatch(matches []int, cursor int) int {
	if len(matches) == 0 {
		return -1
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i] < cursor {
			return matches[i]
		}
	}
	return matches[len(matches)-1] // wrap
}

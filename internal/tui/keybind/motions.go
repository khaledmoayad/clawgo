package keybind

import "unicode"

// Motion represents a vim cursor motion.
type Motion int

const (
	MotionNone         Motion = iota
	MotionLeft                // h
	MotionRight               // l
	MotionDown                // j
	MotionUp                  // k
	MotionWord                // w - next word start
	MotionWordEnd             // e - end of word
	MotionWordBack            // b - previous word start
	MotionBigWord             // W - next WORD start (whitespace-delimited)
	MotionBigWordEnd          // E - end of WORD
	MotionBigWordBack         // B - previous WORD start
	MotionLineStart           // 0 - column 0
	MotionLineEnd             // $ - end of line
	MotionFirstNonBlank       // ^ - first non-whitespace
	MotionFindChar            // f{char} - find char forward
	MotionTillChar            // t{char} - till char forward
	MotionFindCharBack        // F{char} - find char backward
	MotionTillCharBack        // T{char} - till char backward
	MotionTop                 // gg - top of text
	MotionBottom              // G - bottom of text
)

// ResolveMotion calculates the new cursor position after applying a motion.
// count multiplies the motion (e.g. 3w = three words forward).
// findChar is used for f/t/F/T motions.
func ResolveMotion(m Motion, text string, cursor int, count int, findChar rune) int {
	if count < 1 {
		count = 1
	}
	pos := cursor
	for i := 0; i < count; i++ {
		newPos := applySingleMotion(m, text, pos, findChar)
		if newPos == pos {
			break // no progress
		}
		pos = newPos
	}
	return pos
}

// MotionRange returns the text range [start, end) that a motion covers from the cursor.
// Used for operator composition (e.g., dw deletes the range the w motion covers).
func MotionRange(m Motion, text string, cursor int, count int, findChar rune) (start, end int) {
	newCursor := ResolveMotion(m, text, cursor, count, findChar)
	if newCursor < cursor {
		start, end = newCursor, cursor
	} else {
		start, end = cursor, newCursor
	}

	// Inclusive motions include the character at the destination
	if isInclusiveMotion(m) && end < len(text) {
		end++ // include the char at end position
	}

	return start, end
}

// isInclusiveMotion returns true if the motion includes the character at destination.
func isInclusiveMotion(m Motion) bool {
	switch m {
	case MotionWordEnd, MotionBigWordEnd, MotionLineEnd,
		MotionFindChar, MotionFindCharBack:
		return true
	default:
		return false
	}
}

// IsLinewiseMotion returns true if the motion operates on full lines with operators.
func IsLinewiseMotion(m Motion) bool {
	switch m {
	case MotionDown, MotionUp, MotionTop, MotionBottom:
		return true
	default:
		return false
	}
}

// applySingleMotion applies one step of a motion.
func applySingleMotion(m Motion, text string, cursor int, findChar rune) int {
	switch m {
	case MotionLeft:
		return moveLeft(text, cursor)
	case MotionRight:
		return moveRight(text, cursor)
	case MotionDown:
		return moveDown(text, cursor)
	case MotionUp:
		return moveUp(text, cursor)
	case MotionWord:
		return nextWordStart(text, cursor, false)
	case MotionWordEnd:
		return wordEnd(text, cursor, false)
	case MotionWordBack:
		return prevWordStart(text, cursor, false)
	case MotionBigWord:
		return nextWordStart(text, cursor, true)
	case MotionBigWordEnd:
		return wordEnd(text, cursor, true)
	case MotionBigWordBack:
		return prevWordStart(text, cursor, true)
	case MotionLineStart:
		return lineStart(text, cursor)
	case MotionLineEnd:
		return lineEnd(text, cursor)
	case MotionFirstNonBlank:
		return firstNonBlank(text, cursor)
	case MotionFindChar:
		return findCharForward(text, cursor, findChar)
	case MotionTillChar:
		pos := findCharForward(text, cursor, findChar)
		if pos > cursor {
			return pos - 1
		}
		return cursor
	case MotionFindCharBack:
		return findCharBackward(text, cursor, findChar)
	case MotionTillCharBack:
		pos := findCharBackward(text, cursor, findChar)
		if pos < cursor {
			return pos + 1
		}
		return cursor
	case MotionTop:
		return 0
	case MotionBottom:
		return lastLineStart(text)
	default:
		return cursor
	}
}

// moveLeft moves cursor one character left, not past line start.
func moveLeft(text string, cursor int) int {
	if cursor <= 0 {
		return 0
	}
	// Don't cross newline boundary going left
	if cursor > 0 && text[cursor-1] == '\n' {
		return cursor
	}
	return cursor - 1
}

// moveRight moves cursor one character right, not past line end.
func moveRight(text string, cursor int) int {
	if cursor >= len(text)-1 {
		return cursor
	}
	if text[cursor] == '\n' {
		return cursor
	}
	return cursor + 1
}

// moveDown moves cursor to the same column on the next line.
func moveDown(text string, cursor int) int {
	// Find current line boundaries
	ls := lineStart(text, cursor)
	col := cursor - ls

	// Find next line start
	nextNL := -1
	for i := cursor; i < len(text); i++ {
		if text[i] == '\n' {
			nextNL = i
			break
		}
	}
	if nextNL == -1 {
		return cursor // already on last line
	}

	nextLineStart := nextNL + 1
	if nextLineStart >= len(text) {
		return len(text) - 1
	}

	// Find end of next line
	nextLineEnd := len(text)
	for i := nextLineStart; i < len(text); i++ {
		if text[i] == '\n' {
			nextLineEnd = i
			break
		}
	}

	pos := nextLineStart + col
	if pos >= nextLineEnd {
		pos = nextLineEnd
		if pos > nextLineStart && pos < len(text) && text[pos] == '\n' {
			pos--
		}
	}
	if pos >= len(text) {
		pos = len(text) - 1
	}
	return pos
}

// moveUp moves cursor to the same column on the previous line.
func moveUp(text string, cursor int) int {
	ls := lineStart(text, cursor)
	if ls == 0 {
		return cursor // already on first line
	}

	col := cursor - ls
	// Previous line ends at ls-1 (the newline char)
	prevLineEnd := ls - 1
	prevLineStart := 0
	for i := prevLineEnd - 1; i >= 0; i-- {
		if text[i] == '\n' {
			prevLineStart = i + 1
			break
		}
	}

	pos := prevLineStart + col
	if pos > prevLineEnd {
		pos = prevLineEnd
		if pos > prevLineStart && text[pos] == '\n' {
			pos--
		}
	}
	return pos
}

// lineStart returns the byte offset of the start of the current line.
func lineStart(text string, cursor int) int {
	for i := cursor - 1; i >= 0; i-- {
		if text[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// lineEnd returns the byte offset of the last character on the current line (before newline).
func lineEnd(text string, cursor int) int {
	for i := cursor; i < len(text); i++ {
		if text[i] == '\n' {
			if i > 0 && i > cursor {
				return i - 1
			}
			return cursor
		}
	}
	if len(text) == 0 {
		return 0
	}
	return len(text) - 1
}

// firstNonBlank returns offset of first non-whitespace char on current line.
func firstNonBlank(text string, cursor int) int {
	ls := lineStart(text, cursor)
	for i := ls; i < len(text); i++ {
		if text[i] == '\n' {
			return ls
		}
		if !unicode.IsSpace(rune(text[i])) {
			return i
		}
	}
	return ls
}

// nextWordStart moves to the start of the next word.
// bigWord=true uses whitespace-only word boundaries (WORD).
func nextWordStart(text string, cursor int, bigWord bool) int {
	if cursor >= len(text)-1 {
		return cursor
	}

	i := cursor
	n := len(text)

	if bigWord {
		// Skip non-whitespace
		for i < n && !isWhitespace(text[i]) {
			i++
		}
		// Skip whitespace
		for i < n && isWhitespace(text[i]) {
			i++
		}
	} else {
		ch := rune(text[i])
		if isWordChar(ch) {
			// Skip word chars
			for i < n && isWordChar(rune(text[i])) {
				i++
			}
		} else if isPunct(ch) {
			// Skip punctuation
			for i < n && isPunct(rune(text[i])) {
				i++
			}
		} else {
			i++
		}
		// Skip whitespace to next word
		for i < n && isWhitespace(text[i]) {
			i++
		}
	}

	if i >= n {
		return n - 1
	}
	return i
}

// wordEnd moves to the end of the current or next word.
func wordEnd(text string, cursor int, bigWord bool) int {
	if cursor >= len(text)-1 {
		return cursor
	}

	i := cursor + 1
	n := len(text)

	// Skip whitespace first
	for i < n && isWhitespace(text[i]) {
		i++
	}

	if i >= n {
		return n - 1
	}

	if bigWord {
		// Skip non-whitespace
		for i < n && !isWhitespace(text[i]) {
			i++
		}
	} else {
		ch := rune(text[i])
		if isWordChar(ch) {
			for i < n && isWordChar(rune(text[i])) {
				i++
			}
		} else if isPunct(ch) {
			for i < n && isPunct(rune(text[i])) {
				i++
			}
		}
	}

	if i > cursor+1 {
		i--
	}
	if i >= n {
		return n - 1
	}
	return i
}

// prevWordStart moves to the start of the previous word.
func prevWordStart(text string, cursor int, bigWord bool) int {
	if cursor <= 0 {
		return 0
	}

	i := cursor - 1

	// Skip whitespace backward
	for i > 0 && isWhitespace(text[i]) {
		i--
	}

	if i <= 0 {
		return 0
	}

	if bigWord {
		// Skip non-whitespace backward
		for i > 0 && !isWhitespace(text[i-1]) {
			i--
		}
	} else {
		ch := rune(text[i])
		if isWordChar(ch) {
			for i > 0 && isWordChar(rune(text[i-1])) {
				i--
			}
		} else if isPunct(ch) {
			for i > 0 && isPunct(rune(text[i-1])) {
				i--
			}
		}
	}

	return i
}

// findCharForward finds the next occurrence of ch after cursor on the same line.
func findCharForward(text string, cursor int, ch rune) int {
	for i := cursor + 1; i < len(text); i++ {
		if text[i] == '\n' {
			return cursor // not found on this line
		}
		if rune(text[i]) == ch {
			return i
		}
	}
	return cursor
}

// findCharBackward finds the previous occurrence of ch before cursor on the same line.
func findCharBackward(text string, cursor int, ch rune) int {
	for i := cursor - 1; i >= 0; i-- {
		if text[i] == '\n' {
			return cursor // not found on this line
		}
		if rune(text[i]) == ch {
			return i
		}
	}
	return cursor
}

// lastLineStart returns the byte offset of the start of the last line.
func lastLineStart(text string) int {
	if len(text) == 0 {
		return 0
	}
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '\n' {
			if i == len(text)-1 {
				continue
			}
			return i + 1
		}
	}
	return 0
}

// isWordChar returns true for alphanumeric and underscore.
func isWordChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

// isPunct returns true for punctuation (non-word, non-whitespace).
func isPunct(ch rune) bool {
	return !isWordChar(ch) && !unicode.IsSpace(ch) && ch != '\n'
}

// isWhitespace returns true for space, tab, newline.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

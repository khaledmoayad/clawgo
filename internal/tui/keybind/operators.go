package keybind

import "unicode/utf8"

// Operator represents a vim operator (d, c, y).
type Operator int

const (
	// OpNone means no operator is pending.
	OpNone Operator = iota
	// OpDelete removes text (d operator).
	OpDelete
	// OpChange removes text and enters insert mode (c operator).
	OpChange
	// OpYank copies text to register without modifying (y operator).
	OpYank
)

// OperatorResult holds the outcome of an operator execution.
type OperatorResult struct {
	// NewText is the resulting text after the operator.
	NewText string
	// NewCursor is the new cursor position.
	NewCursor int
	// DeletedText is the text that was removed/yanked (for register).
	DeletedText string
	// EnterInsert is true if the editor should switch to insert mode (c operator).
	EnterInsert bool
	// Linewise indicates the operation was on full lines.
	Linewise bool
}

// Register holds the vim unnamed register content for yank/paste.
type Register struct {
	Content  string
	Linewise bool
}

// ExecuteOperator applies an operator over a text range [start, end).
// The range is exclusive on end: text[start:end] is affected.
func ExecuteOperator(op Operator, text string, start, end int) OperatorResult {
	if start > end {
		start, end = end, start
	}
	// Clamp to bounds
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start == end {
		return OperatorResult{NewText: text, NewCursor: start}
	}

	deleted := text[start:end]

	switch op {
	case OpDelete:
		newText := text[:start] + text[end:]
		newCursor := start
		if newCursor > len(newText) {
			newCursor = len(newText)
		}
		// In normal mode cursor shouldn't go past last char
		if newCursor > 0 && newCursor == len(newText) {
			_, sz := utf8.DecodeLastRuneInString(newText)
			newCursor = len(newText) - sz
		}
		return OperatorResult{
			NewText:     newText,
			NewCursor:   clampCursor(newCursor, newText),
			DeletedText: deleted,
		}

	case OpChange:
		newText := text[:start] + text[end:]
		return OperatorResult{
			NewText:     newText,
			NewCursor:   start,
			DeletedText: deleted,
			EnterInsert: true,
		}

	case OpYank:
		return OperatorResult{
			NewText:     text,
			NewCursor:   start,
			DeletedText: deleted,
		}

	default:
		return OperatorResult{NewText: text, NewCursor: start}
	}
}

// ExecuteLineOp performs a line-wise operation (dd, cc, yy).
// It operates on `count` lines starting from the line containing `cursor`.
func ExecuteLineOp(op Operator, text string, cursor int, count int) OperatorResult {
	lines := splitLines(text)
	curLine := lineAtOffset(text, cursor)
	if curLine >= len(lines) {
		curLine = len(lines) - 1
	}
	if curLine < 0 {
		curLine = 0
	}

	linesToAffect := count
	if curLine+linesToAffect > len(lines) {
		linesToAffect = len(lines) - curLine
	}

	// Calculate byte offsets for the affected line range
	lineStart := lineStartOffset(lines, curLine)
	lineEnd := lineStart
	for i := 0; i < linesToAffect; i++ {
		idx := curLine + i
		if idx < len(lines) {
			lineEnd += len(lines[idx])
			if idx < len(lines)-1 {
				lineEnd++ // newline
			}
		}
	}
	// If not at the last line, include trailing newline
	if curLine+linesToAffect <= len(lines)-1 && lineEnd < len(text) {
		if lineEnd < len(text) && text[lineEnd] == '\n' {
			lineEnd++
		}
	}

	content := text[lineStart:lineEnd]
	// Ensure linewise content ends with newline for paste detection
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content += "\n"
	}

	switch op {
	case OpYank:
		return OperatorResult{
			NewText:     text,
			NewCursor:   lineStart,
			DeletedText: content,
			Linewise:    true,
		}

	case OpDelete:
		deleteStart := lineStart
		deleteEnd := lineEnd
		// If deleting to end and there's a preceding newline, include it
		if deleteEnd >= len(text) && deleteStart > 0 && text[deleteStart-1] == '\n' {
			deleteStart--
		}
		newText := text[:deleteStart] + text[deleteEnd:]
		if newText == "" {
			return OperatorResult{
				NewText:     "",
				NewCursor:   0,
				DeletedText: content,
				Linewise:    true,
			}
		}
		return OperatorResult{
			NewText:     newText,
			NewCursor:   clampCursor(deleteStart, newText),
			DeletedText: content,
			Linewise:    true,
		}

	case OpChange:
		// Replace affected lines with empty line and enter insert
		var before, after []string
		if curLine > 0 {
			before = lines[:curLine]
		}
		if curLine+linesToAffect < len(lines) {
			after = lines[curLine+linesToAffect:]
		}
		newLines := make([]string, 0, len(before)+1+len(after))
		newLines = append(newLines, before...)
		newLines = append(newLines, "")
		newLines = append(newLines, after...)
		newText := joinLines(newLines)
		insertPos := lineStartOffset(newLines, curLine)
		return OperatorResult{
			NewText:     newText,
			NewCursor:   insertPos,
			DeletedText: content,
			EnterInsert: true,
			Linewise:    true,
		}

	default:
		return OperatorResult{NewText: text, NewCursor: cursor}
	}
}

// Paste inserts register content at the given position.
// If after is true, inserts after the cursor position.
// For linewise content, inserts on a new line.
func Paste(reg Register, text string, cursor int, after bool) (string, int) {
	if reg.Content == "" {
		return text, cursor
	}

	if reg.Linewise {
		lines := splitLines(text)
		curLine := lineAtOffset(text, cursor)
		content := reg.Content
		// Remove trailing newline for insertion
		if len(content) > 0 && content[len(content)-1] == '\n' {
			content = content[:len(content)-1]
		}

		insertLine := curLine
		if after {
			insertLine = curLine + 1
		}
		if insertLine > len(lines) {
			insertLine = len(lines)
		}

		contentLines := splitLines(content)
		newLines := make([]string, 0, len(lines)+len(contentLines))
		newLines = append(newLines, lines[:insertLine]...)
		newLines = append(newLines, contentLines...)
		newLines = append(newLines, lines[insertLine:]...)

		newText := joinLines(newLines)
		newCursor := lineStartOffset(newLines, insertLine)
		return newText, newCursor
	}

	// Character-wise paste
	insertPoint := cursor
	if after && cursor < len(text) {
		_, sz := utf8.DecodeRuneInString(text[cursor:])
		insertPoint = cursor + sz
	}
	if insertPoint > len(text) {
		insertPoint = len(text)
	}

	newText := text[:insertPoint] + reg.Content + text[insertPoint:]
	// Cursor goes to last char of pasted text
	newCursor := insertPoint + len(reg.Content) - 1
	if newCursor < insertPoint {
		newCursor = insertPoint
	}
	return newText, clampCursor(newCursor, newText)
}

// clampCursor ensures cursor is within valid bounds for the text.
func clampCursor(pos int, text string) int {
	if pos < 0 {
		return 0
	}
	if len(text) == 0 {
		return 0
	}
	if pos >= len(text) {
		// Put cursor on last rune
		_, sz := utf8.DecodeLastRuneInString(text)
		return len(text) - sz
	}
	return pos
}

// splitLines splits text by newlines, preserving empty trailing elements.
func splitLines(text string) []string {
	if text == "" {
		return []string{""}
	}
	result := []string{}
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			result = append(result, text[start:i])
			start = i + 1
		}
	}
	result = append(result, text[start:])
	return result
}

// joinLines joins lines with newline separators.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

// lineAtOffset returns the 0-based line number for a byte offset.
func lineAtOffset(text string, offset int) int {
	line := 0
	for i := 0; i < offset && i < len(text); i++ {
		if text[i] == '\n' {
			line++
		}
	}
	return line
}

// lineStartOffset returns the byte offset of the start of line `lineIndex`.
func lineStartOffset(lines []string, lineIndex int) int {
	offset := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		offset += len(lines[i]) + 1 // +1 for newline
	}
	return offset
}

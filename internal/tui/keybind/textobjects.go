package keybind

import "unicode"

// TextObject represents a vim text object for operator composition.
type TextObject int

const (
	ObjNone         TextObject = iota
	ObjInnerWord               // iw
	ObjAWord                   // aw
	ObjInnerBigWord            // iW
	ObjABigWord                // aW
	ObjInnerDQuote             // i"
	ObjADQuote                 // a"
	ObjInnerSQuote             // i'
	ObjASQuote                 // a'
	ObjInnerBacktick           // i`
	ObjABacktick               // a`
	ObjInnerParen              // i( or i) or ib
	ObjAParen                  // a( or a) or ab
	ObjInnerBracket            // i[ or i]
	ObjABracket                // a[ or a]
	ObjInnerBrace              // i{ or i} or iB
	ObjABrace                  // a{ or a} or aB
	ObjInnerAngle              // i< or i>
	ObjAAngle                  // a< or a>
)

// TextObjectRange represents the byte range of a resolved text object.
// Start is inclusive, End is exclusive.
type TextObjectRange struct {
	Start int
	End   int
}

// ResolveTextObject finds the text range for a text object at the cursor position.
// Returns nil if the text object cannot be found (e.g., no surrounding delimiters).
func ResolveTextObject(obj TextObject, text string, cursor int) *TextObjectRange {
	switch obj {
	case ObjInnerWord:
		return findWordObject(text, cursor, true, false)
	case ObjAWord:
		return findWordObject(text, cursor, false, false)
	case ObjInnerBigWord:
		return findWordObject(text, cursor, true, true)
	case ObjABigWord:
		return findWordObject(text, cursor, false, true)
	case ObjInnerDQuote:
		return findQuoteObject(text, cursor, '"', true)
	case ObjADQuote:
		return findQuoteObject(text, cursor, '"', false)
	case ObjInnerSQuote:
		return findQuoteObject(text, cursor, '\'', true)
	case ObjASQuote:
		return findQuoteObject(text, cursor, '\'', false)
	case ObjInnerBacktick:
		return findQuoteObject(text, cursor, '`', true)
	case ObjABacktick:
		return findQuoteObject(text, cursor, '`', false)
	case ObjInnerParen:
		return findBracketObject(text, cursor, '(', ')', true)
	case ObjAParen:
		return findBracketObject(text, cursor, '(', ')', false)
	case ObjInnerBracket:
		return findBracketObject(text, cursor, '[', ']', true)
	case ObjABracket:
		return findBracketObject(text, cursor, '[', ']', false)
	case ObjInnerBrace:
		return findBracketObject(text, cursor, '{', '}', true)
	case ObjABrace:
		return findBracketObject(text, cursor, '{', '}', false)
	case ObjInnerAngle:
		return findBracketObject(text, cursor, '<', '>', true)
	case ObjAAngle:
		return findBracketObject(text, cursor, '<', '>', false)
	default:
		return nil
	}
}

// TextObjectFromKeys maps the vim key sequence (scope + type) to a TextObject.
// scope is 'i' (inner) or 'a' (around).
// objType is the object character (w, W, ", ', `, (, ), b, [, ], {, }, B, <, >).
func TextObjectFromKeys(scope byte, objType byte) TextObject {
	inner := scope == 'i'

	switch objType {
	case 'w':
		if inner {
			return ObjInnerWord
		}
		return ObjAWord
	case 'W':
		if inner {
			return ObjInnerBigWord
		}
		return ObjABigWord
	case '"':
		if inner {
			return ObjInnerDQuote
		}
		return ObjADQuote
	case '\'':
		if inner {
			return ObjInnerSQuote
		}
		return ObjASQuote
	case '`':
		if inner {
			return ObjInnerBacktick
		}
		return ObjABacktick
	case '(', ')', 'b':
		if inner {
			return ObjInnerParen
		}
		return ObjAParen
	case '[', ']':
		if inner {
			return ObjInnerBracket
		}
		return ObjABracket
	case '{', '}', 'B':
		if inner {
			return ObjInnerBrace
		}
		return ObjABrace
	case '<', '>':
		if inner {
			return ObjInnerAngle
		}
		return ObjAAngle
	default:
		return ObjNone
	}
}

// findWordObject finds the range of a word object at the cursor.
func findWordObject(text string, cursor int, inner bool, bigWord bool) *TextObjectRange {
	if cursor >= len(text) {
		return nil
	}

	ch := rune(text[cursor])
	start := cursor
	end := cursor

	isWs := unicode.IsSpace(ch)
	var isInClass func(rune) bool

	if bigWord {
		// WORD: anything non-whitespace is a word
		isInClass = func(r rune) bool { return !unicode.IsSpace(r) }
	} else {
		if isWordChar(ch) {
			isInClass = isWordChar
		} else if isPunct(ch) {
			isInClass = isPunct
		} else {
			// On whitespace
			isInClass = unicode.IsSpace
		}
	}

	if isWs && !bigWord {
		// On whitespace: select the whitespace run
		for start > 0 && unicode.IsSpace(rune(text[start-1])) && text[start-1] != '\n' {
			start--
		}
		end++
		for end < len(text) && unicode.IsSpace(rune(text[end])) && text[end] != '\n' {
			end++
		}
		return &TextObjectRange{Start: start, End: end}
	}

	// Expand backward within the class
	for start > 0 && isInClass(rune(text[start-1])) {
		start--
	}
	// Expand forward within the class
	end++
	for end < len(text) && isInClass(rune(text[end])) {
		end++
	}

	if !inner {
		// "a word" includes surrounding whitespace
		if end < len(text) && unicode.IsSpace(rune(text[end])) {
			for end < len(text) && unicode.IsSpace(rune(text[end])) && text[end] != '\n' {
				end++
			}
		} else if start > 0 && unicode.IsSpace(rune(text[start-1])) {
			for start > 0 && unicode.IsSpace(rune(text[start-1])) && text[start-1] != '\n' {
				start--
			}
		}
	}

	return &TextObjectRange{Start: start, End: end}
}

// findQuoteObject finds the range of a quote text object.
// Searches on the current line only, pairing quotes left-to-right.
func findQuoteObject(text string, cursor int, quote byte, inner bool) *TextObjectRange {
	// Find current line boundaries
	ls := 0
	for i := cursor - 1; i >= 0; i-- {
		if text[i] == '\n' {
			ls = i + 1
			break
		}
	}
	le := len(text)
	for i := cursor; i < len(text); i++ {
		if text[i] == '\n' {
			le = i
			break
		}
	}

	line := text[ls:le]
	posInLine := cursor - ls

	// Collect positions of the quote character on this line
	positions := []int{}
	for i := 0; i < len(line); i++ {
		if line[i] == quote {
			positions = append(positions, i)
		}
	}

	// Pair quotes: 0-1, 2-3, 4-5, ...
	for i := 0; i+1 < len(positions); i += 2 {
		qs := positions[i]
		qe := positions[i+1]
		if qs <= posInLine && posInLine <= qe {
			if inner {
				return &TextObjectRange{Start: ls + qs + 1, End: ls + qe}
			}
			return &TextObjectRange{Start: ls + qs, End: ls + qe + 1}
		}
	}

	return nil
}

// findBracketObject finds the range of a bracket text object using balanced matching.
func findBracketObject(text string, cursor int, open, close byte, inner bool) *TextObjectRange {
	// Search backward for matching open bracket
	depth := 0
	start := -1
	for i := cursor; i >= 0; i-- {
		if text[i] == close && i != cursor {
			depth++
		} else if text[i] == open {
			if depth == 0 {
				start = i
				break
			}
			depth--
		}
	}
	if start == -1 {
		return nil
	}

	// Search forward for matching close bracket
	depth = 0
	end := -1
	for i := start + 1; i < len(text); i++ {
		if text[i] == open {
			depth++
		} else if text[i] == close {
			if depth == 0 {
				end = i
				break
			}
			depth--
		}
	}
	if end == -1 {
		return nil
	}

	if inner {
		return &TextObjectRange{Start: start + 1, End: end}
	}
	return &TextObjectRange{Start: start, End: end + 1}
}

package render

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// HighlightCode applies syntax highlighting to code for terminal display.
// Uses Chroma with the monokai style and terminal256 formatter.
// If language is empty or unknown, the input code is returned as-is.
func HighlightCode(code, language string) (string, error) {
	if language == "" {
		return code, nil
	}

	lexer := lexers.Get(language)
	if lexer == nil {
		// Unknown language -- return code unchanged.
		return code, nil
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code, nil
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return code, nil
	}

	return buf.String(), nil
}

// HighlightCodeDefault is a convenience wrapper that swallows errors.
func HighlightCodeDefault(code, language string) string {
	result, err := HighlightCode(code, language)
	if err != nil {
		return code
	}
	return result
}

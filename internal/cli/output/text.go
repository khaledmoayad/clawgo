package output

import (
	"fmt"
	"io"
)

// TextWriter writes plain text output to an io.Writer.
// Used by non-interactive text mode (default -p behavior).
type TextWriter struct {
	w io.Writer
}

// NewTextWriter creates a TextWriter that writes to w.
func NewTextWriter(w io.Writer) *TextWriter {
	return &TextWriter{w: w}
}

// WriteText writes text directly to the underlying writer.
func (t *TextWriter) WriteText(text string) error {
	_, err := fmt.Fprint(t.w, text)
	return err
}

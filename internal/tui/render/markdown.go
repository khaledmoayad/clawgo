// Package render provides markdown rendering, syntax highlighting, and
// terminal color profile utilities for the ClawGo TUI.
package render

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

const defaultMarkdownWidth = 80

// RenderMarkdown converts markdown content to ANSI-styled terminal text using
// Glamour. Width controls word wrapping; if <= 0 it defaults to 80.
func RenderMarkdown(content string, width int) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", nil
	}

	if width <= 0 {
		width = defaultMarkdownWidth
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}

	result, err := renderer.Render(content)
	if err != nil {
		return "", err
	}

	// Trim trailing newlines to prevent excess blank lines in TUI output.
	result = strings.TrimRight(result, "\n")
	return result, nil
}

// RenderMarkdownDefault is a convenience wrapper that calls RenderMarkdown
// with width=80 and swallows errors (returns raw content on error).
func RenderMarkdownDefault(content string) string {
	result, err := RenderMarkdown(content, defaultMarkdownWidth)
	if err != nil {
		return content
	}
	return result
}

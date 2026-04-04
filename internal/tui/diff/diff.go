// Package diff provides unified diff parsing, color-coded rendering, and
// scrollable viewport support for the ClawGo TUI.
package diff

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Color palette for diff rendering, consistent with tui/styles.go.
var (
	addColor    = lipgloss.Color("#98C379") // Green for additions
	removeColor = lipgloss.Color("#E06C75") // Red for deletions
	hunkColor   = lipgloss.Color("#61AFEF") // Cyan for hunk headers
	dimColor    = lipgloss.Color("#5C6370") // Gray for dim text

	addStyle    = lipgloss.NewStyle().Foreground(addColor)
	removeStyle = lipgloss.NewStyle().Foreground(removeColor)
	hunkStyle   = lipgloss.NewStyle().Foreground(hunkColor).Faint(true)
	headerStyle = lipgloss.NewStyle().Bold(true)
)

// DiffLine represents a single line in a parsed diff.
type DiffLine struct {
	Type    string // "add", "remove", "context", "header", "hunk"
	Content string // The raw line content
}

// DiffResult is the output of parsing a unified diff.
type DiffResult struct {
	Lines    []DiffLine
	IsDiff   bool
	FileName string
}

// ParseUnifiedDiff parses unified diff text into typed lines.
// It detects whether the input is a valid diff by checking for "---", "+++",
// and "@@" markers. If the text is not a diff, IsDiff is false and Lines is nil.
func ParseUnifiedDiff(text string) DiffResult {
	if !IsDiffContent(text) {
		return DiffResult{IsDiff: false}
	}

	rawLines := strings.Split(text, "\n")
	result := DiffResult{
		IsDiff: true,
		Lines:  make([]DiffLine, 0, len(rawLines)),
	}

	for _, line := range rawLines {
		dl := classifyLine(line)
		result.Lines = append(result.Lines, dl)

		// Extract filename from "+++ b/..." header
		if dl.Type == "header" && strings.HasPrefix(line, "+++ ") {
			name := strings.TrimPrefix(line, "+++ ")
			name = strings.TrimPrefix(name, "b/")
			result.FileName = name
		}
	}

	return result
}

// classifyLine determines the type of a single diff line.
func classifyLine(line string) DiffLine {
	switch {
	case strings.HasPrefix(line, "---"):
		return DiffLine{Type: "header", Content: line}
	case strings.HasPrefix(line, "+++"):
		return DiffLine{Type: "header", Content: line}
	case strings.HasPrefix(line, "@@"):
		return DiffLine{Type: "hunk", Content: line}
	case strings.HasPrefix(line, "+"):
		return DiffLine{Type: "add", Content: line}
	case strings.HasPrefix(line, "-"):
		return DiffLine{Type: "remove", Content: line}
	default:
		return DiffLine{Type: "context", Content: line}
	}
}

// RenderDiff parses and renders a unified diff with color-coded lines.
// If the text is not a valid diff, it is returned unchanged.
// When width > 0, lines are truncated to the given visual width.
func RenderDiff(text string, width int) string {
	parsed := ParseUnifiedDiff(text)
	if !parsed.IsDiff {
		return text
	}

	var sb strings.Builder
	for i, line := range parsed.Lines {
		if i > 0 {
			sb.WriteString("\n")
		}

		content := line.Content
		if width > 0 && len(content) > width {
			content = content[:width]
		}

		switch line.Type {
		case "add":
			sb.WriteString(addStyle.Render(content))
		case "remove":
			sb.WriteString(removeStyle.Render(content))
		case "hunk":
			sb.WriteString(hunkStyle.Render(content))
		case "header":
			sb.WriteString(headerStyle.Render(content))
		default:
			sb.WriteString(content)
		}
	}

	return sb.String()
}

// IsDiffContent performs a quick check to determine if text looks like a
// unified diff. It checks for the presence of "---", "+++", and "@@" markers.
func IsDiffContent(text string) bool {
	return strings.Contains(text, "--- ") &&
		strings.Contains(text, "+++ ") &&
		strings.Contains(text, "@@")
}

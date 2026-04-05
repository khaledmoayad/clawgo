package renderers

import (
	"fmt"
	"os"
	"strings"
)

// supportsOSC8 checks if the terminal supports OSC 8 hyperlinks by examining
// terminal emulator environment variables. iTerm2, WezTerm, Hyper, kitty, and
// most modern emulators support the escape sequence.
func supportsOSC8() bool {
	// Check TERM_PROGRAM which is set by most modern terminal emulators
	termProgram := os.Getenv("TERM_PROGRAM")
	switch strings.ToLower(termProgram) {
	case "iterm.app", "iterm2", "wezterm", "hyper", "kitty", "alacritty":
		return true
	}
	// WezTerm also sets TERM_PROGRAM_VERSION
	if os.Getenv("WEZTERM_EXECUTABLE") != "" {
		return true
	}
	// KONSOLE_VERSION indicates KDE Konsole (supports OSC 8)
	if os.Getenv("KONSOLE_VERSION") != "" {
		return true
	}
	// VTE_VERSION indicates a VTE-based terminal (GNOME Terminal, etc.)
	if os.Getenv("VTE_VERSION") != "" {
		return true
	}
	return false
}

// RenderImage renders image attachment messages. Shows filename with a terminal
// hyperlink (OSC 8 escape sequence) when the terminal supports it. Matches
// Claude Code's UserImageMessage.tsx which uses Ink's Link component for
// clickable file:// URLs.
func RenderImage(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	// Extract image metadata
	imageID := msg.Metadata["image_id"]
	filePath := msg.Metadata["file_path"]
	fileName := msg.Metadata["file_name"]
	width := msg.Metadata["width"]
	height := msg.Metadata["height"]

	// Build the label: "[Image #N]" or "[Image]" matching TS behavior
	label := "[Image]"
	if imageID != "" {
		label = fmt.Sprintf("[Image #%s]", imageID)
	}

	// Render with or without hyperlink
	if filePath != "" && supportsOSC8() {
		// OSC 8 hyperlink: \x1b]8;;URI\x07LABEL\x1b]8;;\x07
		fileURL := "file://" + filePath
		sb.WriteString(fmt.Sprintf("\x1b]8;;%s\x07%s\x1b]8;;\x07",
			fileURL,
			toolStyle.Render("\U0001F5BC "+label),
		))
	} else {
		// Plain text fallback
		if filePath != "" {
			sb.WriteString(toolStyle.Render(fmt.Sprintf("\U0001F5BC %s (%s)", label, filePath)))
		} else if fileName != "" {
			sb.WriteString(toolStyle.Render(fmt.Sprintf("\U0001F5BC %s (%s)", label, fileName)))
		} else {
			sb.WriteString(toolStyle.Render("\U0001F5BC "+label))
		}
	}

	// Show dimensions if available
	if width != "" && height != "" {
		sb.WriteString(dimStyle.Render(fmt.Sprintf(" %sx%s", width, height)))
	}

	return sb.String()
}

// RenderUserImage is the registry-compatible wrapper for rendering user image
// messages. It delegates to RenderImage.
func RenderUserImage(msg DisplayMessage, width int) string {
	return RenderImage(msg, width)
}

package render

import (
	"os"

	"charm.land/lipgloss/v2"
)

// ColorProfile represents the terminal color capability level.
type ColorProfile int

const (
	// TrueColor supports 24-bit RGB colors (16.7M colors).
	TrueColor ColorProfile = iota
	// ANSI256 supports 256 indexed colors.
	ANSI256
	// ANSI supports the basic 16 ANSI colors.
	ANSI
	// Ascii means no color support (monochrome).
	Ascii
)

// DetectProfile detects the terminal's color profile from environment variables.
// Checks NO_COLOR, COLORTERM, and TERM to determine capability level.
func DetectProfile() ColorProfile {
	// NO_COLOR convention: https://no-color.org/
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return Ascii
	}

	colorterm := os.Getenv("COLORTERM")
	if colorterm == "truecolor" || colorterm == "24bit" {
		return TrueColor
	}

	term := os.Getenv("TERM")
	if containsSubstring(term, "256color") {
		return ANSI256
	}

	return ANSI
}

// AdaptStyle adjusts a lipgloss style to match the given color profile.
// TrueColor and ANSI256 return the style unchanged.
// ANSI returns the style unchanged (lipgloss handles 16-color downsampling).
// Ascii strips all foreground and background colors.
func AdaptStyle(s lipgloss.Style, profile ColorProfile) lipgloss.Style {
	switch profile {
	case Ascii:
		return s.UnsetForeground().UnsetBackground()
	default:
		return s
	}
}

// MaxColors returns the maximum number of colors for a profile.
func MaxColors(profile ColorProfile) int {
	switch profile {
	case TrueColor:
		return 16777216
	case ANSI256:
		return 256
	case ANSI:
		return 16
	case Ascii:
		return 0
	default:
		return 0
	}
}

// containsSubstring checks whether s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

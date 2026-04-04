package render

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestDetectProfile_ReturnsValidProfile(t *testing.T) {
	profile := DetectProfile()
	// Should return one of the valid profiles
	if profile < TrueColor || profile > Ascii {
		t.Errorf("DetectProfile returned invalid profile: %d", profile)
	}
}

func TestAdaptStyle_TrueColor(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	adapted := AdaptStyle(style, TrueColor)
	// TrueColor should return style unchanged
	fg := adapted.GetForeground()
	if fg != lipgloss.Color("#FF0000") {
		t.Errorf("TrueColor should not change style, got foreground: %v", fg)
	}
}

func TestAdaptStyle_ANSI256(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	adapted := AdaptStyle(style, ANSI256)
	// ANSI256 should return style unchanged
	fg := adapted.GetForeground()
	if fg != lipgloss.Color("#FF0000") {
		t.Errorf("ANSI256 should not change style, got foreground: %v", fg)
	}
}

func TestAdaptStyle_Ascii(t *testing.T) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000")).
		Background(lipgloss.Color("#00FF00"))
	adapted := AdaptStyle(style, Ascii)
	// Ascii should strip all colors -- GetForeground/Background return NoColor{}
	fg := adapted.GetForeground()
	bg := adapted.GetBackground()
	if fg != (lipgloss.NoColor{}) {
		t.Errorf("Ascii should unset foreground, got: %v", fg)
	}
	if bg != (lipgloss.NoColor{}) {
		t.Errorf("Ascii should unset background, got: %v", bg)
	}
}

func TestMaxColors_TrueColor(t *testing.T) {
	if MaxColors(TrueColor) != 16777216 {
		t.Errorf("TrueColor should have 16777216 colors, got %d", MaxColors(TrueColor))
	}
}

func TestMaxColors_ANSI256(t *testing.T) {
	if MaxColors(ANSI256) != 256 {
		t.Errorf("ANSI256 should have 256 colors, got %d", MaxColors(ANSI256))
	}
}

func TestMaxColors_ANSI(t *testing.T) {
	if MaxColors(ANSI) != 16 {
		t.Errorf("ANSI should have 16 colors, got %d", MaxColors(ANSI))
	}
}

func TestMaxColors_Ascii(t *testing.T) {
	if MaxColors(Ascii) != 0 {
		t.Errorf("Ascii should have 0 colors, got %d", MaxColors(Ascii))
	}
}

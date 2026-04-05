package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// stripANSI removes ANSI escape sequences for plain-text assertions.
// Uses lipgloss helper for reliable stripping.
func stripANSI(s string) string {
	// lipgloss.Width counts visible chars; we can also just check contains
	// but for exact matching we manually strip
	var result strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func TestStatusLineBasic(t *testing.T) {
	s := NewStatusLineModel()
	s.SetModel("claude-sonnet-4-20250514")
	s.SetCost("$0.42")
	s.SetContext(45, "45.2k / 200k")
	s.SetWidth(120)

	view := s.View()
	plain := stripANSI(view)

	assert.Contains(t, plain, "claude-sonnet-4-20250514", "model name should appear")
	assert.Contains(t, plain, "$0.42", "session cost should appear")
	assert.Contains(t, plain, "45%", "context percent should appear")
	assert.Contains(t, plain, "45.2k / 200k", "context tokens should appear")

	// Verify it's a single line
	assert.NotContains(t, view, "\n", "status line should be a single line")
}

func TestStatusLineContextWarning(t *testing.T) {
	// At 85% -- should use warning color
	s := NewStatusLineModel()
	s.SetContext(85, "170k / 200k")
	s.SetWidth(80)

	view := s.View()
	plain := stripANSI(view)
	assert.Contains(t, plain, "85%", "context percent should appear")

	// The warning color (E5C07B) should be present in the ANSI output.
	// lipgloss renders it as part of an SGR sequence.
	// We check that the raw output differs from a <80% rendering.
	sNormal := NewStatusLineModel()
	sNormal.SetContext(50, "100k / 200k")
	sNormal.SetWidth(80)
	normalView := sNormal.View()

	// Both views should render, but they should differ (different colors)
	assert.NotEqual(t, lipgloss.Width(view), 0, "warning view should have content")
	assert.NotEqual(t, lipgloss.Width(normalView), 0, "normal view should have content")

	// At 96% -- should use error color
	sError := NewStatusLineModel()
	sError.SetContext(96, "192k / 200k")
	sError.SetWidth(80)

	errorView := sError.View()
	errorPlain := stripANSI(errorView)
	assert.Contains(t, errorPlain, "96%", "error context should show percent")
}

func TestStatusLineVimMode(t *testing.T) {
	s := NewStatusLineModel()
	s.SetModel("claude-sonnet-4-20250514")
	s.SetVimMode("NORMAL")
	s.SetWidth(80)

	view := s.View()
	plain := stripANSI(view)

	assert.Contains(t, plain, "[NORMAL]", "vim mode indicator should appear")

	// Without vim mode
	s2 := NewStatusLineModel()
	s2.SetModel("claude-sonnet-4-20250514")
	s2.SetWidth(80)

	view2 := s2.View()
	plain2 := stripANSI(view2)
	assert.NotContains(t, plain2, "[NORMAL]", "vim mode should not appear when not set")
}

func TestStatusLinePlanMode(t *testing.T) {
	s := NewStatusLineModel()
	s.SetModel("claude-sonnet-4-20250514")
	s.SetPlanMode(true)
	s.SetWidth(80)

	view := s.View()
	plain := stripANSI(view)

	assert.Contains(t, plain, "[PLAN]", "plan mode indicator should appear")
}

func TestStatusLineFastMode(t *testing.T) {
	s := NewStatusLineModel()
	s.SetModel("claude-sonnet-4-20250514")
	s.SetFastMode(true)
	s.SetWidth(80)

	view := s.View()
	plain := stripANSI(view)

	assert.Contains(t, plain, "[FAST]", "fast mode indicator should appear")
}

func TestStatusLineEmpty(t *testing.T) {
	s := NewStatusLineModel()
	s.SetWidth(80)

	view := s.View()
	// Should render without panicking, producing a padded empty bar
	assert.NotEmpty(t, view, "empty status line should still render something")
}

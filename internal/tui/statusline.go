package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Status line color palette -- matches Claude Code visual style.
var (
	statusBgColor      = lipgloss.Color("#1E1E2E") // Dark background for the bar
	statusFgColor      = lipgloss.Color("#5C6370") // Dim foreground (default)
	statusWarningColor = lipgloss.Color("#E5C07B") // Warning amber (context > 80%)
	statusErrorColor   = lipgloss.Color("#E06C75") // Error red (context > 95%)
	statusAccentColor  = lipgloss.Color("#61AFEF") // Blue accent for vim mode
	statusPlanColor    = lipgloss.Color("#C678DD") // Purple for plan mode
	statusFastColor    = lipgloss.Color("#98C379") // Green for fast mode
)

// StatusLineModel renders a persistent status bar at the bottom of the TUI.
// It displays model name, session cost, context usage, and vim mode indicator.
type StatusLineModel struct {
	modelName      string // current model (e.g. "claude-sonnet-4-20250514")
	sessionCost    string // formatted cost (e.g. "$0.42")
	contextPercent int    // 0-100 context window usage
	contextTokens  string // formatted token count (e.g. "45.2k / 200k")
	vimMode        string // "", "NORMAL", "INSERT", "VISUAL"
	width          int    // terminal width for layout
	planMode       bool   // whether in plan mode
	fastMode       bool   // whether fast mode is active
}

// NewStatusLineModel creates a status line with empty fields.
func NewStatusLineModel() StatusLineModel {
	return StatusLineModel{}
}

// SetModel sets the current model name.
func (s *StatusLineModel) SetModel(name string) {
	s.modelName = name
}

// SetCost sets the formatted session cost string.
func (s *StatusLineModel) SetCost(cost string) {
	s.sessionCost = cost
}

// SetContext sets the context window usage percentage and token display string.
func (s *StatusLineModel) SetContext(percent int, tokens string) {
	s.contextPercent = percent
	s.contextTokens = tokens
}

// SetVimMode sets the vim mode indicator (empty string to hide).
func (s *StatusLineModel) SetVimMode(mode string) {
	s.vimMode = mode
}

// SetWidth sets the terminal width used for layout.
func (s *StatusLineModel) SetWidth(w int) {
	s.width = w
}

// SetPlanMode sets the plan mode indicator.
func (s *StatusLineModel) SetPlanMode(b bool) {
	s.planMode = b
}

// SetFastMode sets the fast mode indicator.
func (s *StatusLineModel) SetFastMode(b bool) {
	s.fastMode = b
}

// View renders the status line as a single terminal-width bar.
//
// Layout:
//
//	Left:  model-name [PLAN] [FAST]
//	Right: context% tokens | $cost [VIM-MODE]
//
// Context percentage is colored: dim < 80%, warning 80-95%, error > 95%.
func (s StatusLineModel) View() string {
	w := s.width
	if w <= 0 {
		w = 80
	}

	barStyle := lipgloss.NewStyle().
		Background(statusBgColor)

	dimStyle := lipgloss.NewStyle().
		Foreground(statusFgColor).
		Background(statusBgColor)

	// -- Left side --
	var left strings.Builder
	if s.modelName != "" {
		left.WriteString(dimStyle.Render(s.modelName))
	}
	if s.planMode {
		left.WriteString(" ")
		left.WriteString(lipgloss.NewStyle().
			Foreground(statusPlanColor).
			Background(statusBgColor).
			Bold(true).
			Render("[PLAN]"))
	}
	if s.fastMode {
		left.WriteString(" ")
		left.WriteString(lipgloss.NewStyle().
			Foreground(statusFastColor).
			Background(statusBgColor).
			Bold(true).
			Render("[FAST]"))
	}

	// -- Right side --
	var right strings.Builder

	// Context usage with color thresholds
	if s.contextTokens != "" || s.contextPercent > 0 {
		contextStyle := dimStyle
		if s.contextPercent > 95 {
			contextStyle = lipgloss.NewStyle().
				Foreground(statusErrorColor).
				Background(statusBgColor).
				Bold(true)
		} else if s.contextPercent > 80 {
			contextStyle = lipgloss.NewStyle().
				Foreground(statusWarningColor).
				Background(statusBgColor)
		}

		contextStr := fmt.Sprintf("%d%%", s.contextPercent)
		if s.contextTokens != "" {
			contextStr = fmt.Sprintf("%d%% %s", s.contextPercent, s.contextTokens)
		}
		right.WriteString(contextStyle.Render(contextStr))
	}

	// Cost
	if s.sessionCost != "" {
		if right.Len() > 0 {
			right.WriteString(dimStyle.Render(" | "))
		}
		right.WriteString(dimStyle.Render(s.sessionCost))
	}

	// Vim mode indicator
	if s.vimMode != "" {
		right.WriteString(" ")
		right.WriteString(lipgloss.NewStyle().
			Foreground(statusAccentColor).
			Background(statusBgColor).
			Bold(true).
			Render("["+s.vimMode+"]"))
	}

	// Compose: left + padding + right
	leftStr := left.String()
	rightStr := right.String()

	// Calculate visible widths (without ANSI escape sequences)
	leftVisible := lipgloss.Width(leftStr)
	rightVisible := lipgloss.Width(rightStr)

	padding := w - leftVisible - rightVisible
	if padding < 1 {
		padding = 1
	}

	padStr := barStyle.Render(strings.Repeat(" ", padding))

	return leftStr + padStr + rightStr
}

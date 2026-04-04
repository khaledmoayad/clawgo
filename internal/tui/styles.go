package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/khaledmoayad/clawgo/internal/tui/render"
)

// Color palette matching Claude Code visual style.
var (
	// Primary colors
	UserColor      = lipgloss.Color("#6B9BD2") // Blue for user messages
	AssistantColor = lipgloss.Color("#E8B86D") // Gold/orange for assistant
	ErrorColor     = lipgloss.Color("#E06C75") // Red for errors
	SuccessColor   = lipgloss.Color("#98C379") // Green for success
	DimColor       = lipgloss.Color("#5C6370") // Gray for secondary text
	ThinkingColor  = lipgloss.Color("#C678DD") // Purple for thinking blocks

	// Styles
	UserPromptStyle = lipgloss.NewStyle().Foreground(UserColor).Bold(true)
	AssistantStyle  = lipgloss.NewStyle().Foreground(AssistantColor).Bold(true)
	ErrorStyle      = lipgloss.NewStyle().Foreground(ErrorColor)
	DimStyle        = lipgloss.NewStyle().Foreground(DimColor)
	ThinkingStyle   = lipgloss.NewStyle().Foreground(ThinkingColor).Italic(true)
	ToolNameStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")).Bold(true)
	CostStyle       = lipgloss.NewStyle().Foreground(DimColor)
	PermissionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true)

	// Layout
	MessagePadding = lipgloss.NewStyle().PaddingLeft(2)
	SeparatorStyle = lipgloss.NewStyle().Foreground(DimColor)
)

// InitStyles adapts all package-level styles to the given terminal color
// profile. Call once at startup.
func InitStyles(profile render.ColorProfile) {
	UserPromptStyle = render.AdaptStyle(UserPromptStyle, profile)
	AssistantStyle = render.AdaptStyle(AssistantStyle, profile)
	ErrorStyle = render.AdaptStyle(ErrorStyle, profile)
	DimStyle = render.AdaptStyle(DimStyle, profile)
	ThinkingStyle = render.AdaptStyle(ThinkingStyle, profile)
	ToolNameStyle = render.AdaptStyle(ToolNameStyle, profile)
	CostStyle = render.AdaptStyle(CostStyle, profile)
	PermissionStyle = render.AdaptStyle(PermissionStyle, profile)
	SeparatorStyle = render.AdaptStyle(SeparatorStyle, profile)
}

// RoleLabel returns the styled label for a message role.
func RoleLabel(role string) string {
	switch role {
	case "user":
		return UserPromptStyle.Render("You")
	case "assistant":
		return AssistantStyle.Render("Claude")
	default:
		return DimStyle.Render(role)
	}
}

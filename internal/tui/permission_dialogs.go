package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// SpecializedPermissionModel extends PermissionModel with tool-specific rendering.
// It replaces the basic PermissionModel with a registry-based approach where each
// tool type has its own rendering logic, matching the TypeScript PermissionRequest
// component that dispatches to BashPermissionRequest, FileWritePermissionRequest, etc.
type SpecializedPermissionModel struct {
	PermissionModel
	details PermissionRequestDetails
}

// NewSpecializedPermissionModel creates a new specialized permission model.
func NewSpecializedPermissionModel() SpecializedPermissionModel {
	return SpecializedPermissionModel{
		PermissionModel: NewPermissionModel(),
	}
}

// ShowDetailed displays a permission dialog with full tool-specific details.
func (m *SpecializedPermissionModel) ShowDetailed(details PermissionRequestDetails) {
	m.details = details
	m.PermissionModel.Show(details.ToolName, details.ToolInput, details.Description)
}

// Details returns the current permission request details.
func (m SpecializedPermissionModel) Details() PermissionRequestDetails {
	return m.details
}

// View renders the specialized permission dialog based on the tool type.
func (m SpecializedPermissionModel) View() string {
	if !m.IsActive() {
		return ""
	}

	switch m.details.DialogType {
	case PermDialogBash:
		return m.renderBashDialog()
	case PermDialogFileWrite:
		return m.renderFileWriteDialog()
	case PermDialogFileEdit:
		return m.renderFileEditDialog()
	case PermDialogFilesystem:
		return m.renderFilesystemDialog()
	case PermDialogWebFetch:
		return m.renderWebFetchDialog()
	case PermDialogPlanMode:
		return m.renderPlanModeDialog()
	case PermDialogSandbox:
		return m.renderSandboxDialog()
	case PermDialogMCP:
		return m.renderMCPDialog()
	default:
		return m.renderFallbackDialog()
	}
}

// --- Bash Permission Dialog ---

func (m SpecializedPermissionModel) renderBashDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#E5C07B")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	// Title with tool name
	inner.WriteString(PermissionStyle.Render("Bash"))
	inner.WriteString("\n")

	// Working directory
	if m.details.WorkingDir != "" {
		inner.WriteString(DimStyle.Render("  in " + m.details.WorkingDir))
		inner.WriteString("\n")
	}
	inner.WriteString("\n")

	// Command display (the main content)
	cmd := m.details.Command
	if cmd == "" {
		cmd = m.details.ToolInput
	}
	if cmd != "" {
		// Truncate very long commands
		maxLen := bw * 6
		if len(cmd) > maxLen {
			cmd = cmd[:maxLen] + "..."
		}
		cmdStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ABB2BF")).
			PaddingLeft(2)
		inner.WriteString(cmdStyle.Render(cmd))
		inner.WriteString("\n")
	}

	// Sandbox indicator
	if m.details.IsSandbox {
		sandboxStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379")).
			PaddingLeft(2)
		inner.WriteString("\n")
		inner.WriteString(sandboxStyle.Render("(sandboxed)"))
		inner.WriteString("\n")
	}

	// Matched rule indicator
	if m.details.MatchedRule != "" {
		inner.WriteString("\n")
		inner.WriteString(DimStyle.Render("  Rule: " + m.details.MatchedRule))
		inner.WriteString("\n")
	}

	// Separator
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")

	// Options
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- File Write Permission Dialog ---

func (m SpecializedPermissionModel) renderFileWriteDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#61AFEF")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(ToolNameStyle.Render("Write file"))
	inner.WriteString("\n\n")

	// File path
	if m.details.FilePath != "" {
		pathStyle := lipgloss.NewStyle().Bold(true).PaddingLeft(2)
		inner.WriteString(pathStyle.Render(m.details.FilePath))
		inner.WriteString("\n")
	}

	// Content preview (first few lines)
	if m.details.NewContent != "" {
		inner.WriteString("\n")
		preview := truncateContent(m.details.NewContent, 10, bw-6)
		previewStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379")).
			PaddingLeft(2)
		inner.WriteString(previewStyle.Render(preview))
		inner.WriteString("\n")
	}

	// Separator and options
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- File Edit Permission Dialog ---

func (m SpecializedPermissionModel) renderFileEditDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#C678DD")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render("Edit file"))
	inner.WriteString("\n\n")

	// File path
	if m.details.FilePath != "" {
		pathStyle := lipgloss.NewStyle().Bold(true).PaddingLeft(2)
		inner.WriteString(pathStyle.Render(m.details.FilePath))
		inner.WriteString("\n")
	}

	// Diff preview
	if m.details.DiffPreview != "" {
		inner.WriteString("\n")
		preview := truncateContent(m.details.DiffPreview, 15, bw-6)
		inner.WriteString(MessagePadding.Render(preview))
		inner.WriteString("\n")
	}

	// Separator and options
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- Filesystem Permission Dialog (Read/Glob/Grep) ---

func (m SpecializedPermissionModel) renderFilesystemDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#56B6C2")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(m.details.ToolName))
	inner.WriteString("\n\n")

	// Show description/input
	if m.details.Description != "" {
		inner.WriteString(MessagePadding.Render(m.details.Description))
		inner.WriteString("\n")
	}
	if m.details.FilePath != "" {
		inner.WriteString(MessagePadding.Render(DimStyle.Render(m.details.FilePath)))
		inner.WriteString("\n")
	}

	// Separator and options
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- Web Fetch Permission Dialog ---

func (m SpecializedPermissionModel) renderWebFetchDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#61AFEF")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(ToolNameStyle.Render("Web Fetch"))
	inner.WriteString("\n\n")

	// URL
	url := m.details.URL
	if url == "" {
		url = m.details.ToolInput
	}
	if url != "" {
		urlStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Underline(true).
			PaddingLeft(2)
		inner.WriteString(urlStyle.Render(url))
		inner.WriteString("\n")
	}

	// Separator and options
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- Plan Mode Permission Dialog ---

func (m SpecializedPermissionModel) renderPlanModeDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#E5C07B")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(PermissionStyle.Render("Plan Mode"))
	inner.WriteString("\n\n")

	if m.details.PlanDescription != "" {
		inner.WriteString(MessagePadding.Render(m.details.PlanDescription))
		inner.WriteString("\n")
	} else if m.details.Description != "" {
		inner.WriteString(MessagePadding.Render(m.details.Description))
		inner.WriteString("\n")
	}

	// Separator and options
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- Sandbox Permission Dialog ---

func (m SpecializedPermissionModel) renderSandboxDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#E06C75")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render("Sandbox Network Access"))
	inner.WriteString("\n\n")

	if m.details.Description != "" {
		inner.WriteString(MessagePadding.Render(m.details.Description))
		inner.WriteString("\n")
	}

	// Separator and options (sandbox has yes/yes-dont-ask-again/no)
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(DimStyle.Render("  [y] Allow  [n] Deny  [a] Allow and don't ask again"))

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- MCP Permission Dialog ---

func (m SpecializedPermissionModel) renderMCPDialog() string {
	bw := m.boxWidth()

	titleColor := lipgloss.Color("#56B6C2")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(titleColor).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	title := m.details.ToolName
	if m.details.ServerName != "" {
		title = fmt.Sprintf("%s (via %s)", m.details.ToolName, m.details.ServerName)
	}
	inner.WriteString(lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(title))
	inner.WriteString("\n\n")

	if m.details.Description != "" {
		inner.WriteString(MessagePadding.Render(m.details.Description))
		inner.WriteString("\n")
	}

	// Show truncated input for MCP tools
	if m.details.ToolInput != "" {
		display := m.details.ToolInput
		if len(display) > 200 {
			display = display[:200] + "..."
		}
		inner.WriteString(MessagePadding.Render(DimStyle.Render(display)))
		inner.WriteString("\n")
	}

	// Separator and options
	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(m.renderOptions())

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// --- Fallback Permission Dialog ---

func (m SpecializedPermissionModel) renderFallbackDialog() string {
	// The fallback uses the base PermissionModel's View method
	return m.PermissionModel.View()
}

// --- Shared Helpers ---

// renderOptions renders the standard permission key hints.
func (m SpecializedPermissionModel) renderOptions() string {
	if m.details.ShowAlwaysAllow {
		return DimStyle.Render("  [y] Approve  [n] Deny  [a] Always approve")
	}
	return DimStyle.Render("  [y] Approve  [n] Deny")
}

// truncateContent truncates text to maxLines lines, each capped at maxWidth chars.
func truncateContent(content string, maxLines, maxWidth int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, DimStyle.Render(fmt.Sprintf("  ... (%d more lines)", len(strings.Split(content, "\n"))-maxLines)))
	}
	for i, line := range lines {
		if len(line) > maxWidth {
			lines[i] = line[:maxWidth] + "..."
		}
	}
	return strings.Join(lines, "\n")
}

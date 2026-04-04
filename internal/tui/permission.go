package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PermissionModel manages the tool permission approval dialog.
type PermissionModel struct {
	toolName    string
	toolInput   string
	description string
	keys        KeyMap
	active      bool
	width       int
}

// NewPermissionModel creates a permission sub-model.
func NewPermissionModel() PermissionModel {
	return PermissionModel{keys: DefaultKeyMap(), width: 60}
}

// Show displays the permission dialog for a tool.
func (m *PermissionModel) Show(toolName, toolInput, description string) {
	m.toolName = toolName
	m.toolInput = toolInput
	m.description = description
	m.active = true
}

// Hide dismisses the dialog.
func (m *PermissionModel) Hide() {
	m.active = false
}

// IsActive returns whether the dialog is showing.
func (m PermissionModel) IsActive() bool { return m.active }

// SetWidth sets the available terminal width for box sizing.
func (m *PermissionModel) SetWidth(w int) {
	m.width = w
}

// Update processes key events for the permission dialog.
func (m PermissionModel) Update(msg tea.Msg) (PermissionModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		k := keyMsg.Key()
		switch {
		case m.keys.IsApprove(k):
			m.active = false
			toolName := m.toolName
			return m, func() tea.Msg {
				return PermissionResponseMsg{Approved: true, ToolName: toolName}
			}
		case m.keys.IsDeny(k):
			m.active = false
			toolName := m.toolName
			return m, func() tea.Msg {
				return PermissionResponseMsg{Approved: false, ToolName: toolName}
			}
		case m.keys.IsAlways(k):
			m.active = false
			toolName := m.toolName
			return m, func() tea.Msg {
				return PermissionResponseMsg{Approved: true, Always: true, ToolName: toolName}
			}
		}
	}
	return m, nil
}

// boxWidth calculates the box width: min 40, max min(width-4, 70).
func (m PermissionModel) boxWidth() int {
	w := m.width - 4
	if w > 70 {
		w = 70
	}
	if w < 40 {
		w = 40
	}
	return w
}

// View renders the permission dialog as a styled bordered box.
func (m PermissionModel) View() string {
	if !m.active {
		return ""
	}

	bw := m.boxWidth()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#E5C07B")).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	// Title
	inner.WriteString(PermissionStyle.Render("Permission Required"))
	inner.WriteString("\n\n")

	// Tool name
	inner.WriteString(ToolNameStyle.Render("Tool: " + m.toolName))
	inner.WriteString("\n")

	// Description
	if m.description != "" {
		inner.WriteString(MessagePadding.Render(m.description))
		inner.WriteString("\n")
	}

	// Tool input (truncated if >200 chars)
	if m.toolInput != "" {
		display := m.toolInput
		if len(display) > 200 {
			display = display[:200] + "..."
		}
		inner.WriteString(MessagePadding.Render(DimStyle.Render(display)))
		inner.WriteString("\n")
	}

	// Separator
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")

	// Options line with y/n/a keys
	inner.WriteString(DimStyle.Render("  [y] Approve  [n] Deny  [a] Always approve"))

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PermissionRuleType classifies the kind of always-allow rule.
type PermissionRuleType int

const (
	// RuleTypeTool allows a specific tool unconditionally.
	RuleTypeTool PermissionRuleType = iota
	// RuleTypePrefix allows a bash command matching a prefix (e.g., "git *").
	RuleTypePrefix
	// RuleTypePath allows file operations on a specific path or glob.
	RuleTypePath
	// RuleTypeDomain allows web-fetch for a specific domain.
	RuleTypeDomain
)

// PermissionRuleEntry represents a single always-allow rule in the UI.
type PermissionRuleEntry struct {
	Type        PermissionRuleType
	ToolName    string // Which tool this rule applies to
	Pattern     string // The pattern (prefix, path glob, domain)
	Description string // Human-readable description
}

// PermissionRuleListModel displays the current list of permission rules
// and allows adding/removing rules. This mirrors the TypeScript
// AddPermissionRules and PermissionRuleList components.
type PermissionRuleListModel struct {
	rules    []PermissionRuleEntry
	selected int
	active   bool
	width    int
}

// NewPermissionRuleListModel creates a new rule list model.
func NewPermissionRuleListModel() PermissionRuleListModel {
	return PermissionRuleListModel{
		rules:    make([]PermissionRuleEntry, 0),
		selected: 0,
		width:    60,
	}
}

// SetRules replaces the current rule list.
func (m *PermissionRuleListModel) SetRules(rules []PermissionRuleEntry) {
	m.rules = rules
	if m.selected >= len(rules) {
		m.selected = max(0, len(rules)-1)
	}
}

// Rules returns the current rules.
func (m PermissionRuleListModel) Rules() []PermissionRuleEntry {
	return m.rules
}

// Show activates the rule list display.
func (m *PermissionRuleListModel) Show() {
	m.active = true
}

// Hide deactivates the rule list display.
func (m *PermissionRuleListModel) Hide() {
	m.active = false
}

// IsActive returns whether the rule list is visible.
func (m PermissionRuleListModel) IsActive() bool {
	return m.active
}

// SetWidth sets the available terminal width.
func (m *PermissionRuleListModel) SetWidth(w int) {
	m.width = w
}

// AddRule appends a rule entry.
func (m *PermissionRuleListModel) AddRule(entry PermissionRuleEntry) {
	m.rules = append(m.rules, entry)
}

// RemoveSelected removes the currently selected rule and returns it.
func (m *PermissionRuleListModel) RemoveSelected() (PermissionRuleEntry, bool) {
	if len(m.rules) == 0 || m.selected >= len(m.rules) {
		return PermissionRuleEntry{}, false
	}
	removed := m.rules[m.selected]
	m.rules = append(m.rules[:m.selected], m.rules[m.selected+1:]...)
	if m.selected >= len(m.rules) && m.selected > 0 {
		m.selected--
	}
	return removed, true
}

// PermissionRuleAddMsg signals that a new permission rule should be added.
type PermissionRuleAddMsg struct {
	Rule PermissionRuleEntry
}

// PermissionRuleRemoveMsg signals that a permission rule should be removed.
type PermissionRuleRemoveMsg struct {
	Rule PermissionRuleEntry
}

// Update processes key events for the rule list.
func (m PermissionRuleListModel) Update(msg tea.Msg) (PermissionRuleListModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	k := keyMsg.Key()
	switch {
	case k.Code == 'j' || k.Code == tea.KeyDown:
		if m.selected < len(m.rules)-1 {
			m.selected++
		}
	case k.Code == 'k' || k.Code == tea.KeyUp:
		if m.selected > 0 {
			m.selected--
		}
	case k.Code == 'd' || k.Code == tea.KeyDelete:
		// Remove selected rule
		if removed, ok := m.RemoveSelected(); ok {
			return m, func() tea.Msg {
				return PermissionRuleRemoveMsg{Rule: removed}
			}
		}
	case k.Code == tea.KeyEscape || k.Code == 'q':
		m.active = false
	}

	return m, nil
}

// View renders the permission rule list.
func (m PermissionRuleListModel) View() string {
	if !m.active {
		return ""
	}

	bw := m.width - 4
	if bw > 70 {
		bw = 70
	}
	if bw < 40 {
		bw = 40
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#61AFEF")).
		Padding(1, 2).
		Width(bw)

	var inner strings.Builder

	inner.WriteString(ToolNameStyle.Render("Permission Rules"))
	inner.WriteString("\n\n")

	if len(m.rules) == 0 {
		inner.WriteString(DimStyle.Render("  No permission rules configured."))
		inner.WriteString("\n")
	} else {
		for i, rule := range m.rules {
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			inner.WriteString(m.renderRuleEntry(prefix, rule, i == m.selected))
			inner.WriteString("\n")
		}
	}

	inner.WriteString("\n")
	sep := strings.Repeat("-", max(1, bw-6))
	inner.WriteString(SeparatorStyle.Render(sep))
	inner.WriteString("\n")
	inner.WriteString(DimStyle.Render("  [d] Remove  [j/k] Navigate  [q] Close"))

	return "\n" + boxStyle.Render(inner.String()) + "\n"
}

// renderRuleEntry renders a single rule entry.
func (m PermissionRuleListModel) renderRuleEntry(prefix string, rule PermissionRuleEntry, selected bool) string {
	typeLabel := ruleTypeLabel(rule.Type)
	line := fmt.Sprintf("%s%s  %s: %s", prefix, typeLabel, rule.ToolName, rule.Pattern)
	if rule.Description != "" {
		line += DimStyle.Render("  " + rule.Description)
	}

	if selected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Render(line)
	}
	return line
}

// ruleTypeLabel returns a short label for the rule type.
func ruleTypeLabel(t PermissionRuleType) string {
	switch t {
	case RuleTypeTool:
		return "[tool]"
	case RuleTypePrefix:
		return "[prefix]"
	case RuleTypePath:
		return "[path]"
	case RuleTypeDomain:
		return "[domain]"
	default:
		return "[rule]"
	}
}

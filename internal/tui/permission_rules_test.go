package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionRuleListModel_Empty(t *testing.T) {
	m := NewPermissionRuleListModel()
	assert.False(t, m.IsActive())
	assert.Len(t, m.Rules(), 0)
}

func TestPermissionRuleListModel_AddRule(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.AddRule(PermissionRuleEntry{
		Type:     RuleTypeTool,
		ToolName: "Bash",
		Pattern:  "*",
	})
	assert.Len(t, m.Rules(), 1)
	assert.Equal(t, "Bash", m.Rules()[0].ToolName)
}

func TestPermissionRuleListModel_SetRules(t *testing.T) {
	m := NewPermissionRuleListModel()
	rules := []PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "Bash", Pattern: "*"},
		{Type: RuleTypePrefix, ToolName: "Bash", Pattern: "git *"},
		{Type: RuleTypePath, ToolName: "Write", Pattern: "/tmp/*"},
	}
	m.SetRules(rules)
	assert.Len(t, m.Rules(), 3)
}

func TestPermissionRuleListModel_RemoveSelected(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.SetRules([]PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "Bash", Pattern: "*"},
		{Type: RuleTypePrefix, ToolName: "Bash", Pattern: "git *"},
	})

	// Remove first (default selected = 0)
	removed, ok := m.RemoveSelected()
	require.True(t, ok)
	assert.Equal(t, "Bash", removed.ToolName)
	assert.Equal(t, "*", removed.Pattern)
	assert.Len(t, m.Rules(), 1)
}

func TestPermissionRuleListModel_RemoveEmptyList(t *testing.T) {
	m := NewPermissionRuleListModel()
	_, ok := m.RemoveSelected()
	assert.False(t, ok)
}

func TestPermissionRuleListModel_NavigateDown(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.SetRules([]PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "A"},
		{Type: RuleTypeTool, ToolName: "B"},
		{Type: RuleTypeTool, ToolName: "C"},
	})
	m.Show()

	// Navigate down
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
	assert.Equal(t, 1, m.selected)

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
	assert.Equal(t, 2, m.selected)

	// Shouldn't go past the end
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
	assert.Equal(t, 2, m.selected)
}

func TestPermissionRuleListModel_NavigateUp(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.SetRules([]PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "A"},
		{Type: RuleTypeTool, ToolName: "B"},
	})
	m.Show()
	m.selected = 1

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'k'}))
	assert.Equal(t, 0, m.selected)

	// Shouldn't go below 0
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'k'}))
	assert.Equal(t, 0, m.selected)
}

func TestPermissionRuleListModel_Close(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.Show()
	assert.True(t, m.IsActive())

	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	assert.False(t, m.IsActive())
}

func TestPermissionRuleListModel_View(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.SetRules([]PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "Bash", Pattern: "*", Description: "all bash"},
		{Type: RuleTypePrefix, ToolName: "Bash", Pattern: "git *"},
	})
	m.Show()

	view := m.View()
	assert.Contains(t, view, "Permission Rules")
	assert.Contains(t, view, "[tool]")
	assert.Contains(t, view, "[prefix]")
	assert.Contains(t, view, "Bash")
	assert.Contains(t, view, "git *")
	assert.Contains(t, view, "all bash")
}

func TestPermissionRuleListModel_ViewEmpty(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.Show()

	view := m.View()
	assert.Contains(t, view, "No permission rules configured")
}

func TestPermissionRuleListModel_InactiveReturnsEmpty(t *testing.T) {
	m := NewPermissionRuleListModel()
	assert.Equal(t, "", m.View())
}

func TestPermissionRuleListModel_DeleteEmitsMsg(t *testing.T) {
	m := NewPermissionRuleListModel()
	m.SetRules([]PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "Bash", Pattern: "*"},
	})
	m.Show()

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd'}))
	require.NotNil(t, cmd)
	msg := cmd()
	removeMsg, ok := msg.(PermissionRuleRemoveMsg)
	require.True(t, ok)
	assert.Equal(t, "Bash", removeMsg.Rule.ToolName)
}

func TestRuleTypeLabel(t *testing.T) {
	assert.Equal(t, "[tool]", ruleTypeLabel(RuleTypeTool))
	assert.Equal(t, "[prefix]", ruleTypeLabel(RuleTypePrefix))
	assert.Equal(t, "[path]", ruleTypeLabel(RuleTypePath))
	assert.Equal(t, "[domain]", ruleTypeLabel(RuleTypeDomain))
}

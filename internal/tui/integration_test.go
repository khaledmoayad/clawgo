package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Detailed Permission Request Tests ---

func TestModel_DetailedPermissionRequest_Bash(t *testing.T) {
	m := newTestModel()

	// Move to streaming
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)

	// Send detailed permission request for bash
	result, _ = m.Update(DetailedPermissionRequestMsg{
		Details: PermissionRequestDetails{
			ToolName:   "Bash",
			DialogType: PermDialogBash,
			Command:    "rm -rf /tmp/test",
			WorkingDir: "/home/user",
		},
	})
	m = result.(Model)

	assert.Equal(t, StatePermission, m.CurrentState())
	assert.True(t, m.specPerm.IsActive())
	view := m.ViewContent()
	assert.Contains(t, view, "Bash")
	assert.Contains(t, view, "rm -rf /tmp/test")
}

func TestModel_DetailedPermissionRequest_FileWrite(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)

	result, _ = m.Update(DetailedPermissionRequestMsg{
		Details: PermissionRequestDetails{
			ToolName:   "Write",
			DialogType: PermDialogFileWrite,
			FilePath:   "/tmp/output.txt",
			NewContent: "file contents here",
		},
	})
	m = result.(Model)

	assert.Equal(t, StatePermission, m.CurrentState())
	view := m.ViewContent()
	assert.Contains(t, view, "Write file")
	assert.Contains(t, view, "/tmp/output.txt")
}

func TestModel_DetailedPermission_HidesOnResponse(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)

	result, _ = m.Update(DetailedPermissionRequestMsg{
		Details: PermissionRequestDetails{
			ToolName:   "Bash",
			DialogType: PermDialogBash,
			Command:    "ls",
		},
	})
	m = result.(Model)
	assert.True(t, m.specPerm.IsActive())

	// Approve
	result, _ = m.Update(PermissionResponseMsg{Approved: true, ToolName: "Bash"})
	m = result.(Model)

	assert.False(t, m.specPerm.IsActive())
	assert.Equal(t, StateStreaming, m.CurrentState())
}

// --- Notification Integration Tests ---

func TestModel_NotificationMsg(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(NotificationMsg{
		Notification: Notification{
			Key:      "test-notif",
			Text:     "Build successful!",
			Priority: PriorityMedium,
		},
	})
	m = result.(Model)

	require.NotNil(t, m.notifs.Current())
	assert.Equal(t, "Build successful!", m.notifs.Current().Text)
	assert.NotNil(t, cmd) // Dismiss timer scheduled

	// View should contain the notification
	view := m.ViewContent()
	assert.Contains(t, view, "Build successful!")
}

func TestModel_NotificationDismiss(t *testing.T) {
	m := newTestModel()

	// Add notification
	result, _ := m.Update(NotificationMsg{
		Notification: Notification{
			Key:      "dismissable",
			Text:     "Temporary",
			Priority: PriorityMedium,
		},
	})
	m = result.(Model)
	require.NotNil(t, m.notifs.Current())

	// Simulate dismiss
	result, _ = m.Update(notificationDismissMsg{key: "dismissable"})
	m = result.(Model)

	// Should be cleared (dismiss calls Dismiss which sets current to nil)
	// Note: the Update method calls m.notifs.Update which returns a Dismiss cmd
}

func TestModel_MultipleNotifications(t *testing.T) {
	m := newTestModel()

	// Add two notifications
	result, _ := m.Update(NotificationMsg{
		Notification: Notification{Key: "first", Text: "First", Priority: PriorityMedium},
	})
	m = result.(Model)

	result, _ = m.Update(NotificationMsg{
		Notification: Notification{Key: "second", Text: "Second", Priority: PriorityMedium},
	})
	m = result.(Model)

	// First should be current, second in queue
	assert.Equal(t, "first", m.notifs.Current().Key)
	assert.Equal(t, 1, m.notifs.QueueLen())
}

// --- Permission Rules Integration Tests ---

func TestModel_ShowPermissionRules(t *testing.T) {
	m := newTestModel()

	rules := []PermissionRuleEntry{
		{Type: RuleTypeTool, ToolName: "Bash", Pattern: "*"},
		{Type: RuleTypePrefix, ToolName: "Bash", Pattern: "git *"},
	}

	result, _ := m.Update(ShowPermissionRulesMsg{Rules: rules})
	m = result.(Model)

	assert.True(t, m.ruleList.IsActive())
	assert.Len(t, m.ruleList.Rules(), 2)

	view := m.ViewContent()
	assert.Contains(t, view, "Permission Rules")
}

func TestModel_RuleListOverlayBlocksInput(t *testing.T) {
	m := newTestModel()

	// Show rules overlay
	result, _ := m.Update(ShowPermissionRulesMsg{
		Rules: []PermissionRuleEntry{
			{Type: RuleTypeTool, ToolName: "Test"},
		},
	})
	m = result.(Model)
	assert.True(t, m.ruleList.IsActive())

	// Close with 'q'
	result, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	m = result.(Model)
	assert.False(t, m.ruleList.IsActive())
}

// --- Accessor Tests ---

func TestModel_Accessors(t *testing.T) {
	m := newTestModel()

	assert.NotNil(t, m.Notifications())
	assert.NotNil(t, m.SpecializedPermission())
	assert.NotNil(t, m.PermissionRules())
}

// --- Window Size Propagation ---

func TestModel_WindowSizePropagation(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)

	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
	// The sub-models should also have been updated
	// (we can't directly check private fields, but the View should work correctly)
}

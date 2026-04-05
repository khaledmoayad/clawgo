package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/tui/renderers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestModel() Model {
	return New(Config{Version: "test", Model: "test-model"})
}

func TestModel_InitialState(t *testing.T) {
	m := newTestModel()
	assert.Equal(t, StateInput, m.CurrentState())
}

func TestModel_SubmitTransition(t *testing.T) {
	m := newTestModel()
	assert.Equal(t, StateInput, m.CurrentState())

	// Simulate a submit message
	result, _ := m.Update(SubmitMsg{Text: "hello"})
	m = result.(Model)

	assert.Equal(t, StateStreaming, m.CurrentState())
	// The user message should be in the output
	require.Len(t, m.output.Messages(), 1)
	assert.Equal(t, "user", m.output.Messages()[0].Role)
	assert.Equal(t, "hello", m.output.Messages()[0].Content)
}

func TestModel_StreamText(t *testing.T) {
	m := newTestModel()

	// Move to streaming state
	result, _ := m.Update(SubmitMsg{Text: "hello"})
	m = result.(Model)
	assert.Equal(t, StateStreaming, m.CurrentState())

	// Send text stream event
	result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventText, Text: "world"}})
	m = result.(Model)

	// The streaming buffer should contain the text
	assert.True(t, m.output.isStreaming)
	assert.Equal(t, "world", m.output.streamingText.String())
}

func TestModel_StreamComplete(t *testing.T) {
	m := newTestModel()

	// Move to streaming state
	result, _ := m.Update(SubmitMsg{Text: "hello"})
	m = result.(Model)

	// Send text then complete
	result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventText, Text: "response"}})
	m = result.(Model)
	result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventMessageComplete}})
	m = result.(Model)

	assert.Equal(t, StateInput, m.CurrentState())
	// The response should be in messages now (user + assistant)
	require.Len(t, m.output.Messages(), 2)
	assert.Equal(t, "assistant", m.output.Messages()[1].Role)
	assert.Equal(t, "response", m.output.Messages()[1].Content)
}

func TestModel_PermissionRequest(t *testing.T) {
	m := newTestModel()

	// Move to streaming
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)

	// Send permission request
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)

	assert.Equal(t, StatePermission, m.CurrentState())
}

func TestModel_PermissionApprove(t *testing.T) {
	m := newTestModel()

	// Move to permission state
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)
	assert.Equal(t, StatePermission, m.CurrentState())

	// Approve
	result, _ = m.Update(PermissionResponseMsg{Approved: true, ToolName: "Bash"})
	m = result.(Model)

	assert.Equal(t, StateStreaming, m.CurrentState())
}

func TestModel_PermissionDeny(t *testing.T) {
	m := newTestModel()

	// Move to permission state
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)

	// Deny
	result, _ = m.Update(PermissionResponseMsg{Approved: false, ToolName: "Bash"})
	m = result.(Model)

	assert.Equal(t, StateInput, m.CurrentState())
}

func TestModel_ErrorMsg(t *testing.T) {
	m := newTestModel()

	// Move to streaming
	result, _ := m.Update(SubmitMsg{Text: "hello"})
	m = result.(Model)

	// Send error
	result, _ = m.Update(ErrorMsg{Err: errors.New("something went wrong")})
	m = result.(Model)

	assert.Equal(t, StateInput, m.CurrentState())
	// Error should be in messages
	lastMsg := m.output.Messages()[len(m.output.Messages())-1]
	assert.Equal(t, "error", lastMsg.Role)
	assert.Contains(t, lastMsg.Content, "something went wrong")
}

func TestModel_CtrlC(t *testing.T) {
	m := newTestModel()

	// Send ctrl+c
	keyMsg := tea.KeyPressMsg(tea.Key{
		Code: 'c',
		Mod:  tea.ModCtrl,
	})
	_, cmd := m.Update(keyMsg)

	// The command should be tea.Quit
	require.NotNil(t, cmd)
	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	assert.True(t, isQuit, "ctrl+c should produce a QuitMsg")
}

func TestModel_ViewInput(t *testing.T) {
	m := newTestModel()
	assert.Equal(t, StateInput, m.CurrentState())

	view := m.ViewContent()
	// Header should contain version and model
	assert.True(t, strings.Contains(view, "ClawGo"), "view should contain app name")
	assert.True(t, strings.Contains(view, "test"), "view should contain version")
}

func TestModel_ViewStreaming(t *testing.T) {
	m := newTestModel()

	// Move to streaming state
	result, _ := m.Update(SubmitMsg{Text: "hello"})
	m = result.(Model)
	assert.Equal(t, StateStreaming, m.CurrentState())

	view := m.ViewContent()
	// Should contain the spinner label when streaming
	assert.True(t, strings.Contains(view, "Thinking"), "streaming view should contain spinner label")
}

// --- PERM-04: Permission dialog key tests ---

func TestModel_PermissionKeyY(t *testing.T) {
	m := newTestModel()

	// Move to permission state
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)
	assert.Equal(t, StatePermission, m.CurrentState())

	// Press 'y'
	keyMsg := tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"})
	result, cmd := m.Update(keyMsg)
	m = result.(Model)

	// The permission model should dispatch PermissionResponseMsg via cmd
	require.NotNil(t, cmd, "pressing y should produce a command")
	msg := cmd()
	resp, ok := msg.(PermissionResponseMsg)
	require.True(t, ok, "command should produce PermissionResponseMsg")
	assert.True(t, resp.Approved)
	assert.False(t, resp.Always)
	assert.Equal(t, "Bash", resp.ToolName)
}

func TestModel_PermissionKeyN(t *testing.T) {
	m := newTestModel()

	// Move to permission state
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)
	assert.Equal(t, StatePermission, m.CurrentState())

	// Press 'n'
	keyMsg := tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"})
	result, cmd := m.Update(keyMsg)
	m = result.(Model)

	require.NotNil(t, cmd, "pressing n should produce a command")
	msg := cmd()
	resp, ok := msg.(PermissionResponseMsg)
	require.True(t, ok, "command should produce PermissionResponseMsg")
	assert.False(t, resp.Approved)
}

func TestModel_PermissionKeyA(t *testing.T) {
	m := newTestModel()

	// Move to permission state
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)
	assert.Equal(t, StatePermission, m.CurrentState())

	// Press 'a'
	keyMsg := tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"})
	result, cmd := m.Update(keyMsg)
	m = result.(Model)

	require.NotNil(t, cmd, "pressing a should produce a command")
	msg := cmd()
	resp, ok := msg.(PermissionResponseMsg)
	require.True(t, ok, "command should produce PermissionResponseMsg")
	assert.True(t, resp.Approved)
	assert.True(t, resp.Always)
}

func TestModel_ViewPermission(t *testing.T) {
	m := newTestModel()

	// Move to permission state
	result, _ := m.Update(SubmitMsg{Text: "do something"})
	m = result.(Model)
	result, _ = m.Update(PermissionRequestMsg{ToolName: "Bash", ToolInput: "ls -la", Description: "List files"})
	m = result.(Model)
	assert.Equal(t, StatePermission, m.CurrentState())

	view := m.ViewContent()
	assert.Contains(t, view, "Bash", "permission view should show tool name")
	assert.Contains(t, view, "[y] Approve", "permission view should show approve option")
	assert.Contains(t, view, "[n] Deny", "permission view should show deny option")
	assert.Contains(t, view, "[a] Always approve", "permission view should show always option")
}

// --- Wave 1 integration tests ---

func TestModel_VirtualScrollIntegrated(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30

	// Add many messages to test virtual scroll integration
	for i := 0; i < 100; i++ {
		result, _ := m.Update(SubmitMsg{Text: "msg"})
		m = result.(Model)
		result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventText, Text: "response"}})
		m = result.(Model)
		result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventMessageComplete}})
		m = result.(Model)
	}

	// Virtual scroll should have messages synced
	assert.Greater(t, m.virtualScroll.TotalLines(), 0, "virtual scroll should have content")

	// View should produce output (not crash)
	view := m.ViewContent()
	assert.NotEmpty(t, view, "view should not be empty with 100 messages")
}

func TestModel_OverlayCtrlK(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30

	// Add some messages first
	result, _ := m.Update(SubmitMsg{Text: "hello"})
	m = result.(Model)
	result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventText, Text: "world"}})
	m = result.(Model)
	result, _ = m.Update(StreamEventMsg{Event: api.StreamEvent{Type: api.EventMessageComplete}})
	m = result.(Model)

	// Ctrl+K should open message selector overlay
	keyMsg := tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl})
	result, _ = m.Update(keyMsg)
	m = result.(Model)

	assert.True(t, m.overlayMgr.IsActive(), "overlay should be active after Ctrl+K")
}

func TestModel_StatusLineUpdates(t *testing.T) {
	m := newTestModel()

	// Send context update
	result, _ := m.Update(ContextUpdateMsg{Percent: 42, Tokens: "21k / 200k"})
	m = result.(Model)

	view := m.statusLine.View()
	assert.Contains(t, view, "42%", "status line should show context percentage")
	assert.Contains(t, view, "21k / 200k", "status line should show token count")
}

func TestModel_ToastLifecycle(t *testing.T) {
	m := newTestModel()
	m.width = 80

	// Send a toast message
	result, cmd := m.Update(ToastMsg{Level: "info", Message: "Test notification"})
	m = result.(Model)

	// The toast should be visible
	notifView := m.notifs.View()
	assert.Contains(t, notifView, "Test notification", "toast should be visible")

	// There should be a dismiss command queued
	assert.NotNil(t, cmd, "toast should queue a dismiss command")
}

func TestModel_SuggestionTrigger(t *testing.T) {
	m := newTestModel()
	m.width = 80

	// Trigger suggestion update
	result, _ := m.Update(SuggestionUpdateMsg{Input: "/hel", CursorPos: 4})
	m = result.(Model)

	// We can't easily test async suggestion results without mocking the provider
	// but we verify the message handler doesn't crash
	assert.Equal(t, StateInput, m.CurrentState())
}

func TestModel_ModelChange(t *testing.T) {
	m := newTestModel()
	assert.Equal(t, "test-model", m.config.Model)

	// Send model change
	result, _ := m.Update(ModelChangeMsg{Name: "claude-opus-4-20250514"})
	m = result.(Model)

	assert.Equal(t, "claude-opus-4-20250514", m.config.Model)
	// View should reflect new model name in header
	view := m.ViewContent()
	assert.Contains(t, view, "claude-opus-4-20250514", "header should show new model name")
}

func TestModel_RendererRegistryIntegrated(t *testing.T) {
	m := newTestModel()

	// Registry should be initialized with renderers
	assert.NotNil(t, m.registry, "registry should be initialized")
	assert.Greater(t, m.registry.Count(), 40, "registry should have 40+ renderers")
}

func TestModel_OutputRegistryDispatch(t *testing.T) {
	reg := renderers.NewRegistry()
	out := NewOutputModelWithRegistry(reg)
	out.SetWidth(80)

	// Add messages and check rendering dispatches through registry
	out.AddMessage(DisplayMessage{Role: "user", Content: "Hello"})
	out.AddMessage(DisplayMessage{Role: "assistant", Content: "World"})
	out.AddMessage(DisplayMessage{Role: "error", Content: "Oops"})

	// RenderSingle should work
	rendered := out.RenderSingle(0)
	assert.NotEmpty(t, rendered, "RenderSingle should return content for user message")

	rendered = out.RenderSingle(1)
	assert.NotEmpty(t, rendered, "RenderSingle should return content for assistant message")

	// Out of range returns empty
	assert.Empty(t, out.RenderSingle(-1), "negative index should return empty")
	assert.Empty(t, out.RenderSingle(99), "out of range index should return empty")
}

func TestModel_HelpDialogShow(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30

	// Send ShowHelpMsg
	result, _ := m.Update(ShowHelpMsg{})
	m = result.(Model)

	assert.True(t, m.helpDialog.IsActive(), "help dialog should be active after ShowHelpMsg")

	// View should contain help content
	view := m.ViewContent()
	assert.Contains(t, view, "Help", "view should contain help title when dialog is active")
}

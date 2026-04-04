package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/khaledmoayad/clawgo/internal/api"
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

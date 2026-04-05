package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

// simpleCommand is a minimal Command implementation for testing.
type simpleCommand struct {
	name        string
	description string
	aliases     []string
	cmdType     commands.CommandType
	resultType  string
	resultValue string
}

func (c *simpleCommand) Name() string        { return c.name }
func (c *simpleCommand) Description() string { return c.description }
func (c *simpleCommand) Aliases() []string   { return c.aliases }
func (c *simpleCommand) Type() commands.CommandType { return c.cmdType }
func (c *simpleCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	value := c.resultValue
	if args != "" && c.name == "model" {
		// Model command returns model_change with the arg as value
		return &commands.CommandResult{Type: "model_change", Value: args}, nil
	}
	if value == "" {
		value = "executed " + c.name
	}
	return &commands.CommandResult{Type: c.resultType, Value: value}, nil
}

func newTestRegistry() *commands.CommandRegistry {
	reg := commands.NewCommandRegistry()
	reg.Register(&simpleCommand{
		name:        "help",
		description: "Show all available commands",
		aliases:     []string{"h", "?"},
		cmdType:     commands.CommandTypeLocal,
		resultType:  "text",
		resultValue: "Available commands:\n  /help - Show all commands\n  /clear - Clear\n  /model - Change model",
	})
	reg.Register(&simpleCommand{
		name:        "clear",
		description: "Clear conversation",
		aliases:     []string{"cl"},
		cmdType:     commands.CommandTypeLocal,
		resultType:  "clear",
	})
	reg.Register(&simpleCommand{
		name:        "compact",
		description: "Compact conversation context",
		aliases:     []string{"co"},
		cmdType:     commands.CommandTypeLocal,
		resultType:  "compact",
	})
	reg.Register(&simpleCommand{
		name:        "model",
		description: "Show or change model",
		aliases:     []string{"m"},
		cmdType:     commands.CommandTypeLocal,
		resultType:  "text",
		resultValue: "Current model: test-model",
	})
	reg.Register(&simpleCommand{
		name:        "exit",
		description: "Exit the REPL",
		aliases:     []string{"quit", "q"},
		cmdType:     commands.CommandTypeLocal,
		resultType:  "exit",
	})
	reg.Register(&simpleCommand{
		name:        "version",
		description: "Show version",
		cmdType:     commands.CommandTypeLocal,
		resultType:  "text",
		resultValue: "ClawGo v0.1.0",
	})
	reg.Register(&simpleCommand{
		name:        "status",
		description: "Show status",
		cmdType:     commands.CommandTypeLocal,
		resultType:  "text",
		resultValue: "Status: OK",
	})
	return reg
}

func newTestModelWithCommands() Model {
	return New(Config{
		Version:     "test",
		Model:       "test-model",
		WorkingDir:  "/tmp",
		SessionID:   "test-session",
		CmdRegistry: newTestRegistry(),
	})
}

// --- Test: /help command produces text output ---

func TestSlashCommand_Help(t *testing.T) {
	m := newTestModelWithCommands()

	// Submit /help
	result, cmd := m.Update(SubmitMsg{Text: "/help"})
	m = result.(Model)

	// The submit should produce a command that returns CommandResultMsg
	require.NotNil(t, cmd, "/help should produce a command")
	msg := cmd()
	cmdResult, ok := msg.(CommandResultMsg)
	require.True(t, ok, "command should produce CommandResultMsg")
	assert.Equal(t, "text", cmdResult.Type)
	assert.Contains(t, cmdResult.Value, "Available commands")
	assert.Equal(t, "help", cmdResult.Command)
}

// --- Test: /clear command clears messages ---

func TestSlashCommand_Clear(t *testing.T) {
	m := newTestModelWithCommands()

	// Add some messages first
	m.output.AddMessage(DisplayMessage{Role: "user", Content: "hello"})
	m.output.AddMessage(DisplayMessage{Role: "assistant", Content: "hi"})
	require.Len(t, m.output.Messages(), 2)

	// Submit /clear
	result, cmd := m.Update(SubmitMsg{Text: "/clear"})
	m = result.(Model)

	require.NotNil(t, cmd)
	msg := cmd()
	cmdResult, ok := msg.(CommandResultMsg)
	require.True(t, ok)
	assert.Equal(t, "clear", cmdResult.Type)

	// Process the CommandResultMsg
	result, _ = m.Update(cmdResult)
	m = result.(Model)

	assert.Len(t, m.output.Messages(), 0, "messages should be cleared after /clear")
}

// --- Test: /model <name> triggers model change callback ---

func TestSlashCommand_ModelChange(t *testing.T) {
	m := newTestModelWithCommands()
	callbackCalled := false
	callbackModel := ""
	m.OnModelChange = func(model string) tea.Cmd {
		callbackCalled = true
		callbackModel = model
		return nil
	}

	// Submit /model new-model
	result, cmd := m.Update(SubmitMsg{Text: "/model new-model"})
	m = result.(Model)

	require.NotNil(t, cmd)
	msg := cmd()
	cmdResult, ok := msg.(CommandResultMsg)
	require.True(t, ok)
	assert.Equal(t, "model_change", cmdResult.Type)
	assert.Equal(t, "new-model", cmdResult.Value)

	// Process the CommandResultMsg
	result, _ = m.Update(cmdResult)
	m = result.(Model)

	assert.True(t, callbackCalled, "OnModelChange callback should be called")
	assert.Equal(t, "new-model", callbackModel)
	assert.Equal(t, "new-model", m.config.Model, "config model should be updated")
}

// --- Test: /compact triggers compact callback ---

func TestSlashCommand_Compact(t *testing.T) {
	m := newTestModelWithCommands()
	compactCalled := false
	m.OnCompact = func() tea.Cmd {
		compactCalled = true
		return nil
	}

	// Submit /compact
	result, cmd := m.Update(SubmitMsg{Text: "/compact"})
	m = result.(Model)

	require.NotNil(t, cmd)
	msg := cmd()
	cmdResult, ok := msg.(CommandResultMsg)
	require.True(t, ok)
	assert.Equal(t, "compact", cmdResult.Type)

	// Process the CommandResultMsg
	result, _ = m.Update(cmdResult)
	m = result.(Model)

	assert.True(t, compactCalled, "OnCompact callback should be called")
}

// --- Test: /exit produces tea.Quit ---

func TestSlashCommand_Exit(t *testing.T) {
	m := newTestModelWithCommands()

	// Submit /exit
	result, cmd := m.Update(SubmitMsg{Text: "/exit"})
	m = result.(Model)

	require.NotNil(t, cmd)
	msg := cmd()
	cmdResult, ok := msg.(CommandResultMsg)
	require.True(t, ok)
	assert.Equal(t, "exit", cmdResult.Type)

	// Process the CommandResultMsg -- should return tea.Quit
	_, quitCmd := m.Update(cmdResult)
	require.NotNil(t, quitCmd, "/exit should produce a quit command")
	quitMsg := quitCmd()
	_, isQuit := quitMsg.(tea.QuitMsg)
	assert.True(t, isQuit, "/exit should produce tea.QuitMsg")
}

// --- Test: Unknown command shows error ---

func TestSlashCommand_Unknown(t *testing.T) {
	m := newTestModelWithCommands()

	// Submit /foo (unknown)
	result, _ := m.Update(SubmitMsg{Text: "/foo"})
	m = result.(Model)

	// Should add an error message
	msgs := m.output.Messages()
	require.NotEmpty(t, msgs)
	lastMsg := msgs[len(msgs)-1]
	assert.Equal(t, "error", lastMsg.Role)
	assert.Contains(t, lastMsg.Content, "Unknown command: /foo")
	assert.Contains(t, lastMsg.Content, "/help")
}

// --- Test: Normal text bypasses command handling ---

func TestSlashCommand_NormalTextBypass(t *testing.T) {
	m := newTestModelWithCommands()
	submitCalled := false
	m.OnSubmit = func(text string) tea.Cmd {
		submitCalled = true
		return nil
	}

	// Submit normal text (not a command)
	result, _ := m.Update(SubmitMsg{Text: "hello world"})
	m = result.(Model)

	assert.True(t, submitCalled, "normal text should call OnSubmit")
	assert.Equal(t, StateStreaming, m.CurrentState(), "should be in streaming state for normal input")
}

// --- Test: Alias resolution ---

func TestSlashCommand_AliasResolution(t *testing.T) {
	m := newTestModelWithCommands()

	// Submit /h (alias for help)
	result, cmd := m.Update(SubmitMsg{Text: "/h"})
	m = result.(Model)

	require.NotNil(t, cmd, "/h should resolve to help and produce a command")
	msg := cmd()
	cmdResult, ok := msg.(CommandResultMsg)
	require.True(t, ok)
	assert.Equal(t, "text", cmdResult.Type)
	assert.Equal(t, "help", cmdResult.Command)
}

// --- Test: Tab completion for /he completes to /help ---

func TestTabCompletion_UniquePrefix(t *testing.T) {
	m := NewInputModel()
	m.SetCommandNames([]string{"help", "clear", "compact", "model", "exit", "version", "status"})

	// Set input to /he
	m.textarea.SetValue("/he")

	// Try completion
	completed, ok := m.tryCompleteCommand("/he")
	assert.True(t, ok, "should find unique match for /he")
	assert.Equal(t, "/help", completed)
}

// --- Test: Tab completion with ambiguous prefix ---

func TestTabCompletion_AmbiguousPrefix(t *testing.T) {
	m := NewInputModel()
	m.SetCommandNames([]string{"help", "clear", "compact", "model", "exit", "version", "status"})

	// /co matches both compact and cost (or in our test: compact only since we don't have cost)
	// Actually "co" matches "compact" and "color" wouldn't be here. Let's test with /cl -> clear
	completed, ok := m.tryCompleteCommand("/cl")
	assert.True(t, ok, "should match /cl to clear (unique)")
	assert.Equal(t, "/clear", completed)

	// /c matches clear, compact -- ambiguous
	completed, ok = m.tryCompleteCommand("/c")
	assert.False(t, ok, "should not complete ambiguous /c")
	assert.Equal(t, "", completed)
}

// --- Test: Tab completion with non-command text ---

func TestTabCompletion_NonCommand(t *testing.T) {
	m := NewInputModel()
	m.SetCommandNames([]string{"help", "clear"})

	// Regular text should not complete
	completed, ok := m.tryCompleteCommand("hello")
	assert.False(t, ok)
	assert.Equal(t, "", completed)
}

// --- Test: Tab completion with empty slash ---

func TestTabCompletion_EmptySlash(t *testing.T) {
	m := NewInputModel()
	m.SetCommandNames([]string{"help", "clear"})

	// Just "/" should not complete (would need to pick from all)
	completed, ok := m.tryCompleteCommand("/")
	assert.False(t, ok)
	assert.Equal(t, "", completed)
}

// --- Test: Tab completion with already-complete command ---

func TestTabCompletion_AlreadyComplete(t *testing.T) {
	m := NewInputModel()
	m.SetCommandNames([]string{"help", "clear"})

	// /help is already complete, trying to tab-complete should still match
	completed, ok := m.tryCompleteCommand("/help")
	assert.True(t, ok, "exact match should complete")
	assert.Equal(t, "/help", completed)
}

// --- Test: Tab completion with arguments ---

func TestTabCompletion_WithArgs(t *testing.T) {
	m := NewInputModel()
	m.SetCommandNames([]string{"help", "model"})

	// /model claude-3 has arguments -- should not attempt completion
	completed, ok := m.tryCompleteCommand("/model claude-3")
	assert.False(t, ok, "should not complete when args present")
	assert.Equal(t, "", completed)
}

// --- Test: CommandResultMsg with text type adds command-role message ---

func TestCommandResult_TextType(t *testing.T) {
	m := newTestModelWithCommands()

	result, _ := m.Update(CommandResultMsg{
		Type:    "text",
		Value:   "Some output text",
		Command: "version",
	})
	m = result.(Model)

	msgs := m.output.Messages()
	require.NotEmpty(t, msgs)
	lastMsg := msgs[len(msgs)-1]
	assert.Equal(t, "command", lastMsg.Role)
	assert.Equal(t, "Some output text", lastMsg.Content)
}

// --- Test: Model without command registry still works ---

func TestNoCommandRegistry(t *testing.T) {
	m := newTestModel() // uses nil registry
	submitCalled := false
	m.OnSubmit = func(text string) tea.Cmd {
		submitCalled = true
		return nil
	}

	// Submit /help -- without registry, should go to OnSubmit as normal text
	result, _ := m.Update(SubmitMsg{Text: "/help"})
	m = result.(Model)

	assert.True(t, submitCalled, "without registry, /help should pass through to OnSubmit")
}

// --- Test: Input is reset after command execution ---

func TestSlashCommand_InputReset(t *testing.T) {
	m := newTestModelWithCommands()

	// Verify input starts empty
	assert.Equal(t, "", m.input.Value())

	// Submit /help -- after command execution, input should be reset
	result, _ := m.Update(SubmitMsg{Text: "/help"})
	m = result.(Model)

	assert.Equal(t, "", m.input.Value(), "input should be reset after command execution")
}

// --- Test: State returns to input after command ---

func TestSlashCommand_StateReturnsToInput(t *testing.T) {
	m := newTestModelWithCommands()
	assert.Equal(t, StateInput, m.CurrentState())

	// Submit /help and process result
	result, cmd := m.Update(SubmitMsg{Text: "/help"})
	m = result.(Model)

	msg := cmd()
	cmdResult := msg.(CommandResultMsg)
	result, _ = m.Update(cmdResult)
	m = result.(Model)

	assert.Equal(t, StateInput, m.CurrentState(), "state should return to input after command")
}

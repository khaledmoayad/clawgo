package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommand implements Command for testing purposes.
type mockCommand struct {
	name        string
	description string
	aliases     []string
	cmdType     CommandType
}

func (m *mockCommand) Name() string                                            { return m.name }
func (m *mockCommand) Description() string                                     { return m.description }
func (m *mockCommand) Aliases() []string                                       { return m.aliases }
func (m *mockCommand) Type() CommandType                                       { return m.cmdType }
func (m *mockCommand) Execute(args string, ctx *CommandContext) (*CommandResult, error) {
	return &CommandResult{Type: "text", Value: "executed " + m.name}, nil
}

func TestCommandTypeConstants(t *testing.T) {
	assert.Equal(t, CommandType(0), CommandTypeLocal, "CommandTypeLocal should be 0")
	assert.Equal(t, CommandType(1), CommandTypePrompt, "CommandTypePrompt should be 1")
}

func TestRegistryRegisterAndFind(t *testing.T) {
	reg := NewCommandRegistry()
	cmd := &mockCommand{name: "help", description: "Show help", aliases: []string{"h", "?"}}
	reg.Register(cmd)

	found, ok := reg.Find("help")
	require.True(t, ok, "should find registered command by name")
	assert.Equal(t, "help", found.Name())
}

func TestRegistryFindResolveAlias(t *testing.T) {
	reg := NewCommandRegistry()
	cmd := &mockCommand{name: "help", description: "Show help", aliases: []string{"h", "?"}}
	reg.Register(cmd)

	found, ok := reg.Find("h")
	require.True(t, ok, "should find command by alias")
	assert.Equal(t, "help", found.Name())

	found2, ok2 := reg.Find("?")
	require.True(t, ok2, "should find command by second alias")
	assert.Equal(t, "help", found2.Name())
}

func TestRegistryFindUnknown(t *testing.T) {
	reg := NewCommandRegistry()
	_, ok := reg.Find("nonexistent")
	assert.False(t, ok, "should return false for unknown command")
}

func TestRegistryAllReturnsRegistrationOrder(t *testing.T) {
	reg := NewCommandRegistry()
	reg.Register(&mockCommand{name: "help", description: "Show help"})
	reg.Register(&mockCommand{name: "model", description: "Change model"})
	reg.Register(&mockCommand{name: "clear", description: "Clear conversation"})

	all := reg.All()
	require.Len(t, all, 3)
	assert.Equal(t, "help", all[0].Name())
	assert.Equal(t, "model", all[1].Name())
	assert.Equal(t, "clear", all[2].Name())
}

func TestParseCommandInput(t *testing.T) {
	reg := NewCommandRegistry()
	reg.Register(&mockCommand{name: "help", description: "Show help", aliases: []string{"h"}})
	reg.Register(&mockCommand{name: "model", description: "Change model"})

	tests := []struct {
		input    string
		wantName string
		wantArgs string
	}{
		{"/help", "help", ""},
		{"/model claude-3-opus", "model", "claude-3-opus"},
		{"/h", "help", ""},
		{"not a command", "", ""},
		{"", "", ""},
		{"/model   multiple  args  ", "model", "multiple  args"},
		{"/unknown", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, args := reg.ParseCommandInput(tt.input)
			assert.Equal(t, tt.wantName, name, "name mismatch")
			assert.Equal(t, tt.wantArgs, args, "args mismatch")
		})
	}
}

func TestIsCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/help", true},
		{"/model claude", true},
		{"hello", false},
		{"", false},
		{"/", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, IsCommand(tt.input))
		})
	}
}

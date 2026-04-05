// Package commands implements the slash command system for ClawGo.
// It defines the Command interface, registry, and argument parsing,
// mirroring the TypeScript commands.ts patterns.
package commands

import (
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// CommandType distinguishes how a command's result is handled by the REPL.
type CommandType int

const (
	// CommandTypeLocal executes immediately and returns text to display.
	CommandTypeLocal CommandType = iota

	// CommandTypePrompt injects text into the conversation as a user message.
	CommandTypePrompt
)

// CommandResult is the return value from Command.Execute.
type CommandResult struct {
	Type          string // "text", "compact", "clear", "model_change", "exit", "skip", "rewind"
	Value         string // Display text or prompt content
	PromptContent string // For prompt-type commands: text to inject as user message
	ExitRequested bool   // Signal to exit the REPL
}

// CommandContext provides runtime state to command execution.
type CommandContext struct {
	WorkingDir   string
	Messages     []api.Message
	CostTracker  *cost.Tracker
	Model        string
	SessionID    string
	Version      string
	ToolRegistry *tools.Registry
	CmdRegistry  *CommandRegistry // Self-reference for /help to list commands
	SystemPrompt string

	// MCPManager holds a reference to the live MCP Manager (typed as any
	// to avoid circular imports between commands and mcp packages).
	// Command implementations that need MCP access type-assert this to
	// *mcp.Manager.
	MCPManager any
}

// Command defines the contract for all slash commands.
// Each command lives in its own sub-package under commands/.
type Command interface {
	// Name returns the primary command name (e.g., "help").
	Name() string

	// Description returns a short human-readable description.
	Description() string

	// Aliases returns alternate names for this command (e.g., ["h", "?"] for help).
	Aliases() []string

	// Type returns whether this is a local or prompt command.
	Type() CommandType

	// Execute runs the command with the given arguments and context.
	Execute(args string, ctx *CommandContext) (*CommandResult, error)
}

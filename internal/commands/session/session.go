// Package session implements the /session slash command.
package session

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// SessionCommand manages conversation sessions.
type SessionCommand struct{}

// New creates a new SessionCommand.
func New() *SessionCommand { return &SessionCommand{} }

func (c *SessionCommand) Name() string                  { return "session" }
func (c *SessionCommand) Description() string            { return "Session management (list, switch)" }
func (c *SessionCommand) Aliases() []string              { return nil }
func (c *SessionCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *SessionCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Current session: %s", ctx.SessionID),
		}, nil
	}
	if args == "list" {
		// Session listing will be implemented with the session persistence layer.
		return &commands.CommandResult{
			Type:  "text",
			Value: "Session listing will be available when session persistence is implemented.",
		}, nil
	}
	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Switching to session: %s", args),
	}, nil
}

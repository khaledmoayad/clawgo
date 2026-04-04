// Package status implements the /status slash command.
package status

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// StatusCommand shows the current session status.
type StatusCommand struct{}

// New creates a new StatusCommand.
func New() *StatusCommand { return &StatusCommand{} }

func (c *StatusCommand) Name() string        { return "status" }
func (c *StatusCommand) Description() string  { return "Show current session status" }
func (c *StatusCommand) Aliases() []string    { return []string{"st"} }
func (c *StatusCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *StatusCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	text := fmt.Sprintf("Session Status\n"+
		"  Model:       %s\n"+
		"  Working dir: %s\n"+
		"  Session ID:  %s\n"+
		"  Messages:    %d",
		ctx.Model, ctx.WorkingDir, ctx.SessionID, len(ctx.Messages))
	return &commands.CommandResult{Type: "text", Value: text}, nil
}

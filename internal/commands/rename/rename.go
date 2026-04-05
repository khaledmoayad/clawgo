// Package rename implements the /rename slash command.
// It renames the current conversation session.
package rename

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// RenameCommand renames the current session.
type RenameCommand struct{}

// New creates a new RenameCommand.
func New() *RenameCommand { return &RenameCommand{} }

func (c *RenameCommand) Name() string              { return "rename" }
func (c *RenameCommand) Description() string        { return "Rename the current conversation" }
func (c *RenameCommand) Aliases() []string          { return nil }
func (c *RenameCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *RenameCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	name := strings.TrimSpace(args)

	if name == "" {
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Current session: %s\nUsage: /rename <new-name>", ctx.SessionID),
		}, nil
	}

	// The actual session renaming is handled by the session persistence layer.
	// We return the new name so the REPL can update session metadata.
	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Session renamed to: %s", name),
	}, nil
}

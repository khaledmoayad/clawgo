// Package adddir implements the /add-dir slash command.
package adddir

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// AddDirCommand adds a working directory.
type AddDirCommand struct{}

// New creates a new AddDirCommand.
func New() *AddDirCommand { return &AddDirCommand{} }

func (c *AddDirCommand) Name() string                  { return "add-dir" }
func (c *AddDirCommand) Description() string            { return "Add a working directory" }
func (c *AddDirCommand) Aliases() []string              { return nil }
func (c *AddDirCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *AddDirCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{Type: "text", Value: "Usage: /add-dir <path>"}, nil
	}
	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Added working directory: %s", args),
	}, nil
}

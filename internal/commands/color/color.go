// Package color implements the /color slash command.
package color

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ColorCommand changes the agent color.
type ColorCommand struct{}

// New creates a new ColorCommand.
func New() *ColorCommand { return &ColorCommand{} }

func (c *ColorCommand) Name() string                  { return "color" }
func (c *ColorCommand) Description() string            { return "Change agent color" }
func (c *ColorCommand) Aliases() []string              { return nil }
func (c *ColorCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *ColorCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{Type: "text", Value: "Usage: /color <color-name>"}, nil
	}
	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Agent color changed to: %s", args),
	}, nil
}

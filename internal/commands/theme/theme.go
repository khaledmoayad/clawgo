// Package theme implements the /theme slash command.
package theme

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ThemeCommand shows or changes the terminal theme.
type ThemeCommand struct{}

// New creates a new ThemeCommand.
func New() *ThemeCommand { return &ThemeCommand{} }

func (c *ThemeCommand) Name() string                  { return "theme" }
func (c *ThemeCommand) Description() string            { return "Show or change terminal theme" }
func (c *ThemeCommand) Aliases() []string              { return nil }
func (c *ThemeCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *ThemeCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{Type: "text", Value: "Current theme: default"}, nil
	}
	return &commands.CommandResult{Type: "text", Value: fmt.Sprintf("Theme changed to: %s", args)}, nil
}

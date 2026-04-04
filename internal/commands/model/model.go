// Package model implements the /model slash command.
package model

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ModelCommand shows or changes the active model.
type ModelCommand struct{}

// New creates a new ModelCommand.
func New() *ModelCommand { return &ModelCommand{} }

func (c *ModelCommand) Name() string        { return "model" }
func (c *ModelCommand) Description() string  { return "Show or change the current model" }
func (c *ModelCommand) Aliases() []string    { return []string{"m"} }
func (c *ModelCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ModelCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Current model: %s", ctx.Model),
		}, nil
	}
	return &commands.CommandResult{
		Type:  "model_change",
		Value: args,
	}, nil
}

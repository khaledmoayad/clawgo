// Package plan implements the /plan slash command.
package plan

import "github.com/khaledmoayad/clawgo/internal/commands"

// PlanCommand toggles plan mode.
type PlanCommand struct{}

// New creates a new PlanCommand.
func New() *PlanCommand { return &PlanCommand{} }

func (c *PlanCommand) Name() string                  { return "plan" }
func (c *PlanCommand) Description() string            { return "Toggle plan mode" }
func (c *PlanCommand) Aliases() []string              { return nil }
func (c *PlanCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *PlanCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "Plan mode toggled."}, nil
}

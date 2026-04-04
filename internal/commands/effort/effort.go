// Package effort implements the /effort slash command.
package effort

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// EffortCommand sets the reasoning effort level.
type EffortCommand struct{}

// New creates a new EffortCommand.
func New() *EffortCommand { return &EffortCommand{} }

func (c *EffortCommand) Name() string                  { return "effort" }
func (c *EffortCommand) Description() string            { return "Set reasoning effort level (low, medium, high)" }
func (c *EffortCommand) Aliases() []string              { return []string{"e"} }
func (c *EffortCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *EffortCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	level := strings.TrimSpace(strings.ToLower(args))
	switch level {
	case "low", "medium", "high":
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Reasoning effort set to: %s", level),
		}, nil
	case "":
		return &commands.CommandResult{
			Type:  "text",
			Value: "Usage: /effort <low|medium|high>",
		}, nil
	default:
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Invalid effort level '%s'. Use: low, medium, or high", level),
		}, nil
	}
}

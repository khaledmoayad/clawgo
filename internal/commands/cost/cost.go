// Package cost implements the /cost slash command.
package cost

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// CostCommand shows session cost and token usage.
type CostCommand struct{}

// New creates a new CostCommand.
func New() *CostCommand { return &CostCommand{} }

func (c *CostCommand) Name() string        { return "cost" }
func (c *CostCommand) Description() string  { return "Show session cost and token usage" }
func (c *CostCommand) Aliases() []string    { return []string{"$"} }
func (c *CostCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *CostCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if ctx.CostTracker == nil {
		return &commands.CommandResult{Type: "text", Value: "Cost tracking not available."}, nil
	}

	tracker := ctx.CostTracker
	totalCost := tracker.Cost()
	text := fmt.Sprintf("Session Cost Summary\n"+
		"  Total cost:    $%.4f\n"+
		"  Input tokens:  %d\n"+
		"  Output tokens: %d\n"+
		"  API turns:     %d",
		totalCost,
		tracker.TotalInputTokens,
		tracker.TotalOutputTokens,
		tracker.TurnCount,
	)
	return &commands.CommandResult{Type: "text", Value: text}, nil
}

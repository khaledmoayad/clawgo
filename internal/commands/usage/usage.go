// Package usage implements the /usage slash command.
package usage

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// UsageCommand shows API usage information.
type UsageCommand struct{}

// New creates a new UsageCommand.
func New() *UsageCommand { return &UsageCommand{} }

func (c *UsageCommand) Name() string                  { return "usage" }
func (c *UsageCommand) Description() string            { return "Show API usage info" }
func (c *UsageCommand) Aliases() []string              { return nil }
func (c *UsageCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *UsageCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if ctx.CostTracker == nil {
		return &commands.CommandResult{Type: "text", Value: "Usage tracking not available."}, nil
	}

	tracker := ctx.CostTracker
	text := fmt.Sprintf("API Usage\n"+
		"  Input tokens:  %d\n"+
		"  Output tokens: %d\n"+
		"  Cache creation: %d\n"+
		"  Cache read:    %d\n"+
		"  Total cost:    $%.4f\n"+
		"  API turns:     %d",
		tracker.TotalInputTokens,
		tracker.TotalOutputTokens,
		tracker.TotalCacheCreationTokens,
		tracker.TotalCacheReadTokens,
		tracker.Cost(),
		tracker.TurnCount,
	)
	return &commands.CommandResult{Type: "text", Value: text}, nil
}

// Package stats implements the /stats slash command.
package stats

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// StatsCommand shows session statistics.
type StatsCommand struct{}

// New creates a new StatsCommand.
func New() *StatsCommand { return &StatsCommand{} }

func (c *StatsCommand) Name() string                  { return "stats" }
func (c *StatsCommand) Description() string            { return "Show session statistics" }
func (c *StatsCommand) Aliases() []string              { return nil }
func (c *StatsCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *StatsCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	msgCount := len(ctx.Messages)
	userMsgs := 0
	assistantMsgs := 0
	toolUses := 0
	for _, msg := range ctx.Messages {
		switch msg.Role {
		case "user":
			userMsgs++
		case "assistant":
			assistantMsgs++
		}
		for _, block := range msg.Content {
			if block.Type == "tool_use" {
				toolUses++
			}
		}
	}

	text := fmt.Sprintf("Session Statistics\n"+
		"  Total messages:    %d\n"+
		"  User messages:     %d\n"+
		"  Assistant messages: %d\n"+
		"  Tool uses:         %d",
		msgCount, userMsgs, assistantMsgs, toolUses)
	return &commands.CommandResult{Type: "text", Value: text}, nil
}

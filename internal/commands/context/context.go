// Package context implements the /context slash command.
package context

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ContextCommand shows conversation context info.
type ContextCommand struct{}

// New creates a new ContextCommand.
func New() *ContextCommand { return &ContextCommand{} }

func (c *ContextCommand) Name() string        { return "context" }
func (c *ContextCommand) Description() string  { return "Show conversation context info" }
func (c *ContextCommand) Aliases() []string    { return []string{"ctx"} }
func (c *ContextCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ContextCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	msgCount := len(ctx.Messages)
	// Rough token estimate: ~4 chars per token
	charCount := 0
	for _, msg := range ctx.Messages {
		for _, block := range msg.Content {
			charCount += len(block.Text) + len(block.Content) + len(block.Thinking)
		}
	}
	estimatedTokens := charCount / 4
	if estimatedTokens < 1 && charCount > 0 {
		estimatedTokens = 1
	}

	text := fmt.Sprintf("Conversation Context\n"+
		"  Messages:         %d\n"+
		"  Estimated tokens: ~%d",
		msgCount, estimatedTokens)
	return &commands.CommandResult{Type: "text", Value: text}, nil
}

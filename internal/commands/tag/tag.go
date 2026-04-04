// Package tag implements the /tag slash command.
package tag

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// TagCommand tags the current conversation.
type TagCommand struct{}

// New creates a new TagCommand.
func New() *TagCommand { return &TagCommand{} }

func (c *TagCommand) Name() string                  { return "tag" }
func (c *TagCommand) Description() string            { return "Tag current conversation" }
func (c *TagCommand) Aliases() []string              { return nil }
func (c *TagCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *TagCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{Type: "text", Value: "Usage: /tag <name>"}, nil
	}
	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Conversation tagged as: %s", args),
	}, nil
}

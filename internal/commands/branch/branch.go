// Package branch implements the /branch slash command.
package branch

import "github.com/khaledmoayad/clawgo/internal/commands"

// BranchCommand injects a prompt about git branch operations.
type BranchCommand struct{}

// New creates a new BranchCommand.
func New() *BranchCommand { return &BranchCommand{} }

func (c *BranchCommand) Name() string        { return "branch" }
func (c *BranchCommand) Description() string  { return "Show or manage git branches" }
func (c *BranchCommand) Aliases() []string    { return nil }
func (c *BranchCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *BranchCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	prompt := "Show me the current git branch and list recent branches."
	if args != "" {
		prompt = "Help me with git branch operations: " + args
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: prompt,
	}, nil
}

// Package diff implements the /diff slash command.
package diff

import "github.com/khaledmoayad/clawgo/internal/commands"

// DiffCommand injects a prompt asking Claude to show recent file changes.
type DiffCommand struct{}

// New creates a new DiffCommand.
func New() *DiffCommand { return &DiffCommand{} }

func (c *DiffCommand) Name() string        { return "diff" }
func (c *DiffCommand) Description() string  { return "Show recent file changes via git diff" }
func (c *DiffCommand) Aliases() []string    { return nil }
func (c *DiffCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *DiffCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	prompt := "Show me the recent file changes using git diff. Summarize what was modified."
	if args != "" {
		prompt = "Show me the git diff for: " + args
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: prompt,
	}, nil
}

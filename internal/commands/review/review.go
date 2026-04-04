// Package review implements the /review slash command.
package review

import "github.com/khaledmoayad/clawgo/internal/commands"

// ReviewCommand injects a code review prompt.
type ReviewCommand struct{}

// New creates a new ReviewCommand.
func New() *ReviewCommand { return &ReviewCommand{} }

func (c *ReviewCommand) Name() string                  { return "review" }
func (c *ReviewCommand) Description() string            { return "Review code changes" }
func (c *ReviewCommand) Aliases() []string              { return nil }
func (c *ReviewCommand) Type() commands.CommandType     { return commands.CommandTypePrompt }

func (c *ReviewCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	prompt := "Review my recent code changes. Look for bugs, style issues, and potential improvements."
	if args != "" {
		prompt = "Review the following code or file: " + args
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: prompt,
	}, nil
}

// Package feedback implements the /feedback slash command.
package feedback

import "github.com/khaledmoayad/clawgo/internal/commands"

// FeedbackCommand injects a prompt for user feedback.
type FeedbackCommand struct{}

// New creates a new FeedbackCommand.
func New() *FeedbackCommand { return &FeedbackCommand{} }

func (c *FeedbackCommand) Name() string                  { return "feedback" }
func (c *FeedbackCommand) Description() string            { return "Provide feedback about ClawGo" }
func (c *FeedbackCommand) Aliases() []string              { return nil }
func (c *FeedbackCommand) Type() commands.CommandType     { return commands.CommandTypePrompt }

func (c *FeedbackCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	prompt := "I'd like to provide feedback about my experience with ClawGo."
	if args != "" {
		prompt = "Feedback: " + args
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: prompt,
	}, nil
}

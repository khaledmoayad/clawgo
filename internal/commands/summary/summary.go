// Package summary implements the /summary slash command.
// It is a prompt-type command that asks the model to summarize the conversation.
package summary

import "github.com/khaledmoayad/clawgo/internal/commands"

// SummaryCommand requests a conversation summary from the model.
type SummaryCommand struct{}

// New creates a new SummaryCommand.
func New() *SummaryCommand { return &SummaryCommand{} }

func (c *SummaryCommand) Name() string              { return "summary" }
func (c *SummaryCommand) Description() string        { return "Summarize the conversation so far" }
func (c *SummaryCommand) Aliases() []string          { return []string{"tldr"} }
func (c *SummaryCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *SummaryCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	prompt := "Please summarize our conversation so far, highlighting the key decisions, changes made, and any outstanding items."
	if args != "" {
		prompt = "Please summarize our conversation, focusing on: " + args
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: prompt,
	}, nil
}

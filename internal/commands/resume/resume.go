// Package resume implements the /resume slash command.
package resume

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ResumeCommand resumes a previous conversation session.
type ResumeCommand struct{}

// New creates a new ResumeCommand.
func New() *ResumeCommand { return &ResumeCommand{} }

func (c *ResumeCommand) Name() string        { return "resume" }
func (c *ResumeCommand) Description() string  { return "Resume a previous conversation session" }
func (c *ResumeCommand) Aliases() []string    { return []string{"r"} }
func (c *ResumeCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *ResumeCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if args == "" {
		return &commands.CommandResult{
			Type:          "text",
			PromptContent: "List my recent conversation sessions so I can choose one to resume.",
		}, nil
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: fmt.Sprintf("Resume conversation session with ID: %s", args),
	}, nil
}

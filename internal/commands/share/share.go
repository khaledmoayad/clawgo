// Package share implements the /share slash command.
// It generates a shareable export of the current conversation.
package share

import (
	"fmt"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ShareCommand generates a shareable session export.
type ShareCommand struct{}

// New creates a new ShareCommand.
func New() *ShareCommand { return &ShareCommand{} }

func (c *ShareCommand) Name() string              { return "share" }
func (c *ShareCommand) Description() string        { return "Share the current conversation" }
func (c *ShareCommand) Aliases() []string          { return nil }
func (c *ShareCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ShareCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if len(ctx.Messages) == 0 {
		return &commands.CommandResult{
			Type:  "text",
			Value: "No conversation to share yet.",
		}, nil
	}

	// Build a markdown export of the conversation
	var sb strings.Builder
	sb.WriteString("# Conversation Export\n\n")
	sb.WriteString(fmt.Sprintf("Session: %s\n", ctx.SessionID))
	sb.WriteString(fmt.Sprintf("Model: %s\n", ctx.Model))
	sb.WriteString(fmt.Sprintf("Exported: %s\n\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("---\n\n")

	for _, msg := range ctx.Messages {
		// Capitalize role name without deprecated strings.Title
		role := msg.Role
		if len(role) > 0 {
			role = strings.ToUpper(role[:1]) + role[1:]
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", role))
		for _, block := range msg.Content {
			if block.Type == "text" && block.Text != "" {
				sb.WriteString(block.Text)
				sb.WriteString("\n\n")
			}
		}
	}

	export := sb.String()

	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Conversation exported (%d messages, %d characters)\n\n%s", len(ctx.Messages), len(export), export),
	}, nil
}

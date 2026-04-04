// Package export implements the /export slash command.
package export

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ExportCommand exports the conversation to JSON or markdown.
type ExportCommand struct{}

// New creates a new ExportCommand.
func New() *ExportCommand { return &ExportCommand{} }

func (c *ExportCommand) Name() string        { return "export" }
func (c *ExportCommand) Description() string  { return "Export conversation to JSON or markdown" }
func (c *ExportCommand) Aliases() []string    { return nil }
func (c *ExportCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ExportCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if strings.TrimSpace(args) == "json" {
		return c.exportJSON(ctx)
	}
	return c.exportMarkdown(ctx)
}

func (c *ExportCommand) exportJSON(ctx *commands.CommandContext) (*commands.CommandResult, error) {
	data, err := json.MarshalIndent(ctx.Messages, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to export as JSON: %w", err)
	}
	return &commands.CommandResult{
		Type:  "text",
		Value: string(data),
	}, nil
}

func (c *ExportCommand) exportMarkdown(ctx *commands.CommandContext) (*commands.CommandResult, error) {
	var b strings.Builder
	b.WriteString("# Conversation Export\n\n")
	for _, msg := range ctx.Messages {
		role := msg.Role
		if role == "user" {
			b.WriteString("## User\n\n")
		} else {
			b.WriteString("## Assistant\n\n")
		}
		for _, block := range msg.Content {
			if block.Text != "" {
				b.WriteString(block.Text)
				b.WriteString("\n\n")
			}
		}
	}
	return &commands.CommandResult{
		Type:  "text",
		Value: b.String(),
	}, nil
}

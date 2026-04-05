// Package outputstyle implements the /output-style slash command.
// It allows switching between text, json, and stream-json output formats.
// Note: In Claude Code this command is deprecated in favor of /config,
// but we keep it for backward compatibility.
package outputstyle

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// validStyles are the supported output formats.
var validStyles = []string{"text", "json", "stream-json"}

// OutputStyleCommand switches or displays the output style.
type OutputStyleCommand struct{}

// New creates a new OutputStyleCommand.
func New() *OutputStyleCommand { return &OutputStyleCommand{} }

func (c *OutputStyleCommand) Name() string              { return "output-style" }
func (c *OutputStyleCommand) Description() string        { return "Switch output format (text, json, stream-json)" }
func (c *OutputStyleCommand) Aliases() []string          { return []string{"os"} }
func (c *OutputStyleCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *OutputStyleCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	style := strings.TrimSpace(args)

	if style == "" {
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Current output style: text\nAvailable styles: %s", strings.Join(validStyles, ", ")),
		}, nil
	}

	// Validate the requested style
	normalized := strings.ToLower(style)
	for _, valid := range validStyles {
		if normalized == valid {
			return &commands.CommandResult{
				Type:  "text",
				Value: fmt.Sprintf("Output style changed to: %s", valid),
			}, nil
		}
	}

	return &commands.CommandResult{
		Type:  "text",
		Value: fmt.Sprintf("Unknown output style: %q\nAvailable styles: %s", style, strings.Join(validStyles, ", ")),
	}, nil
}

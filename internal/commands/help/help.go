// Package help implements the /help slash command.
package help

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// HelpCommand lists all available slash commands.
type HelpCommand struct{}

// New creates a new HelpCommand.
func New() *HelpCommand { return &HelpCommand{} }

func (c *HelpCommand) Name() string        { return "help" }
func (c *HelpCommand) Description() string  { return "Show all available commands" }
func (c *HelpCommand) Aliases() []string    { return []string{"h", "?"} }
func (c *HelpCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *HelpCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	if ctx.CmdRegistry == nil {
		return &commands.CommandResult{Type: "text", Value: "No commands registered."}, nil
	}

	var b strings.Builder
	b.WriteString("Available commands:\n\n")
	for _, cmd := range ctx.CmdRegistry.All() {
		aliases := cmd.Aliases()
		if len(aliases) > 0 {
			b.WriteString(fmt.Sprintf("  /%s (%s) - %s\n", cmd.Name(), strings.Join(aliases, ", "), cmd.Description()))
		} else {
			b.WriteString(fmt.Sprintf("  /%s - %s\n", cmd.Name(), cmd.Description()))
		}
	}
	return &commands.CommandResult{Type: "text", Value: b.String()}, nil
}

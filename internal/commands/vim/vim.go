// Package vim implements the /vim slash command.
package vim

import "github.com/khaledmoayad/clawgo/internal/commands"

// VimCommand toggles vim keybinding mode.
type VimCommand struct{}

// New creates a new VimCommand.
func New() *VimCommand { return &VimCommand{} }

func (c *VimCommand) Name() string              { return "vim" }
func (c *VimCommand) Description() string        { return "Toggle vim keybinding mode" }
func (c *VimCommand) Aliases() []string          { return nil }
func (c *VimCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *VimCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{
		Type:  "vim_toggle",
		Value: "Vim mode toggled",
	}, nil
}

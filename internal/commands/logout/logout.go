// Package logout implements the /logout slash command.
package logout

import "github.com/khaledmoayad/clawgo/internal/commands"

// LogoutCommand performs OAuth logout.
type LogoutCommand struct{}

// New creates a new LogoutCommand.
func New() *LogoutCommand { return &LogoutCommand{} }

func (c *LogoutCommand) Name() string                  { return "logout" }
func (c *LogoutCommand) Description() string            { return "Log out" }
func (c *LogoutCommand) Aliases() []string              { return nil }
func (c *LogoutCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *LogoutCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "OAuth logout available in Phase 3."}, nil
}

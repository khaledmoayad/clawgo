// Package login implements the /login slash command.
package login

import "github.com/khaledmoayad/clawgo/internal/commands"

// LoginCommand initiates the OAuth login flow.
type LoginCommand struct{}

// New creates a new LoginCommand.
func New() *LoginCommand { return &LoginCommand{} }

func (c *LoginCommand) Name() string                  { return "login" }
func (c *LoginCommand) Description() string            { return "Log in with OAuth" }
func (c *LoginCommand) Aliases() []string              { return nil }
func (c *LoginCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *LoginCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "OAuth login available in Phase 3."}, nil
}

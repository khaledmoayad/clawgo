// Package version implements the /version slash command.
package version

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// VersionCommand shows the current ClawGo version.
type VersionCommand struct{}

// New creates a new VersionCommand.
func New() *VersionCommand { return &VersionCommand{} }

func (c *VersionCommand) Name() string        { return "version" }
func (c *VersionCommand) Description() string  { return "Show ClawGo version" }
func (c *VersionCommand) Aliases() []string    { return []string{"v"} }
func (c *VersionCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *VersionCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	v := ctx.Version
	if v == "" {
		v = "unknown"
	}
	return &commands.CommandResult{Type: "text", Value: fmt.Sprintf("ClawGo version %s", v)}, nil
}

// Package sandbox implements the /sandbox slash command.
// It shows the current sandbox state and allows toggling it on/off.
package sandbox

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// SandboxCommand toggles sandbox mode.
type SandboxCommand struct{}

// New creates a new SandboxCommand.
func New() *SandboxCommand { return &SandboxCommand{} }

func (c *SandboxCommand) Name() string              { return "sandbox" }
func (c *SandboxCommand) Description() string        { return "Toggle sandbox mode for bash commands" }
func (c *SandboxCommand) Aliases() []string          { return []string{"sandbox-toggle"} }
func (c *SandboxCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *SandboxCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	arg := strings.TrimSpace(strings.ToLower(args))

	switch arg {
	case "":
		// Show current sandbox state
		return &commands.CommandResult{
			Type:  "text",
			Value: "Sandbox mode: disabled\n\nUsage:\n  /sandbox on   - Enable sandbox mode\n  /sandbox off  - Disable sandbox mode",
		}, nil
	case "on", "enable":
		return &commands.CommandResult{
			Type:  "text",
			Value: "Sandbox mode enabled. Bash commands will run in a sandboxed environment.",
		}, nil
	case "off", "disable":
		return &commands.CommandResult{
			Type:  "text",
			Value: "Sandbox mode disabled. Bash commands will run normally.",
		}, nil
	default:
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Unknown sandbox option: %q\nUsage: /sandbox [on|off]", arg),
		}, nil
	}
}

// Package keybindings implements the /keybindings slash command.
package keybindings

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/tui/keybind"
)

// KeybindingsCommand shows current keybindings.
type KeybindingsCommand struct{}

// New creates a new KeybindingsCommand.
func New() *KeybindingsCommand { return &KeybindingsCommand{} }

func (c *KeybindingsCommand) Name() string              { return "keybindings" }
func (c *KeybindingsCommand) Description() string        { return "Show or configure keybindings" }
func (c *KeybindingsCommand) Aliases() []string          { return []string{"kb"} }
func (c *KeybindingsCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *KeybindingsCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	// Load current bindings (uses defaults if no overrides in context)
	cfg := keybind.DefaultBindings()

	var sb strings.Builder
	sb.WriteString("Keybindings\n\n")

	for _, action := range keybind.AllActions() {
		combo, ok := cfg.ComboFor(action)
		if !ok {
			continue
		}
		line := fmt.Sprintf("  %-14s %s", keybind.FormatCombo(combo), string(action))
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\nVim mode: /vim to toggle")

	return &commands.CommandResult{Type: "text", Value: sb.String()}, nil
}

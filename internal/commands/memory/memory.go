// Package memory implements the /memory slash command.
package memory

import "github.com/khaledmoayad/clawgo/internal/commands"

// MemoryCommand injects a prompt for Claude to manage CLAUDE.md memory entries.
type MemoryCommand struct{}

// New creates a new MemoryCommand.
func New() *MemoryCommand { return &MemoryCommand{} }

func (c *MemoryCommand) Name() string        { return "memory" }
func (c *MemoryCommand) Description() string  { return "Manage CLAUDE.md memory entries" }
func (c *MemoryCommand) Aliases() []string    { return []string{"mem"} }
func (c *MemoryCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *MemoryCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	prompt := "Review and manage my CLAUDE.md memory file. Show current entries and ask what I'd like to add, update, or remove."
	if args != "" {
		prompt = "Update my CLAUDE.md memory file: " + args
	}
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: prompt,
	}, nil
}

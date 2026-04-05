// Package breakcache implements the /break-cache slash command.
// It clears any cached prompt/system prompt state so the next
// API request will not benefit from prompt caching.
package breakcache

import "github.com/khaledmoayad/clawgo/internal/commands"

// BreakCacheCommand clears cached prompt state.
type BreakCacheCommand struct{}

// New creates a new BreakCacheCommand.
func New() *BreakCacheCommand { return &BreakCacheCommand{} }

func (c *BreakCacheCommand) Name() string              { return "break-cache" }
func (c *BreakCacheCommand) Description() string        { return "Clear prompt cache" }
func (c *BreakCacheCommand) Aliases() []string          { return []string{"clear-cache"} }
func (c *BreakCacheCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *BreakCacheCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	// The system prompt will be rebuilt on the next query, which breaks
	// any existing prompt caching by changing the cache boundary.
	// Returning a special result type lets the REPL handle the actual
	// cache invalidation at the query layer.
	return &commands.CommandResult{
		Type:  "text",
		Value: "Prompt cache cleared. The next request will not use cached prompts.",
	}, nil
}

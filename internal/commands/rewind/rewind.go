// Package rewind implements the /rewind slash command.
package rewind

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// RewindCommand rewinds the conversation by N messages.
type RewindCommand struct{}

// New creates a new RewindCommand.
func New() *RewindCommand { return &RewindCommand{} }

func (c *RewindCommand) Name() string                  { return "rewind" }
func (c *RewindCommand) Description() string            { return "Rewind conversation by N messages" }
func (c *RewindCommand) Aliases() []string              { return nil }
func (c *RewindCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *RewindCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	countStr := strings.TrimSpace(args)
	if countStr == "" {
		countStr = "1"
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Invalid count '%s'. Usage: /rewind [N]", countStr),
		}, nil
	}
	return &commands.CommandResult{
		Type:  "rewind",
		Value: strconv.Itoa(count),
	}, nil
}

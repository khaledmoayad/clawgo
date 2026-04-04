// Package tasks implements the /tasks slash command.
package tasks

import "github.com/khaledmoayad/clawgo/internal/commands"

// TasksCommand shows background task status.
type TasksCommand struct{}

// New creates a new TasksCommand.
func New() *TasksCommand { return &TasksCommand{} }

func (c *TasksCommand) Name() string                  { return "tasks" }
func (c *TasksCommand) Description() string            { return "Show background task status" }
func (c *TasksCommand) Aliases() []string              { return nil }
func (c *TasksCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *TasksCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "No background tasks running."}, nil
}

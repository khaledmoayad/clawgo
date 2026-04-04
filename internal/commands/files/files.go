// Package files implements the /files slash command.
package files

import (
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// FilesCommand lists files tracked in the conversation context.
type FilesCommand struct{}

// New creates a new FilesCommand.
func New() *FilesCommand { return &FilesCommand{} }

func (c *FilesCommand) Name() string                  { return "files" }
func (c *FilesCommand) Description() string            { return "List files in conversation context" }
func (c *FilesCommand) Aliases() []string              { return nil }
func (c *FilesCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *FilesCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	// Scan messages for tool uses that reference files (file_read, file_write, etc.)
	// This is a simplified version; full implementation will integrate with file state cache.
	fileSet := make(map[string]bool)
	for _, msg := range ctx.Messages {
		for _, block := range msg.Content {
			if block.Name == "file_read" || block.Name == "file_write" || block.Name == "file_edit" {
				// File paths are in the tool input; for now just note tool usage
				fileSet[block.Name] = true
			}
		}
	}

	if len(fileSet) == 0 {
		return &commands.CommandResult{Type: "text", Value: "No files tracked in current conversation."}, nil
	}

	var b strings.Builder
	b.WriteString("File tools used in conversation:\n")
	for name := range fileSet {
		b.WriteString("  - " + name + "\n")
	}
	return &commands.CommandResult{Type: "text", Value: b.String()}, nil
}

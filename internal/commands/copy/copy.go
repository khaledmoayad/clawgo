// Package copy implements the /copy slash command.
package copy

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// CopyCommand copies the last assistant message to the clipboard.
type CopyCommand struct{}

// New creates a new CopyCommand.
func New() *CopyCommand { return &CopyCommand{} }

func (c *CopyCommand) Name() string                  { return "copy" }
func (c *CopyCommand) Description() string            { return "Copy last assistant message to clipboard" }
func (c *CopyCommand) Aliases() []string              { return nil }
func (c *CopyCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *CopyCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	// Find the last assistant message
	var lastText string
	for i := len(ctx.Messages) - 1; i >= 0; i-- {
		if ctx.Messages[i].Role == "assistant" {
			for _, block := range ctx.Messages[i].Content {
				if block.Text != "" {
					lastText = block.Text
					break
				}
			}
			if lastText != "" {
				break
			}
		}
	}

	if lastText == "" {
		return &commands.CommandResult{Type: "text", Value: "No assistant message to copy."}, nil
	}

	if err := copyToClipboard(lastText); err != nil {
		return &commands.CommandResult{
			Type:  "text",
			Value: fmt.Sprintf("Failed to copy to clipboard: %v", err),
		}, nil
	}

	return &commands.CommandResult{Type: "text", Value: "Copied last assistant message to clipboard."}, nil
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

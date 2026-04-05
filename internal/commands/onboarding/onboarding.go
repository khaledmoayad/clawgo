// Package onboarding implements the /onboarding slash command.
// It displays a welcome message with key features and quick start tips.
package onboarding

import (
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// OnboardingCommand displays onboarding information.
type OnboardingCommand struct{}

// New creates a new OnboardingCommand.
func New() *OnboardingCommand { return &OnboardingCommand{} }

func (c *OnboardingCommand) Name() string              { return "onboarding" }
func (c *OnboardingCommand) Description() string        { return "Show welcome message and quick start tips" }
func (c *OnboardingCommand) Aliases() []string          { return []string{"welcome", "intro"} }
func (c *OnboardingCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *OnboardingCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	var sb strings.Builder
	sb.WriteString("Welcome to ClawGo!\n\n")
	sb.WriteString("ClawGo is a Go implementation of Claude Code -- an AI coding assistant\n")
	sb.WriteString("that runs in your terminal.\n\n")
	sb.WriteString("Quick Start:\n")
	sb.WriteString("  - Type a question or instruction to start a conversation\n")
	sb.WriteString("  - Use /help to see all available commands\n")
	sb.WriteString("  - Use /model to switch between AI models\n")
	sb.WriteString("  - Use /compact to manage conversation context\n")
	sb.WriteString("  - Use /init to set up CLAUDE.md for your project\n\n")
	sb.WriteString("Key Features:\n")
	sb.WriteString("  - Read, write, and edit files in your project\n")
	sb.WriteString("  - Run shell commands\n")
	sb.WriteString("  - Search code with grep and glob patterns\n")
	sb.WriteString("  - Connect to MCP servers for extended capabilities\n")
	sb.WriteString("  - Use plugins and skills for custom workflows\n\n")
	sb.WriteString("Keyboard Shortcuts:\n")
	sb.WriteString("  Ctrl+C  - Cancel current operation\n")
	sb.WriteString("  Ctrl+D  - Exit ClawGo\n")
	sb.WriteString("  Esc     - Toggle vim mode (if enabled)\n\n")
	sb.WriteString("For more information, visit: https://github.com/khaledmoayad/clawgo")

	return &commands.CommandResult{Type: "text", Value: sb.String()}, nil
}

// Package initcmd implements the /init slash command.
// The package is named initcmd to avoid conflict with Go's built-in init function.
// It is a prompt-type command that asks the model to analyze the codebase
// and create a CLAUDE.md file.
package initcmd

import "github.com/khaledmoayad/clawgo/internal/commands"

const initPrompt = `Please analyze this codebase and create a CLAUDE.md file, which will be given to future instances of Claude Code to operate in this repository.

What to add:
1. Commands that will be commonly used, such as how to build, lint, and run tests. Include the necessary commands to develop in this codebase, such as how to run a single test.
2. High-level code architecture and structure so that future instances can be productive more quickly. Focus on the "big picture" architecture that requires reading multiple files to understand.

Usage notes:
- If there's already a CLAUDE.md, suggest improvements to it.
- When you make the initial CLAUDE.md, do not repeat yourself and do not include obvious instructions like "Provide helpful error messages to users", "Write unit tests for all new utilities", "Never include sensitive information (API keys, tokens) in code or commits".
- Avoid listing every component or file structure that can be easily discovered.
- Don't include generic development practices.
- If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include the important parts.
- If there is a README.md, make sure to include the important parts.
- Do not make up information such as "Common Development Tasks", "Tips for Development", "Support and Documentation" unless this is expressly included in other files that you read.
- Be sure to prefix the file with the following text:

` + "```" + `
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
` + "```"

// InitCommand initializes CLAUDE.md for the project.
type InitCommand struct{}

// New creates a new InitCommand.
func New() *InitCommand { return &InitCommand{} }

func (c *InitCommand) Name() string              { return "init" }
func (c *InitCommand) Description() string        { return "Initialize a new CLAUDE.md file with codebase documentation" }
func (c *InitCommand) Aliases() []string          { return nil }
func (c *InitCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *InitCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: initPrompt,
	}, nil
}

// Package skills implements the /skills slash command.
package skills

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// SkillsCommand lists available skills from .claude/skills/.
type SkillsCommand struct{}

// New creates a new SkillsCommand.
func New() *SkillsCommand { return &SkillsCommand{} }

func (c *SkillsCommand) Name() string                  { return "skills" }
func (c *SkillsCommand) Description() string            { return "List available skills" }
func (c *SkillsCommand) Aliases() []string              { return nil }
func (c *SkillsCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *SkillsCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	skillsDir := filepath.Join(ctx.WorkingDir, ".claude", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return &commands.CommandResult{Type: "text", Value: "No skills directory found."}, nil
	}

	var b strings.Builder
	b.WriteString("Available Skills\n\n")
	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			b.WriteString("  - " + entry.Name() + "\n")
			found = true
		}
	}
	if !found {
		b.WriteString("  No skills configured.\n")
	}
	return &commands.CommandResult{Type: "text", Value: b.String()}, nil
}

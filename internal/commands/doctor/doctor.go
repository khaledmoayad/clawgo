// Package doctor implements the /doctor slash command.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// DoctorCommand diagnoses environment issues.
type DoctorCommand struct{}

// New creates a new DoctorCommand.
func New() *DoctorCommand { return &DoctorCommand{} }

func (c *DoctorCommand) Name() string                  { return "doctor" }
func (c *DoctorCommand) Description() string            { return "Diagnose environment issues" }
func (c *DoctorCommand) Aliases() []string              { return nil }
func (c *DoctorCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *DoctorCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	var b strings.Builder
	b.WriteString("Environment Diagnostics\n\n")

	// Check git
	if _, err := exec.LookPath("git"); err != nil {
		b.WriteString("  [FAIL] git: not found in PATH\n")
	} else {
		b.WriteString("  [OK]   git: available\n")
	}

	// Check API key
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		b.WriteString("  [OK]   ANTHROPIC_API_KEY: set\n")
	} else {
		b.WriteString("  [WARN] ANTHROPIC_API_KEY: not set\n")
	}

	// Check config dir
	homeDir, _ := os.UserHomeDir()
	configDir := homeDir + "/.claude"
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		b.WriteString(fmt.Sprintf("  [OK]   Config dir: %s\n", configDir))
	} else {
		b.WriteString(fmt.Sprintf("  [WARN] Config dir: %s not found\n", configDir))
	}

	// Check working directory
	if ctx.WorkingDir != "" {
		if _, err := os.Stat(ctx.WorkingDir); err == nil {
			b.WriteString(fmt.Sprintf("  [OK]   Working dir: %s\n", ctx.WorkingDir))
		} else {
			b.WriteString(fmt.Sprintf("  [FAIL] Working dir: %s (not accessible)\n", ctx.WorkingDir))
		}
	}

	return &commands.CommandResult{Type: "text", Value: b.String()}, nil
}

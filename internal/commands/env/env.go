// Package env implements the /env slash command.
package env

import (
	"fmt"
	"os"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// EnvCommand shows relevant environment variables.
type EnvCommand struct{}

// New creates a new EnvCommand.
func New() *EnvCommand { return &EnvCommand{} }

func (c *EnvCommand) Name() string        { return "env" }
func (c *EnvCommand) Description() string  { return "Show relevant environment variables" }
func (c *EnvCommand) Aliases() []string    { return nil }
func (c *EnvCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

// relevantEnvVars lists the environment variables to display.
var relevantEnvVars = []string{
	"ANTHROPIC_MODEL",
	"ANTHROPIC_BASE_URL",
	"CLAUDE_CODE_API_BASE_URL",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX",
	"CLAUDE_CODE_USE_FOUNDRY",
	"CLAUDE_CONFIG_DIR",
	"CLAUDE_CODE_REMOTE",
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
}

func (c *EnvCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	var b strings.Builder
	b.WriteString("Environment Variables\n\n")

	for _, name := range relevantEnvVars {
		val := os.Getenv(name)
		if val == "" {
			b.WriteString(fmt.Sprintf("  %s: (not set)\n", name))
		} else {
			// Redact sensitive values, showing only last 4 chars
			if strings.Contains(strings.ToLower(name), "key") || strings.Contains(strings.ToLower(name), "token") {
				if len(val) > 4 {
					val = "****" + val[len(val)-4:]
				} else {
					val = "****"
				}
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", name, val))
		}
	}
	return &commands.CommandResult{Type: "text", Value: b.String()}, nil
}

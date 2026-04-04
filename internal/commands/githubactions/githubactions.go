// Package githubactions implements the /install-github-app slash command.
// It sets up a Claude Code GitHub Actions workflow in the current repository.
package githubactions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

const workflowTemplate = `name: Claude Code
on:
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]
jobs:
  claude:
    if: contains(github.event.comment.body, '@claude')
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
      issues: write
    steps:
      - uses: actions/checkout@v4
      - uses: anthropics/claude-code-action@v1
        with:
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
`

// GitHubActionsCommand sets up the Claude Code GitHub Actions workflow.
type GitHubActionsCommand struct{}

// New creates a new GitHubActionsCommand.
func New() *GitHubActionsCommand { return &GitHubActionsCommand{} }

func (c *GitHubActionsCommand) Name() string              { return "install-github-app" }
func (c *GitHubActionsCommand) Description() string        { return "Set up Claude Code GitHub Actions workflow" }
func (c *GitHubActionsCommand) Aliases() []string          { return []string{"github-actions"} }
func (c *GitHubActionsCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *GitHubActionsCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	result, err := Run(context.Background(), strings.Fields(args))
	if err != nil {
		return &commands.CommandResult{Type: "text", Value: fmt.Sprintf("Error: %s", err)}, nil
	}
	return &commands.CommandResult{Type: "text", Value: result}, nil
}

// Run executes the GitHub Actions setup workflow.
// It creates the workflow YAML file and optionally sets the ANTHROPIC_API_KEY
// secret via the gh CLI.
func Run(ctx context.Context, args []string) (string, error) {
	// Check for gh CLI
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh CLI required: install from https://cli.github.com/ and run 'gh auth login'")
	}

	// Determine working directory
	workDir := "."

	// Create .github/workflows/ directory
	workflowDir := filepath.Join(workDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", workflowDir, err)
	}

	// Write workflow file
	workflowPath := filepath.Join(workflowDir, "claude-code.yml")
	if err := os.WriteFile(workflowPath, []byte(generateWorkflow()), 0o644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", workflowPath, err)
	}

	// Check for --key flag
	var apiKey string
	for i, arg := range args {
		if arg == "--key" && i+1 < len(args) {
			apiKey = args[i+1]
			break
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Created workflow: %s\n", workflowPath))

	if apiKey != "" {
		// Set the secret via gh CLI
		cmd := exec.CommandContext(ctx, "gh", "secret", "set", "ANTHROPIC_API_KEY", "--body", apiKey)
		output, err := cmd.CombinedOutput()
		if err != nil {
			sb.WriteString(fmt.Sprintf("Warning: failed to set secret: %s\n", strings.TrimSpace(string(output))))
			sb.WriteString("Run manually: gh secret set ANTHROPIC_API_KEY\n")
		} else {
			sb.WriteString("Set repository secret: ANTHROPIC_API_KEY\n")
		}
	} else {
		sb.WriteString("\nNext steps:\n")
		sb.WriteString("  1. Set your API key as a repository secret:\n")
		sb.WriteString("     gh secret set ANTHROPIC_API_KEY\n")
		sb.WriteString("  2. Commit and push the workflow file\n")
		sb.WriteString("  3. Comment '@claude' on any issue or PR to trigger Claude\n")
	}

	return sb.String(), nil
}

// generateWorkflow returns the GitHub Actions workflow YAML template.
func generateWorkflow() string {
	return workflowTemplate
}

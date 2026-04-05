// Package commit implements the /commit slash command.
// It is a prompt-type command that asks the model to create a git commit
// based on the current changes in the working directory.
package commit

import "github.com/khaledmoayad/clawgo/internal/commands"

const commitPrompt = `## Context

- Current git status: Run 'git status' to check
- Current git diff (staged and unstaged changes): Run 'git diff HEAD' to check
- Current branch: Run 'git branch --show-current' to check
- Recent commits: Run 'git log --oneline -10' to check

## Git Safety Protocol

- NEVER update the git config
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- CRITICAL: ALWAYS create NEW commits. NEVER use git commit --amend, unless the user explicitly requests it
- Do not commit files that likely contain secrets (.env, credentials.json, etc). Warn the user if they specifically request to commit those files
- If there are no changes to commit (i.e., no untracked files and no modifications), do not create an empty commit
- Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported

## Your task

Based on the above changes, create a single git commit:

1. First run the git commands above to understand the current state
2. Analyze all staged changes and draft a commit message:
   - Look at the recent commits to follow this repository's commit message style
   - Summarize the nature of the changes (new feature, enhancement, bug fix, refactoring, test, docs, etc.)
   - Ensure the message accurately reflects the changes and their purpose
   - Draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what"

3. Stage relevant files and create the commit using HEREDOC syntax:
` + "```" + `
git commit -m "$(cat <<'EOF'
Commit message here.
EOF
)"
` + "```" + `

Stage and create the commit using a single message. Do not use any other tools or do anything else.`

// CommitCommand generates a git commit from the current changes.
type CommitCommand struct{}

// New creates a new CommitCommand.
func New() *CommitCommand { return &CommitCommand{} }

func (c *CommitCommand) Name() string              { return "commit" }
func (c *CommitCommand) Description() string        { return "Create a git commit" }
func (c *CommitCommand) Aliases() []string          { return nil }
func (c *CommitCommand) Type() commands.CommandType { return commands.CommandTypePrompt }

func (c *CommitCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{
		Type:          "text",
		PromptContent: commitPrompt,
	}, nil
}

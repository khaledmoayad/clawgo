package enterworktree

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Creates a new git worktree and switches the working directory to it.

This allows working on multiple branches simultaneously without stashing changes. The new worktree will be checked out at the specified branch.`

// inputSchemaJSON is the JSON Schema for EnterWorktreeTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "branch": {
            "type": "string",
            "description": "The branch to check out in the new worktree"
        },
        "path": {
            "type": "string",
            "description": "Optional path for the new worktree (defaults to ../branch-name)"
        }
    },
    "required": ["branch"]
}`

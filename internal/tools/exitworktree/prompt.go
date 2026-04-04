package exitworktree

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Exits a git worktree and switches back to the main project directory.

Optionally removes the worktree from disk. The working directory is reset to the project root.`

// inputSchemaJSON is the JSON Schema for ExitWorktreeTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "path": {
            "type": "string",
            "description": "The path of the worktree to exit"
        },
        "remove": {
            "type": "boolean",
            "description": "Whether to remove the worktree from disk (default: false)"
        }
    },
    "required": ["path"]
}`

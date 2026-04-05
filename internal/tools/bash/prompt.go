package bash

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Executes a given bash command and returns its output.

The working directory persists between commands, but shell state does not. The shell environment is initialized from the user's profile.

IMPORTANT: Avoid using this tool for tasks that can be accomplished with dedicated tools (Read, Write, Edit, Grep, Glob).`

// inputSchemaJSON is the JSON Schema for BashTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "command": {
            "type": "string",
            "description": "The bash command to execute"
        },
        "timeout": {
            "type": "integer",
            "description": "Optional timeout in milliseconds (max 600000)"
        },
        "run_in_background": {
            "type": "boolean",
            "description": "Set to true to run this command in the background. Use Read to read the output later."
        },
        "dangerouslyDisableSandbox": {
            "type": "boolean",
            "description": "Set this to true to dangerously override sandbox mode and run commands without sandboxing."
        },
        "description": {
            "type": "string",
            "description": "Clear, concise description of what this command does in active voice."
        }
    },
    "required": ["command"]
}`

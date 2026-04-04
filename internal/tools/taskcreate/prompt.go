package taskcreate

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Creates a new background task for concurrent execution.

Tasks can be shell commands or sub-agent work. Use this to run long-running operations in the background while continuing other work.`

// inputSchemaJSON is the JSON Schema for TaskCreateTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "description": {
            "type": "string",
            "description": "A description of the task to create"
        },
        "type": {
            "type": "string",
            "enum": ["local_bash", "local_agent"],
            "description": "The type of task to create (default: local_bash)"
        },
        "command": {
            "type": "string",
            "description": "The command to execute (for local_bash tasks)"
        }
    },
    "required": ["description"]
}`

package taskupdate

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Updates the status or message of a background task.

Use this to mark tasks as running, completed, or failed, and to add output messages.`

// inputSchemaJSON is the JSON Schema for TaskUpdateTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "task_id": {
            "type": "string",
            "description": "The ID of the task to update"
        },
        "status": {
            "type": "string",
            "enum": ["pending", "running", "completed", "stopped", "failed"],
            "description": "The new status for the task"
        },
        "message": {
            "type": "string",
            "description": "A message or output to associate with the task"
        }
    },
    "required": ["task_id"]
}`

package tasklist

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Lists all background tasks and their current statuses.

Optionally filter by status to see only pending, running, completed, stopped, or failed tasks.`

// inputSchemaJSON is the JSON Schema for TaskListTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "status": {
            "type": "string",
            "enum": ["pending", "running", "completed", "stopped", "failed"],
            "description": "Optional status filter to show only tasks with this status"
        }
    }
}`

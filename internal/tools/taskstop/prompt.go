package taskstop

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Stops a running background task by its ID.

The task will be marked as stopped and any running process will be terminated.`

// inputSchemaJSON is the JSON Schema for TaskStopTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "task_id": {
            "type": "string",
            "description": "The ID of the task to stop"
        }
    },
    "required": ["task_id"]
}`

package taskget

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Gets the status and details of a background task by its ID.

Returns the task's current status, type, description, and any output.`

// inputSchemaJSON is the JSON Schema for TaskGetTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "taskId": {
            "type": "string",
            "description": "The ID of the task to retrieve"
        }
    },
    "required": ["taskId"]
}`

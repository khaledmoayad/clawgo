package taskoutput

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Retrieves the output or log of a background task by its ID.

Returns any output produced by the task during its execution.`

// inputSchemaJSON is the JSON Schema for TaskOutputTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "task_id": {
            "type": "string",
            "description": "The ID of the task whose output to retrieve"
        }
    },
    "required": ["task_id"]
}`

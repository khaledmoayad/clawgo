package teamdelete

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Deletes an existing team and cancels all its running workers.

All worker agents in the team will be stopped and their contexts cancelled.`

// inputSchemaJSON is the JSON Schema for TeamDeleteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "name": {
            "type": "string",
            "description": "The name of the team to delete"
        }
    },
    "required": ["name"]
}`

package teamdelete

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Deletes the current team and cancels all its running workers.

All worker agents in the team will be stopped and their contexts cancelled.`

// inputSchemaJSON is the JSON Schema for TeamDeleteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {}
}`

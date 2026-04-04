package crondelete

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Deletes a scheduled cron task by name.

The task will no longer run on its schedule.`

// inputSchemaJSON is the JSON Schema for CronDeleteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "name": {
            "type": "string",
            "description": "The name of the cron task to delete"
        }
    },
    "required": ["name"]
}`

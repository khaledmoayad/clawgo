package crondelete

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Deletes a scheduled cron task by ID.

The task will no longer fire on its schedule. Use CronList to find task IDs.`

// inputSchemaJSON is the JSON Schema for CronDeleteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "id": {
            "type": "string",
            "description": "The ID of the cron task to delete (returned by CronCreate)"
        }
    },
    "required": ["id"]
}`

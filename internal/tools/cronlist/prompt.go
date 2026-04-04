package cronlist

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Lists all configured cron tasks and their schedules.

Shows the name, cron expression, command, and status of each scheduled task.`

// inputSchemaJSON is the JSON Schema for CronListTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {}
}`

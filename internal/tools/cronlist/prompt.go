package cronlist

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Lists all configured cron tasks and their schedules.

Shows the ID, cron expression, human-readable schedule, prompt, and status of each scheduled task. Includes both durable (file-backed) and session-scoped tasks.`

// inputSchemaJSON is the JSON Schema for CronListTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {}
}`

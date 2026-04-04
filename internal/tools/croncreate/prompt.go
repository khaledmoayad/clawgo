package croncreate

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Creates a new scheduled task with a cron expression.

The task will run the specified command according to the cron schedule.`

// inputSchemaJSON is the JSON Schema for CronCreateTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "name": {
            "type": "string",
            "description": "A unique name for the scheduled task"
        },
        "schedule": {
            "type": "string",
            "description": "Cron expression defining the schedule (e.g., '0 */6 * * *')"
        },
        "command": {
            "type": "string",
            "description": "The command to execute on each scheduled run"
        }
    },
    "required": ["name", "schedule", "command"]
}`

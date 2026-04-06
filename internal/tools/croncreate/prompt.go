package croncreate

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Creates a new scheduled task with a cron expression.

The task will execute the given prompt at each scheduled fire time. Use a standard 5-field cron expression (minute, hour, day-of-month, month, day-of-week) in local time.

By default tasks are recurring (fire on every cron match, auto-expire after 7 days) and non-durable (in-memory only, lost when session ends). Set durable=true to persist across restarts.`

// inputSchemaJSON is the JSON Schema for CronCreateTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "cron": {
            "type": "string",
            "description": "Standard 5-field cron expression in local time: \"M H DoM Mon DoW\""
        },
        "prompt": {
            "type": "string",
            "description": "Prompt to enqueue at each fire time"
        },
        "recurring": {
            "type": "boolean",
            "description": "true = fire on every cron match (auto-expires after 7 days); false = fire once then auto-delete. Defaults to true."
        },
        "durable": {
            "type": "boolean",
            "description": "true = persist to .claude/scheduled_tasks.json and survive restarts; false = in-memory only. Defaults to false."
        }
    },
    "required": ["cron", "prompt"]
}`

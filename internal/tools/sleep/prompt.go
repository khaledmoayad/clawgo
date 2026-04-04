package sleep

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Pause execution for a specified number of seconds. Useful when waiting for external processes or rate limiting. Maximum duration is 300 seconds (5 minutes). The sleep is cancellable via context cancellation.`

// inputSchemaJSON is the JSON Schema for SleepTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "seconds": {
            "type": "number",
            "description": "Number of seconds to sleep (1-300)"
        }
    },
    "required": ["seconds"]
}`

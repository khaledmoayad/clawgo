package sendmessage

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Sends a follow-up message to a running worker agent.

Used to continue a worker with additional instructions, corrections, or new tasks.
The worker receives the message and resumes its query loop with the new context.`

// inputSchemaJSON is the JSON Schema for SendMessageTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "to": {
            "type": "string",
            "description": "The agent ID of the worker to send the message to (from Agent tool launch result)"
        },
        "message": {
            "type": "string",
            "description": "The message content to send to the worker"
        }
    },
    "required": ["to", "message"]
}`

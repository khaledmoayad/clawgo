package brief

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Send a message to the user. This is your primary visible output channel. Use this to communicate findings, status updates, and responses. Supports markdown formatting and file attachments.`

// inputSchemaJSON is the JSON Schema for SendUserMessage (Brief) tool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "message": {
            "type": "string",
            "description": "The message for the user. Supports markdown formatting."
        },
        "attachments": {
            "type": "array",
            "items": {
                "type": "string"
            },
            "description": "Optional file paths (absolute or relative to cwd) to attach. Use for photos, screenshots, diffs, logs, or any file the user should see alongside your message."
        },
        "status": {
            "type": "string",
            "enum": ["normal", "proactive"],
            "description": "Use 'proactive' when you're surfacing something the user hasn't asked for and needs to see now - task completion while they're away, a blocker you hit, an unsolicited status update. Use 'normal' when replying to something the user just said."
        }
    },
    "required": ["message", "status"]
}`

package brief

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Enable brief mode for subsequent responses. When brief mode is active, responses should be shorter and more concise. Use this when the user requests shorter output or when detailed explanations are not needed.`

// inputSchemaJSON is the JSON Schema for BriefTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "message": {
            "type": "string",
            "description": "Optional message to display when entering brief mode"
        }
    },
    "required": ["message"]
}`

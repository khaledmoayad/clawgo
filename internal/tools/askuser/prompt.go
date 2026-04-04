package askuser

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Ask the user a question and wait for their response. Use this when you need clarification, confirmation, or additional input from the user to proceed with the task. The question will be displayed to the user and their response will be returned.`

// inputSchemaJSON is the JSON Schema for AskUserTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "question": {
            "type": "string",
            "description": "The question to ask the user"
        }
    },
    "required": ["question"]
}`

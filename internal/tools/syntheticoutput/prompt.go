package syntheticoutput

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Returns content directly as a tool result.

Used by the system to inject synthetic tool outputs into the conversation. Supports text, JSON, and markdown formats.`

// inputSchemaJSON is the JSON Schema for SyntheticOutputTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "content": {
            "type": "string",
            "description": "The content to return as the tool result"
        },
        "format": {
            "type": "string",
            "enum": ["text", "json", "markdown"],
            "description": "The format of the content (default: text)"
        }
    },
    "required": ["content"]
}`

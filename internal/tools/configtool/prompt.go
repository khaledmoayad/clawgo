package configtool

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Read and modify project configuration settings. Supports getting and setting configuration values from the project's .claude/settings.json. Omit value to get current setting; provide value to set it.`

// inputSchemaJSON is the JSON Schema for ConfigTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "setting": {
            "type": "string",
            "description": "The setting key (e.g., \"theme\", \"model\", \"permissions.defaultMode\")"
        },
        "value": {
            "description": "The new value. Omit to get current value.",
            "oneOf": [
                {"type": "string"},
                {"type": "boolean"},
                {"type": "number"}
            ]
        }
    },
    "required": ["setting"]
}`

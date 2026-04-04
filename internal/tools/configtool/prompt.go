package configtool

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Read and modify project configuration settings. Supports getting, setting, and listing configuration values from the project's .claude/settings.json.`

// inputSchemaJSON is the JSON Schema for ConfigTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "action": {
            "type": "string",
            "enum": ["get", "set", "list"],
            "description": "The config operation to perform"
        },
        "key": {
            "type": "string",
            "description": "Configuration key (required for get and set)"
        },
        "value": {
            "type": "string",
            "description": "Configuration value (required for set)"
        }
    },
    "required": ["action"]
}`

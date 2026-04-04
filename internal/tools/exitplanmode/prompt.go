package exitplanmode

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Exit plan mode and return to the previous permission mode. Optionally provide prompt-based permissions needed to implement the plan.`

// inputSchemaJSON is the JSON Schema for ExitPlanModeTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "allowedPrompts": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "tool": {
                        "type": "string",
                        "description": "The tool this prompt applies to"
                    },
                    "prompt": {
                        "type": "string",
                        "description": "Semantic description of the action"
                    }
                },
                "required": ["tool", "prompt"]
            },
            "description": "Prompt-based permissions needed to implement the plan"
        }
    }
}`

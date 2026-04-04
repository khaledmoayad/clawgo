package exitplanmode

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Exit plan mode and return to the previous permission mode. Provide a summary of the plan for user review before exiting.`

// inputSchemaJSON is the JSON Schema for ExitPlanModeTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "plan_summary": {
            "type": "string",
            "description": "Summary of the plan for user review"
        }
    },
    "required": ["plan_summary"]
}`

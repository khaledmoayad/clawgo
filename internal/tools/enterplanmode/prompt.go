package enterplanmode

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Enter plan mode where all mutations require explicit user approval. In plan mode, the assistant can read and analyze but cannot make changes without permission. Use this to switch to a more cautious execution mode.`

// inputSchemaJSON is the JSON Schema for EnterPlanModeTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {},
    "required": []
}`

package syntheticoutput

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Return structured output in the requested format. You MUST call this tool exactly once at the end of your response to provide the structured output.`

// inputSchemaJSON is the JSON Schema for StructuredOutput tool input.
// The schema accepts any JSON object -- the actual schema is dynamic and
// validated at runtime against the user-provided --json-schema flag.
const inputSchemaJSON = `{
    "type": "object",
    "additionalProperties": true
}`

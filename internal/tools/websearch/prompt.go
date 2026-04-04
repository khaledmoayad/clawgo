package websearch

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Performs a web search and returns results. Uses Anthropic's server-side web search capability. Provide a search query to find relevant web content.`

// inputSchemaJSON is the JSON Schema for WebSearchTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "query": {
            "type": "string",
            "description": "The search query"
        }
    },
    "required": ["query"]
}`

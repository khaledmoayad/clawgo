package toolsearch

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Searches available tools by name or description.

Returns a list of tools matching the query string, useful for discovering available capabilities.`

// inputSchemaJSON is the JSON Schema for ToolSearchTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "query": {
            "type": "string",
            "description": "The search query to match against tool names and descriptions"
        }
    },
    "required": ["query"]
}`

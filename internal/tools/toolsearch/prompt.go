package toolsearch

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Searches available tools by name or description.

Returns a list of tools matching the query string. Use "select:<tool_name>" for direct tool selection, or keywords to search. Supports comma-separated multi-select: "select:A,B,C".`

// inputSchemaJSON is the JSON Schema for ToolSearchTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "query": {
            "type": "string",
            "description": "Query to find tools. Use \"select:<tool_name>\" for direct selection, or keywords to search."
        },
        "max_results": {
            "type": "number",
            "description": "Maximum number of results to return (default: 5)"
        }
    },
    "required": ["query"]
}`

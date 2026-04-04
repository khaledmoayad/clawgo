package listmcpresources

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Lists available MCP (Model Context Protocol) resources from connected servers.

Returns resources provided by MCP servers, optionally filtered by server name.`

// inputSchemaJSON is the JSON Schema for ListMcpResourcesTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "server": {
            "type": "string",
            "description": "Optional server name to filter resources by"
        }
    }
}`

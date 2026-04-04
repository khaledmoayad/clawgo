package readmcpresource

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Reads a specific MCP (Model Context Protocol) resource by URI from a connected server.

Returns the content of the specified resource from the named MCP server.`

// inputSchemaJSON is the JSON Schema for ReadMcpResourceTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "server": {
            "type": "string",
            "description": "The name of the MCP server providing the resource"
        },
        "uri": {
            "type": "string",
            "description": "The URI of the resource to read"
        }
    },
    "required": ["server", "uri"]
}`

package lsp

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Queries a Language Server Protocol (LSP) server for code intelligence.

Supports diagnostics, go-to-definition, find-references, and hover information. Currently a stub -- use Grep and Read tools for code navigation.`

// inputSchemaJSON is the JSON Schema for LSPTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "action": {
            "type": "string",
            "enum": ["diagnostics", "definition", "references", "hover"],
            "description": "The LSP action to perform"
        },
        "file": {
            "type": "string",
            "description": "The file path to query"
        },
        "line": {
            "type": "integer",
            "description": "The line number (0-indexed)"
        },
        "character": {
            "type": "integer",
            "description": "The character offset (0-indexed)"
        }
    },
    "required": ["action", "file", "line", "character"]
}`

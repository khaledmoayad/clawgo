package write

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Writes a file to the local filesystem.

Usage:
- This tool will overwrite the existing file if there is one at the provided path.
- ALWAYS use the Read tool first to read the file's contents before overwriting.
- If writing to an existing file, you MUST read it first.
- Prefer the Edit tool for modifying existing files -- it only sends the diff.`

// inputSchemaJSON is the JSON Schema for WriteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "file_path": {
            "type": "string",
            "description": "The absolute path to the file to write (must be absolute, not relative)"
        },
        "content": {
            "type": "string",
            "description": "The content to write to the file"
        }
    },
    "required": ["file_path", "content"]
}`

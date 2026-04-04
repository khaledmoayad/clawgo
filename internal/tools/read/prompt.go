package read

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Reads a file from the local filesystem. You can access any file directly by using this tool.

Usage:
- The file_path parameter must be an absolute path, not a relative path
- By default, it reads up to 2000 lines starting from the beginning of the file
- When you already know which part of the file you need, only read that part using offset and limit
- Results are returned using cat -n format, with line numbers starting at 1`

// inputSchemaJSON is the JSON Schema for ReadTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "file_path": {
            "type": "string",
            "description": "The absolute path to the file to read"
        },
        "offset": {
            "type": "integer",
            "description": "The line number to start reading from (0-indexed). Only provide if the file is too large to read at once",
            "minimum": 0
        },
        "limit": {
            "type": "integer",
            "description": "The number of lines to read. Only provide if the file is too large to read at once",
            "exclusiveMinimum": 0
        }
    },
    "required": ["file_path"]
}`

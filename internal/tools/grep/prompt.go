package grep

const toolDescription = `A powerful search tool built on ripgrep.

Usage:
- Supports full regex syntax (e.g., "log.*Error", "function\\s+\\w+")
- Filter files with include parameter (e.g., "*.js", "**/*.tsx")
- Use this for searching file contents across the project`

const inputSchemaJSON = `{
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "The regular expression pattern to search for"
    },
    "path": {
      "type": "string",
      "description": "The directory to search in (defaults to working directory)"
    },
    "include": {
      "type": "string",
      "description": "Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\")"
    },
    "context": {
      "type": "integer",
      "description": "Number of context lines to show before and after each match"
    },
    "max_results": {
      "type": "integer",
      "description": "Maximum number of results to return (default 250)"
    }
  },
  "required": ["pattern"]
}`

package webfetch

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Fetches content from a URL and converts it to markdown. Takes a URL and a prompt describing what to extract. Returns the processed content. Use for reading web pages, documentation, and API responses.`

// inputSchemaJSON is the JSON Schema for WebFetchTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "url": {
            "type": "string",
            "description": "The URL to fetch"
        },
        "prompt": {
            "type": "string",
            "description": "What information to extract from the page"
        }
    },
    "required": ["url", "prompt"]
}`

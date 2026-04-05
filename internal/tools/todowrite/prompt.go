package todowrite

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Use this tool to create and manage a todo list for tracking tasks during complex workflows. Each call REPLACES the full todo list. Each item has content, status (pending/in_progress/completed), and activeForm (the currently active phrasing of the task).`

// inputSchemaJSON is the JSON Schema for TodoWriteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "todos": {
            "type": "array",
            "description": "The complete updated todo list (replaces any existing list)",
            "items": {
                "type": "object",
                "properties": {
                    "content": {
                        "type": "string",
                        "description": "Description of the todo task"
                    },
                    "status": {
                        "type": "string",
                        "enum": ["pending", "in_progress", "completed"],
                        "description": "Current status of the todo"
                    },
                    "activeForm": {
                        "type": "string",
                        "description": "The currently active phrasing of this task"
                    }
                },
                "required": ["content", "status", "activeForm"]
            }
        }
    },
    "required": ["todos"]
}`

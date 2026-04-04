package todowrite

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Use this tool to create and manage a todo list for tracking tasks during complex workflows. Todos are persisted to .claude/todos.json in the project root. Each todo has an id, content, status (pending/in_progress/done), and priority (high/medium/low).`

// inputSchemaJSON is the JSON Schema for TodoWriteTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "todos": {
            "type": "array",
            "description": "Array of todo items to create or update",
            "items": {
                "type": "object",
                "properties": {
                    "id": {
                        "type": "string",
                        "description": "Unique identifier for the todo"
                    },
                    "content": {
                        "type": "string",
                        "description": "Description of the todo task"
                    },
                    "status": {
                        "type": "string",
                        "enum": ["pending", "in_progress", "done"],
                        "description": "Current status of the todo"
                    },
                    "priority": {
                        "type": "string",
                        "enum": ["high", "medium", "low"],
                        "description": "Priority level"
                    }
                },
                "required": ["id", "content", "status"]
            }
        }
    },
    "required": ["todos"]
}`

package askuser

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Ask the user a question and wait for their response. Use this tool to present the user with 1-4 multiple-choice questions, each with 2-4 options. The user can select from the provided options or type a custom response. Each question should have a short header label and clear options with descriptions.`

// inputSchemaJSON is the JSON Schema for AskUserTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "questions": {
            "type": "array",
            "description": "Questions to ask the user (1-4 questions)",
            "minItems": 1,
            "maxItems": 4,
            "items": {
                "type": "object",
                "properties": {
                    "question": {
                        "type": "string",
                        "description": "The complete question to ask the user"
                    },
                    "header": {
                        "type": "string",
                        "description": "Very short label displayed as a chip/tag (max 20 chars)"
                    },
                    "options": {
                        "type": "array",
                        "description": "Available choices (2-4 options)",
                        "minItems": 2,
                        "maxItems": 4,
                        "items": {
                            "type": "object",
                            "properties": {
                                "label": {
                                    "type": "string",
                                    "description": "Display text for this option (1-5 words)"
                                },
                                "description": {
                                    "type": "string",
                                    "description": "Explanation of what this option means"
                                },
                                "preview": {
                                    "type": "string",
                                    "description": "Optional preview content rendered when focused"
                                }
                            },
                            "required": ["label", "description"]
                        }
                    },
                    "multiSelect": {
                        "type": "boolean",
                        "description": "Allow selecting multiple options",
                        "default": false
                    }
                },
                "required": ["question", "header", "options"]
            }
        },
        "answers": {
            "type": "object",
            "description": "User answers collected by the permission component"
        },
        "annotations": {
            "type": "object",
            "description": "Per-question annotations from the user"
        },
        "metadata": {
            "type": "object",
            "description": "Optional metadata for tracking",
            "properties": {
                "source": {"type": "string"}
            }
        }
    },
    "required": ["questions"]
}`

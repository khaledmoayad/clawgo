package notebookedit

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Edits Jupyter notebook (.ipynb) files. Can add, edit, or delete cells. Supports code and markdown cell types.`

// inputSchemaJSON is the JSON Schema for NotebookEditTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "path": {
            "type": "string",
            "description": "Path to the .ipynb notebook file"
        },
        "command": {
            "type": "string",
            "description": "The operation to perform on the notebook",
            "enum": ["add_cell", "edit_cell", "delete_cell", "insert_cell"]
        },
        "cell_type": {
            "type": "string",
            "description": "Type of cell (required for add_cell and insert_cell)",
            "enum": ["code", "markdown"]
        },
        "index": {
            "type": "integer",
            "description": "Cell index (required for edit_cell, delete_cell, insert_cell)"
        },
        "source": {
            "type": "string",
            "description": "Cell content (required for add_cell, edit_cell, insert_cell)"
        }
    },
    "required": ["path", "command"]
}`

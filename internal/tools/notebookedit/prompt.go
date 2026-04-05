package notebookedit

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Completely replaces the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. The cell_number is 0-indexed. Use edit_mode=insert to add a new cell at the index specified by cell_number. Use edit_mode=delete to delete the cell at the index specified by cell_number.`

// inputSchemaJSON is the JSON Schema for NotebookEditTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "notebook_path": {
            "type": "string",
            "description": "The absolute path to the Jupyter notebook file"
        },
        "cell_number": {
            "type": "integer",
            "description": "The 0-indexed cell number to edit, insert at, or delete",
            "minimum": 0
        },
        "new_source": {
            "type": "string",
            "description": "The new source content for the cell"
        },
        "cell_type": {
            "type": "string",
            "description": "Type of cell (for insert mode)",
            "enum": ["code", "markdown"]
        },
        "edit_mode": {
            "type": "string",
            "description": "The edit operation to perform",
            "enum": ["replace", "insert", "delete"]
        }
    },
    "required": ["notebook_path", "cell_number", "new_source"]
}`

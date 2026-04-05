package notebookedit

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Completely replaces the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. Use edit_mode=insert to add a new cell after the cell identified by cell_id. Use edit_mode=delete to remove the cell identified by cell_id.`

// inputSchemaJSON is the JSON Schema for NotebookEditTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "notebook_path": {
            "type": "string",
            "description": "The absolute path to the Jupyter notebook file to edit (must be absolute, not relative)"
        },
        "cell_id": {
            "type": "string",
            "description": "The ID of the cell to edit. When inserting, the new cell is inserted after this cell. Omit or leave empty to insert at the beginning."
        },
        "new_source": {
            "type": "string",
            "description": "The new source for the cell"
        },
        "cell_type": {
            "type": "string",
            "description": "The type of the cell (code or markdown). Defaults to current cell type for replace, required for insert.",
            "enum": ["code", "markdown"]
        },
        "edit_mode": {
            "type": "string",
            "description": "The type of edit to make. Defaults to replace.",
            "enum": ["replace", "insert", "delete"]
        }
    },
    "required": ["notebook_path", "new_source"]
}`

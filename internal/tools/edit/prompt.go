package edit

const toolDescription = `Performs a string replacement edit on a file.

The tool replaces the first occurrence of old_str with new_str in the specified file. The old_str must match EXACTLY one location in the file (including whitespace and indentation). If old_str matches multiple locations, the edit is rejected for safety.

Special case: If old_str is empty, the entire new_str is written as a new file (creating it if needed).`

const inputSchemaJSON = `{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path to the file to edit"
    },
    "old_str": {
      "type": "string",
      "description": "The exact string to replace (must match exactly one location). Empty string means create new file."
    },
    "new_str": {
      "type": "string",
      "description": "The replacement string"
    }
  },
  "required": ["file_path", "old_str", "new_str"]
}`

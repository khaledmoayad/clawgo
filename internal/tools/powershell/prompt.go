package powershell

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Executes a PowerShell command and returns its output.

Uses powershell on Windows or pwsh on Linux/macOS. For cross-platform scripting when bash is not available.`

// inputSchemaJSON is the JSON Schema for PowerShellTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "command": {
            "type": "string",
            "description": "The PowerShell command to execute"
        },
        "timeout": {
            "type": "integer",
            "description": "Optional timeout in milliseconds (max 600000)"
        }
    },
    "required": ["command"]
}`

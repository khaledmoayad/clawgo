package teamcreate

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Creates a new team of agents for collaborative work.

Teams enable coordinated multi-agent workflows with leader-worker patterns. All running workers in the team can be cancelled by deleting the team.`

// inputSchemaJSON is the JSON Schema for TeamCreateTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "name": {
            "type": "string",
            "description": "The name for the new team"
        },
        "description": {
            "type": "string",
            "description": "Optional description of the team's purpose"
        }
    },
    "required": ["name"]
}`

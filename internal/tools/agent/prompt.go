package agent

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Launch a sub-agent to handle a complex task. The sub-agent has its own conversation context with Claude and can use all available tools. Use for tasks that require multi-step reasoning or focused work on a specific subtask. In coordinator mode, 'worker' type sub-agents run asynchronously in the background.`

// inputSchemaJSON is the JSON Schema for AgentTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "prompt": {
            "type": "string",
            "description": "The task for the sub-agent"
        },
        "model": {
            "type": "string",
            "description": "Model override for the sub-agent"
        },
        "permitted_tools": {
            "type": "array",
            "items": {"type": "string"},
            "description": "Tools the sub-agent is allowed to use"
        },
        "subagent_type": {
            "type": "string",
            "enum": ["worker", "subagent"],
            "description": "Type of sub-agent. 'worker' for async background execution in coordinator mode. 'subagent' for standard synchronous sub-agent."
        }
    },
    "required": ["prompt"]
}`

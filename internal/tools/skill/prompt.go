package skill

// toolDescription is the human-readable description sent to the Anthropic API.
const toolDescription = `Load a skill file by name. Skills provide specialized knowledge and instructions for specific tasks. Searches .claude/skills/ in the project root and user config directory. Returns the skill content or a list of available skills if the requested one is not found.`

// inputSchemaJSON is the JSON Schema for SkillTool input.
const inputSchemaJSON = `{
    "type": "object",
    "properties": {
        "name": {
            "type": "string",
            "description": "Skill name to load"
        }
    },
    "required": ["name"]
}`

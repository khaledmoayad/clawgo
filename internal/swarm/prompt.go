package swarm

import "strings"

// TeammateSystemPromptAddendum is appended to the main system prompt for
// teammates. It explains visibility constraints and communication requirements.
// This matches the TypeScript TEAMMATE_SYSTEM_PROMPT_ADDENDUM from
// utils/swarm/teammatePromptAddendum.ts.
const TeammateSystemPromptAddendum = `
# Agent Teammate Communication

IMPORTANT: You are running as an agent in a team. To communicate with anyone on your team:
- Use the SendMessage tool with ` + "`" + `to: "<name>"` + "`" + ` to send messages to specific teammates
- Use the SendMessage tool with ` + "`" + `to: "*"` + "`" + ` sparingly for team-wide broadcasts

Just writing a response in text is not visible to others on your team - you MUST use the SendMessage tool.

The user interacts primarily with the team lead. Your work is coordinated through the task system and teammate messaging.
`

// BuildTeammateSystemPrompt constructs the system prompt for a teammate based
// on the SystemPromptMode. This matches the TypeScript inProcessRunner.ts
// system prompt resolution logic:
//
//   - "replace": Use only the custom systemPrompt, ignoring the default.
//   - "append": Use the default prompt + teammate addendum + custom systemPrompt.
//   - "default" (or empty): Use the default prompt + teammate addendum.
//
// The defaultPromptParts are the sections from the main agent's system prompt
// (same prompt the leader uses). The teammate addendum is always included
// unless the mode is "replace".
//
// If an agentDefinitionPrompt is provided (from custom agent definitions in
// .claude/agents/), it is appended after the addendum with a header.
func BuildTeammateSystemPrompt(
	mode SystemPromptMode,
	customPrompt string,
	defaultPromptParts []string,
	agentDefinitionPrompt string,
) string {
	// Replace mode: use only the custom prompt, no addendum
	if mode == SystemPromptReplace && customPrompt != "" {
		return customPrompt
	}

	// Build from default parts + addendum
	parts := make([]string, 0, len(defaultPromptParts)+3)
	parts = append(parts, defaultPromptParts...)
	parts = append(parts, TeammateSystemPromptAddendum)

	// Custom agent definition prompt (from .claude/agents/ directory)
	if agentDefinitionPrompt != "" {
		parts = append(parts, "\n# Custom Agent Instructions\n"+agentDefinitionPrompt)
	}

	// Append mode: add custom prompt after default + addendum
	if mode == SystemPromptAppend && customPrompt != "" {
		parts = append(parts, customPrompt)
	}

	return strings.Join(parts, "\n")
}

// TeamEssentialTools returns the tool names that must always be available to
// teammates for coordination, even when explicit tool lists are provided.
// This matches the TypeScript inProcessRunner.ts injection of team-essential
// tools into custom agent definitions.
func TeamEssentialTools() []string {
	return []string{
		"SendMessage",
		"TeamCreate",
		"TeamDelete",
		"TaskCreate",
		"TaskGet",
		"TaskList",
		"TaskUpdate",
	}
}

// MergeToolPermissions merges an explicit tool list with team-essential tools.
// If allowedTools is empty/nil, returns ["*"] (all tools).
// Otherwise, returns the union of allowedTools and TeamEssentialTools().
func MergeToolPermissions(allowedTools []string) []string {
	if len(allowedTools) == 0 {
		return []string{"*"}
	}

	// Build set from allowed tools
	seen := make(map[string]bool, len(allowedTools)+len(TeamEssentialTools()))
	result := make([]string, 0, len(allowedTools)+len(TeamEssentialTools()))

	for _, t := range allowedTools {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}

	for _, t := range TeamEssentialTools() {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}

	return result
}

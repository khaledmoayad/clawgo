package memory

import "fmt"

// DefaultSessionMemoryTemplate is the template for session memory extraction.
// Each section captures a different aspect of the session context that would
// help continue the work in a future session. Matches the TypeScript
// SessionMemory/prompts.ts template structure.
const DefaultSessionMemoryTemplate = `# Session Title
_A short and distinctive 5-10 word descriptive title for the session._

# Current State
_What is actively being worked on right now?_

# Task specification
_What did the user ask to build?_

# Files and Functions
_What are the important files?_

# Workflow
_What bash commands are usually run?_

# Errors & Corrections
_Errors encountered and how they were fixed._

# Codebase and System Documentation
_What are the important system components?_
`

// MemoryUpdatePrompt is the system prompt for the memory extraction API call.
// It instructs Claude to analyze the conversation and fill in the template sections.
const MemoryUpdatePrompt = `You are updating session memory notes. Analyze the conversation and fill in each section of the template with relevant information. Be concise but thorough. Focus on information that would help continue this work in a future session.

Output ONLY the filled-in template with no additional commentary. Keep each section focused and actionable. If a section has no relevant information from the conversation, write "N/A" for that section.`

// FormatMemoryPrompt combines the update prompt with the template and any
// existing memory content to create the full user prompt for the extraction
// API call.
func FormatMemoryPrompt(template string, existingMemory string) string {
	if existingMemory != "" {
		return fmt.Sprintf(
			"Here is the template to fill in:\n\n%s\n\nHere is the existing memory from a previous session (update it with new information, don't lose important existing context):\n\n%s",
			template, existingMemory,
		)
	}
	return fmt.Sprintf("Here is the template to fill in:\n\n%s", template)
}

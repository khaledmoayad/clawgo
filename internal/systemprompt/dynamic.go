// Package systemprompt implements system prompt construction for ClawGo,
// matching Claude Code's multi-section system prompt architecture.
package systemprompt

import (
	"fmt"
	"os"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/claudemd"
)

// McpServerInstruction represents instructions from a connected MCP server.
type McpServerInstruction struct {
	ServerName   string
	Instructions string
}

// GetLanguageSection returns a system prompt section instructing the model
// to respond in a specific language. Returns empty string if language is empty.
//
// Matches Claude Code's getLanguageSection() from constants/prompts.ts.
func GetLanguageSection(language string) string {
	if language == "" {
		return ""
	}
	return fmt.Sprintf(`# Language
Always respond in %s. Use %s for all explanations, comments, and communications with the user. Technical terms and code identifiers should remain in their original form.`, language, language)
}

// GetOutputStyleSection returns a system prompt section with custom output
// style instructions. Returns empty string if name is empty.
//
// Matches Claude Code's getOutputStyleSection() from constants/prompts.ts.
func GetOutputStyleSection(name string, prompt string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf("# Output Style: %s\n%s", name, prompt)
}

// GetMcpInstructionsSection returns a system prompt section with MCP server
// instructions. Returns empty string if there are no servers with instructions.
//
// CACHE-BREAKING: This section must be recomputed every turn, never cached.
// MCP servers connect/disconnect between turns, so the instructions can change
// at any time. Any section placed after MCP instructions in the prompt array
// also cannot be cached (SYS-09).
//
// Matches Claude Code's getMcpInstructionsSection() / getMcpInstructions()
// from constants/prompts.ts.
func GetMcpInstructionsSection(servers []McpServerInstruction) string {
	if len(servers) == 0 {
		return ""
	}

	// Filter to only servers that have instructions
	var blocks []string
	for _, s := range servers {
		if s.Instructions != "" {
			blocks = append(blocks, fmt.Sprintf("## %s\n%s", s.ServerName, s.Instructions))
		}
	}

	if len(blocks) == 0 {
		return ""
	}

	return fmt.Sprintf(`# MCP Server Instructions

The following MCP servers have provided instructions for how to use their tools and resources:

%s`, strings.Join(blocks, "\n\n"))
}

// GetScratchpadSection returns a system prompt section with scratchpad directory
// instructions. Returns empty string if dir is empty.
//
// Matches Claude Code's getScratchpadInstructions() from constants/prompts.ts.
func GetScratchpadSection(dir string) string {
	if dir == "" {
		return ""
	}

	return fmt.Sprintf(`# Scratchpad Directory

IMPORTANT: Always use this scratchpad directory for temporary files instead of `+"`/tmp`"+` or other system temp directories:
`+"`%s`"+`

Use this directory for ALL temporary file needs:
- Storing intermediate results or data during multi-step tasks
- Writing temporary scripts or configuration files
- Saving outputs that don't belong in the user's project
- Creating working files during analysis or processing
- Any file that would otherwise go to `+"`/tmp`"+`

Only use `+"`/tmp`"+` if the user explicitly requests it.

The scratchpad directory is session-specific, isolated from the user's project, and can be used freely without permission prompts.`, dir)
}

// GetFunctionResultClearingSection returns a system prompt section explaining
// the function result clearing lifecycle. This is gated by the
// CACHED_MICROCOMPACT feature flag (passed as enabled parameter).
// Returns empty string if not enabled.
//
// Matches Claude Code's getFunctionResultClearingSection() from constants/prompts.ts.
func GetFunctionResultClearingSection(enabled bool) string {
	if !enabled {
		return ""
	}
	return `# Function Result Clearing

Old tool results will be automatically cleared from context to free up space. The most recent results are always kept.`
}

// SummarizeToolResultsSection is a constant prompt section instructing the
// model to write down important information from tool results.
//
// Matches Claude Code's SUMMARIZE_TOOL_RESULTS_SECTION from constants/prompts.ts.
const SummarizeToolResultsSection = `When working with tool results, write down any important information you might need later in your response, as the original tool result may be cleared later.`

// GetTokenBudgetSection returns a system prompt section with token budget
// management instructions. This is gated by the TOKEN_BUDGET feature flag
// (passed as enabled parameter). Returns empty string if not enabled.
//
// Matches Claude Code's token_budget section from constants/prompts.ts.
func GetTokenBudgetSection(enabled bool) string {
	if !enabled {
		return ""
	}
	return `When the user specifies a token target (e.g., "+500k", "spend 2M tokens", "use 1B tokens"), your output token count will be shown each turn. Keep working until you approach the target — plan your work to fill it productively. The target is a hard minimum, not a suggestion. If you stop early, the system will automatically continue you.`
}

// GetBriefModeSection returns a system prompt section with brief mode
// instructions. This is gated by the KAIROS/KAIROS_BRIEF feature flags
// (passed as enabled parameter). Returns empty string if not enabled.
//
// The actual brief mode prompt text will be loaded from the BriefTool
// module when the feature system is wired in Phase 13.
func GetBriefModeSection(enabled bool) string {
	if !enabled {
		return ""
	}
	// Placeholder for the BRIEF_PROACTIVE_SECTION content that will be
	// loaded from the brief tool module when feature flags are wired.
	return `# Brief Mode

When brief mode is enabled, keep your responses extremely concise. Focus on actions and results, not explanations. Use bullet points over prose. Skip preamble and transitions.`
}

// LoadMemoryPromptSection loads CLAUDE.md content from the project directory
// hierarchy and returns it as a single string suitable for inclusion in the
// system prompt. Returns empty string if no CLAUDE.md files are found.
//
// This uses the claudemd.LoadMemoryFiles() function to discover and load
// CLAUDE.md files in priority order, matching Claude Code's loadMemoryPrompt()
// pattern from memdir/memdir.ts.
func LoadMemoryPromptSection(workDir, homeDir string) string {
	files, err := claudemd.LoadMemoryFiles(workDir, homeDir)
	if err != nil || len(files) == 0 {
		return ""
	}

	var parts []string
	for _, f := range files {
		if f.Content != "" {
			parts = append(parts, f.Content)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}

// writeFile is a helper for tests.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

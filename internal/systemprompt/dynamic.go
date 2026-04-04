package systemprompt

import "os"

// McpServerInstruction represents instructions from a connected MCP server.
type McpServerInstruction struct {
	ServerName   string
	Instructions string
}

// GetLanguageSection stub - not yet implemented.
func GetLanguageSection(language string) string { return "" }

// GetOutputStyleSection stub - not yet implemented.
func GetOutputStyleSection(name string, prompt string) string { return "" }

// GetMcpInstructionsSection stub - not yet implemented.
// CACHE-BREAKING: This section must be recomputed every turn, never cached.
func GetMcpInstructionsSection(servers []McpServerInstruction) string { return "" }

// GetScratchpadSection stub - not yet implemented.
func GetScratchpadSection(dir string) string { return "" }

// GetFunctionResultClearingSection stub - not yet implemented.
func GetFunctionResultClearingSection(enabled bool) string { return "" }

// SummarizeToolResultsSection stub.
const SummarizeToolResultsSection = ""

// GetTokenBudgetSection stub - not yet implemented.
func GetTokenBudgetSection(enabled bool) string { return "" }

// GetBriefModeSection stub - not yet implemented.
func GetBriefModeSection(enabled bool) string { return "" }

// LoadMemoryPromptSection stub - not yet implemented.
func LoadMemoryPromptSection(workDir, homeDir string) string { return "" }

// writeFile is a helper for tests.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

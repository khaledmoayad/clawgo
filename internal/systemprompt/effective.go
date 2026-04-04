package systemprompt

// EffectivePromptConfig holds configuration for building the effective
// system prompt with priority resolution.
type EffectivePromptConfig struct {
	// Priority chain (highest to lowest)
	OverridePrompt    string // --system-prompt flag or API override
	CoordinatorPrompt string // Coordinator mode prompt
	AgentPrompt       string // Agent-specific prompt
	CustomPrompt      string // Custom prompt from settings
	AppendPrompt      string // --append-system-prompt (always appended)

	// Base prompt config (used when no override)
	BaseConfig SystemPromptConfig

	// Dynamic section inputs
	Language        string
	OutputStyle     string
	OutputStyleName string
	McpInstructions []McpServerInstruction
	ScratchpadDir   string
	MemoryWorkDir   string
	MemoryHomeDir   string

	// Feature flags (SYS-08)
	CachedMicrocompact   bool
	TokenBudgetEnabled   bool
	TokenBudgetRemaining int
	BriefModeEnabled     bool
	ProactiveMode        bool
}

// SystemPromptConfig stub for base config.
type SystemPromptConfig struct{}

// BuildEffectiveSystemPrompt stub - not yet implemented.
func BuildEffectiveSystemPrompt(cfg EffectivePromptConfig) []string {
	return nil
}

// ResolveDynamicSections stub - not yet implemented.
func ResolveDynamicSections(cfg EffectivePromptConfig) []string {
	return nil
}

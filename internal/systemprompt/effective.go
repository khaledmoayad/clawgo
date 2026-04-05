package systemprompt

// EffectivePromptConfig holds configuration for building the effective
// system prompt with priority resolution.
//
// The priority chain determines which prompt serves as the "base":
//
//	override > coordinator > agent > custom > default
//
// AppendPrompt is always added at the end (except when OverridePrompt is set,
// which replaces everything including append).
//
// Matches Claude Code's buildEffectiveSystemPrompt() from utils/systemPrompt.ts.
type EffectivePromptConfig struct {
	// Priority chain (highest to lowest)
	OverridePrompt    string // --system-prompt flag or API override
	CoordinatorPrompt string // Coordinator mode prompt
	AgentPrompt       string // Agent-specific prompt
	CustomPrompt      string // Custom prompt from settings
	AppendPrompt      string // --append-system-prompt (always appended)

	// Base config for the default system prompt path (env info, simple mode, etc.)
	BaseConfig SystemPromptConfig

	// Dynamic section inputs
	Language        string
	OutputStyle     string
	OutputStyleName string
	McpInstructions []McpServerInstruction
	ScratchpadDir   string
	MemoryWorkDir   string
	MemoryHomeDir   string

	// Feature flags (SYS-08) -- boolean parameters for now,
	// will be wired to GrowthBook client in Phase 13.
	CachedMicrocompact   bool
	TokenBudgetEnabled   bool
	TokenBudgetRemaining int
	BriefModeEnabled     bool
	ProactiveMode        bool
}

// BuildEffectiveSystemPrompt resolves the system prompt using priority logic
// and appends dynamic sections.
//
// Priority resolution:
//  1. OverridePrompt set -> returns ONLY the override (highest priority, no append)
//  2. CoordinatorPrompt set -> coordinator + dynamic + append
//  3. AgentPrompt set -> agent + dynamic + append
//  4. CustomPrompt set -> custom + dynamic + append
//  5. Default -> static sections + dynamic + append
//
// Dynamic sections are always included after the base (except for override).
// MCP instructions are always last in the dynamic sections (SYS-09 cache-breaking).
//
// Matches Claude Code's buildEffectiveSystemPrompt() from utils/systemPrompt.ts.
func BuildEffectiveSystemPrompt(cfg EffectivePromptConfig) []string {
	// Override replaces everything -- no append, no dynamic sections
	if cfg.OverridePrompt != "" {
		return []string{cfg.OverridePrompt}
	}

	// Simple mode returns a minimal prompt (via BaseConfig)
	if cfg.BaseConfig.SimpleMode {
		return GetSystemPrompt(cfg.BaseConfig)
	}

	var base []string

	switch {
	case cfg.CoordinatorPrompt != "":
		base = []string{cfg.CoordinatorPrompt}
	case cfg.AgentPrompt != "":
		base = []string{cfg.AgentPrompt}
	case cfg.CustomPrompt != "":
		base = []string{cfg.CustomPrompt}
	default:
		// Default path: use the static section generators with BaseConfig
		base = GetStaticSections(cfg.BaseConfig)
	}

	// Boundary marker for prompt caching (SYS-07)
	if cfg.BaseConfig.UseGlobalCache {
		base = append(base, DynamicBoundaryMarker)
	}

	// Append dynamic sections (env info, memory, language, MCP, etc.)
	dynamic := ResolveDynamicSections(cfg)
	result := make([]string, 0, len(base)+len(dynamic)+1)
	result = append(result, base...)
	result = append(result, dynamic...)

	// AppendPrompt is always added at the end (when not empty)
	if cfg.AppendPrompt != "" {
		result = append(result, cfg.AppendPrompt)
	}

	return result
}

// Note: getDefaultSections() was removed -- BuildEffectiveSystemPrompt now
// delegates to GetStaticSections(cfg.BaseConfig) for the default path,
// which respects KeepCodingInstr and other BaseConfig options.

// ResolveDynamicSections assembles the dynamic system prompt sections from
// the config. These sections are session-specific and change per turn.
//
// Important: MCP instructions are ALWAYS last in the dynamic sections array
// (SYS-09 cache-breaking requirement). Any section placed after MCP instructions
// also won't benefit from prompt caching because the MCP section changes
// whenever servers connect/disconnect between turns.
//
// Section ordering:
//  1. Session guidance
//  2. Memory (CLAUDE.md)
//  3. Language preference
//  4. Output style
//  5. Scratchpad
//  6. Function result clearing (feature-gated)
//  7. Summarize tool results
//  8. Token budget (feature-gated)
//  9. Brief mode (feature-gated)
//  10. MCP instructions (LAST -- cache-breaking)
func ResolveDynamicSections(cfg EffectivePromptConfig) []string {
	var sections []string

	// Session guidance
	if guidance := GetSessionGuidanceSection(); guidance != "" {
		sections = append(sections, guidance)
	}

	// Environment info (platform, shell, model, git, working dir)
	if envInfo := ComputeEnvInfo(cfg.BaseConfig.EnvInfo); envInfo != "" {
		sections = append(sections, envInfo)
	}

	// Memory (CLAUDE.md content)
	if mem := LoadMemoryPromptSection(cfg.MemoryWorkDir, cfg.MemoryHomeDir); mem != "" {
		sections = append(sections, mem)
	}

	// Language preference
	if lang := GetLanguageSection(cfg.Language); lang != "" {
		sections = append(sections, lang)
	}

	// Output style
	if style := GetOutputStyleSection(cfg.OutputStyleName, cfg.OutputStyle); style != "" {
		sections = append(sections, style)
	}

	// Scratchpad
	if sp := GetScratchpadSection(cfg.ScratchpadDir); sp != "" {
		sections = append(sections, sp)
	}

	// Function result clearing (feature-gated: CACHED_MICROCOMPACT)
	if frc := GetFunctionResultClearingSection(cfg.CachedMicrocompact); frc != "" {
		sections = append(sections, frc)
	}

	// Summarize tool results (always included)
	sections = append(sections, SummarizeToolResultsSection)

	// Token budget (feature-gated: TOKEN_BUDGET)
	if tb := GetTokenBudgetSection(cfg.TokenBudgetEnabled); tb != "" {
		sections = append(sections, tb)
	}

	// Brief mode (feature-gated: KAIROS/KAIROS_BRIEF)
	if bm := GetBriefModeSection(cfg.BriefModeEnabled); bm != "" {
		sections = append(sections, bm)
	}

	// MCP instructions MUST be last (SYS-09 cache-breaking requirement)
	if mcp := GetMcpInstructionsSection(cfg.McpInstructions); mcp != "" {
		sections = append(sections, mcp)
	}

	return sections
}

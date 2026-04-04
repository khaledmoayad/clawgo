package systemprompt

// DynamicBoundaryMarker separates static (cross-org cacheable) content
// from dynamic content. Everything BEFORE this marker in the system
// prompt array can use scope: 'global'. Everything AFTER contains
// user/session-specific content and should not be cached.
//
// Matches SYSTEM_PROMPT_DYNAMIC_BOUNDARY from constants/prompts.ts.
const DynamicBoundaryMarker = "__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__"

// SystemPromptConfig configures the system prompt assembly.
type SystemPromptConfig struct {
	// EnvInfo provides environment-specific data for the env section.
	EnvInfo EnvInfoConfig

	// KeepCodingInstr corresponds to outputStyleConfig.keepCodingInstructions.
	// When false, the DoingTasks section is omitted.
	KeepCodingInstr bool

	// UseGlobalCache corresponds to shouldUseGlobalCacheScope().
	// When true, inserts DynamicBoundaryMarker between static and dynamic sections.
	UseGlobalCache bool

	// SimpleMode corresponds to CLAUDE_CODE_SIMPLE env var.
	// When true, returns a minimal 3-section prompt.
	SimpleMode bool
}

// GetSystemPrompt assembles the full system prompt as an ordered slice
// of string sections. This is the main entry point matching
// getSystemPrompt() from constants/prompts.ts.
//
// In SimpleMode, returns a minimal 3-section prompt.
// Otherwise, assembles static sections (cacheable), optionally inserts
// DynamicBoundaryMarker, then dynamic sections.
func GetSystemPrompt(cfg SystemPromptConfig) []string {
	if cfg.SimpleMode {
		return getSimplePrompt(cfg)
	}

	var sections []string

	// Static sections (cacheable across all users/sessions)
	sections = append(sections, GetStaticSections(cfg)...)

	// Boundary marker for prompt caching (SYS-07)
	if cfg.UseGlobalCache {
		sections = append(sections, DynamicBoundaryMarker)
	}

	// Dynamic sections (user/session-specific)
	sections = append(sections, GetDynamicSections(cfg)...)

	return sections
}

// GetStaticSections returns just the static (cacheable) sections in order.
// These are identical across all users/sessions and safe for global caching.
func GetStaticSections(cfg SystemPromptConfig) []string {
	sections := []string{
		GetIntroSection(),
		GetSystemSection(),
	}

	// DoingTasks is omitted when an output style has keepCodingInstructions=false
	if cfg.KeepCodingInstr {
		sections = append(sections, GetDoingTasksSection())
	}

	sections = append(sections,
		GetActionsSection(),
		GetUsingToolsSection(),
		GetToneStyleSection(),
		GetOutputEfficiencySection(),
	)

	return sections
}

// GetDynamicSections returns just the dynamic (session-specific) sections.
// These vary per user/session and must not be globally cached.
func GetDynamicSections(cfg SystemPromptConfig) []string {
	var sections []string

	guidance := GetSessionGuidanceSection()
	if guidance != "" {
		sections = append(sections, guidance)
	}

	envInfo := ComputeEnvInfo(cfg.EnvInfo)
	if envInfo != "" {
		sections = append(sections, envInfo)
	}

	return sections
}

// getSimplePrompt returns a minimal prompt for CLAUDE_CODE_SIMPLE mode.
// Matches the early return in getSystemPrompt() from prompts.ts.
func getSimplePrompt(cfg SystemPromptConfig) []string {
	return []string{
		"You are Claude Code, Anthropic's official CLI for Claude.",
		"CWD: " + cfg.EnvInfo.WorkDir,
		"Date: " + cfg.EnvInfo.KnowledgeCutoff,
	}
}

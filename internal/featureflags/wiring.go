// Package featureflags wiring defines known feature flag constants matching
// Claude Code's GrowthBook feature gates, plus convenience functions for
// commonly checked flags.
package featureflags

// Known feature flag keys. These constants match the tengu_* keys used
// throughout the TypeScript Claude Code codebase via
// getFeatureValue_CACHED_MAY_BE_STALE and checkStatsigFeatureGate_CACHED_MAY_BE_STALE.
const (
	// Agent and background features
	FlagAgentListAttach      = "tengu_agent_list_attach"
	FlagAutoBackgroundAgents = "tengu_auto_background_agents"
	FlagSurrealDali          = "tengu_surreal_dali"

	// Amber flags (various experiments)
	FlagAmberFlint         = "tengu_amber_flint"
	FlagAmberJSONTools     = "tengu_amber_json_tools"
	FlagAmberPrism         = "tengu_amber_prism"
	FlagAmberQuartzDisabled = "tengu_amber_quartz_disabled"
	FlagAmberStoat         = "tengu_amber_stoat"

	// Attribution and telemetry
	FlagAttributionHeader = "tengu_attribution_header"
	FlagTraceLantern      = "tengu_trace_lantern"

	// Bridge and remote features
	FlagBridgeReplV2    = "tengu_bridge_repl_v2"
	FlagBridgeSystemInit = "tengu_bridge_system_init"
	FlagCCRBridge       = "tengu_ccr_bridge"
	FlagCCRMirror       = "tengu_ccr_mirror"
	FlagCobaltHarbor    = "tengu_cobalt_harbor"
	FlagCobaltLantern   = "tengu_cobalt_lantern"
	FlagCopperBridge    = "tengu_copper_bridge"
	FlagRemoteBackend   = "tengu_remote_backend"

	// Context and message processing
	FlagBasalt3KR        = "tengu_basalt_3kr"
	FlagBirchTrellis     = "tengu_birch_trellis"
	FlagBrambleLintel    = "tengu_bramble_lintel"
	FlagChairSermon      = "tengu_chair_sermon"
	FlagChompInflection  = "tengu_chomp_inflection"
	FlagCobaltRaccoon    = "tengu_cobalt_raccoon"
	FlagHiveEvidence     = "tengu_hive_evidence"
	FlagSlimSubagentClaudeMD = "tengu_slim_subagent_claudemd"

	// Feature gating and permissions
	FlagDestructiveCommandWarning = "tengu_destructive_command_warning"
	FlagFGTS                      = "tengu_fgts"
	FlagHarbor                    = "tengu_harbor"
	FlagHarborPermissions         = "tengu_harbor_permissions"
	FlagPassportQuail             = "tengu_passport_quail"
	FlagScratch                   = "tengu_scratch"

	// Image and attachment features
	FlagCollageKaleidoscope = "tengu_collage_kaleidoscope"
	FlagMarbleFox           = "tengu_marble_fox"
	FlagMothCopse           = "tengu_moth_copse"

	// Mode and UI features
	FlagChromeAutoEnable = "tengu_chrome_auto_enable"
	FlagCoralFern        = "tengu_coral_fern"
	FlagGlacier2XR       = "tengu_glacier_2xr"
	FlagHerringClock     = "tengu_herring_clock"
	FlagImmediateModelCommand = "tengu_immediate_model_command"
	FlagJadeAnvil4       = "tengu_jade_anvil_4"
	FlagLapisFinch       = "tengu_lapis_finch"
	FlagLodestoneEnabled = "tengu_lodestone_enabled"
	FlagTerminalPanel    = "tengu_terminal_panel"
	FlagTerminalSidebar  = "tengu_terminal_sidebar"
	FlagWillowMode       = "tengu_willow_mode"

	// Memory and session features
	FlagSessionMemory    = "tengu_session_memory"
	FlagPebbleLeafPrune  = "tengu_pebble_leaf_prune"

	// Performance and optimization
	FlagCicadaNapMS      = "tengu_cicada_nap_ms"
	FlagMarbleSandcastle = "tengu_marble_sandcastle"
	FlagOTKSlotV1        = "tengu_otk_slot_v1"
	FlagQuartzLantern    = "tengu_quartz_lantern"
	FlagSlatePrism       = "tengu_slate_prism"
	FlagSlateThimble     = "tengu_slate_thimble"
	FlagStrapFoyer       = "tengu_strap_foyer"

	// Skill and hook features
	FlagCopperPanda = "tengu_copper_panda"

	// Special modes
	FlagKairosBrief    = "tengu_kairos_brief"
	FlagMiraculoTheBard = "tengu_miraculo_the_bard"
	FlagThinkback      = "tengu_thinkback"
	FlagToolPear       = "tengu_tool_pear"
	FlagTurtleCarbon   = "tengu_turtle_carbon"
	FlagUltraplanModel = "tengu_ultraplan_model"
)

// ---- Convenience boolean getters for commonly checked flags ----

// IsAttributionHeaderEnabled returns whether the attribution header is enabled.
func IsAttributionHeaderEnabled() bool { return GetBoolDefault(FlagAttributionHeader, true) }

// IsCCRBridgeEnabled returns whether the CCR bridge is enabled.
func IsCCRBridgeEnabled() bool { return GetBool(FlagCCRBridge) }

// IsCCRMirrorEnabled returns whether the CCR mirror is enabled.
func IsCCRMirrorEnabled() bool { return GetBool(FlagCCRMirror) }

// IsBridgeReplV2Enabled returns whether bridge REPL v2 is enabled.
func IsBridgeReplV2Enabled() bool { return GetBool(FlagBridgeReplV2) }

// IsBridgeSystemInitEnabled returns whether bridge system init is enabled.
func IsBridgeSystemInitEnabled() bool { return GetBool(FlagBridgeSystemInit) }

// IsSessionMemoryEnabled returns whether session memory extraction is enabled.
func IsSessionMemoryEnabled() bool { return GetBool(FlagSessionMemory) }

// IsCobaltRaccoonEnabled returns whether the cobalt raccoon context feature is enabled.
func IsCobaltRaccoonEnabled() bool { return GetBool(FlagCobaltRaccoon) }

// IsHiveEvidenceEnabled returns whether hive evidence is enabled.
func IsHiveEvidenceEnabled() bool { return GetBool(FlagHiveEvidence) }

// IsTerminalPanelEnabled returns whether the terminal panel is enabled.
func IsTerminalPanelEnabled() bool { return GetBool(FlagTerminalPanel) }

// IsTerminalSidebarEnabled returns whether the terminal sidebar is enabled.
func IsTerminalSidebarEnabled() bool { return GetBool(FlagTerminalSidebar) }

// IsThinkbackEnabled returns whether thinkback mode is enabled.
func IsThinkbackEnabled() bool { return GetBool(FlagThinkback) }

// IsScratchEnabled returns whether scratch/coordinator mode is enabled.
func IsScratchEnabled() bool { return GetBool(FlagScratch) }

// IsDestructiveCommandWarningEnabled returns whether destructive command warnings are enabled.
func IsDestructiveCommandWarningEnabled() bool { return GetBool(FlagDestructiveCommandWarning) }

// IsCobaltLanternEnabled returns whether cobalt lantern (remote setup) is enabled.
func IsCobaltLanternEnabled() bool { return GetBool(FlagCobaltLantern) }

// IsAutoBackgroundAgentsEnabled returns whether automatic background agents are enabled.
func IsAutoBackgroundAgentsEnabled() bool { return GetBool(FlagAutoBackgroundAgents) }

// IsTurtleCarbonEnabled returns whether the turtle carbon thinking feature is enabled.
func IsTurtleCarbonEnabled() bool { return GetBoolDefault(FlagTurtleCarbon, true) }

// IsMarbleSandcastleEnabled returns whether marble sandcastle fast mode is enabled.
func IsMarbleSandcastleEnabled() bool { return GetBool(FlagMarbleSandcastle) }

// IsCollageKaleidoscopeEnabled returns whether collage kaleidoscope image paste is enabled.
func IsCollageKaleidoscopeEnabled() bool { return GetBoolDefault(FlagCollageKaleidoscope, true) }

// GetWillowMode returns the willow mode string value ("off" by default).
func GetWillowMode() string { return GetStringDefault(FlagWillowMode, "off") }

// GetUltraplanModel returns the ultraplan model override string.
func GetUltraplanModel() string { return GetString(FlagUltraplanModel) }

// GetCicadaNapMS returns the cicada nap duration in milliseconds.
func GetCicadaNapMS() int { return GetInt(FlagCicadaNapMS) }

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// LoadSettings loads and merges settings from multiple sources in precedence order:
// 1. User: configDir/settings.json (lowest priority)
// 2. Project: projectRoot/.claude/settings.json
// 3. Enterprise/MDM: LoadMDMSettings() (highest priority in this variant)
//
// For full 7-tier precedence including .local and managed-settings.d, use LoadSettingsFull.
// Scalar fields are overridden by higher-priority sources.
// Slice fields (AllowedTools, DisallowedTools, etc.) are appended.
// Map fields (Env, KeyBindings, etc.) are merged per-key.
func LoadSettings(configDir, projectRoot string) (*Settings, error) {
	return LoadSettingsFull(configDir, projectRoot, nil)
}

// LoadSettingsFull loads and merges settings from all 7 tiers in precedence order:
// 1. User: configDir/settings.json (lowest priority)
// 2. User local: configDir/settings.local.json
// 3. Project: projectRoot/.claude/settings.json
// 4. Project local: projectRoot/.claude/settings.local.json
// 5. Enterprise/MDM: LoadMDMSettings()
// 6. Managed-settings.d: *.json files from managed settings directory
// 7. Remote-managed: remoteSettings parameter (highest priority)
//
// Scalar fields are overridden by higher-priority sources.
// Slice fields (AllowedTools, DisallowedTools, AllowedHTTPHookURLs, etc.) are appended.
// Map fields (Env, KeyBindings, ModelOverrides, EnabledPlugins) are merged per-key.
// json.RawMessage fields (Hooks, MCPServers, Plugins, Sandbox, etc.) are overridden entirely.
func LoadSettingsFull(configDir, projectRoot string, remoteSettings *Settings) (*Settings, error) {
	merged := &Settings{}

	// 1. Load user settings
	userPath := filepath.Join(configDir, "settings.json")
	if err := mergeSettingsFromFile(merged, userPath); err != nil {
		return nil, err
	}

	// 2. Load user local settings (higher priority than user settings)
	userLocalPath := filepath.Join(configDir, "settings.local.json")
	if err := mergeSettingsFromFile(merged, userLocalPath); err != nil {
		return nil, err
	}

	// 3. Load project settings
	if projectRoot != "" {
		projectPath := filepath.Join(projectRoot, ".claude", "settings.json")
		if err := mergeSettingsFromFile(merged, projectPath); err != nil {
			return nil, err
		}

		// 4. Load project local settings
		projectLocalPath := filepath.Join(projectRoot, ".claude", "settings.local.json")
		if err := mergeSettingsFromFile(merged, projectLocalPath); err != nil {
			return nil, err
		}
	}

	// 5. Load enterprise/MDM settings
	mdmSettings := LoadMDMSettings()
	mergeSettings(merged, mdmSettings)

	// 6. Load managed-settings.d directory
	if err := loadManagedSettingsDir(merged); err != nil {
		// Non-fatal: log but continue if managed settings directory is inaccessible
		_ = err
	}

	// 7. Load remote-managed settings (highest priority)
	if remoteSettings != nil {
		mergeSettings(merged, remoteSettings)
	}

	return merged, nil
}

// ManagedSettingsDir returns the path to the managed-settings.d directory.
// On Linux this is /etc/claude-code/managed-settings.d/.
func ManagedSettingsDir() string {
	return "/etc/claude-code/managed-settings.d"
}

// loadManagedSettingsDir scans the managed-settings.d directory for *.json files,
// sorts them by filename, and merges each into the target settings.
func loadManagedSettingsDir(target *Settings) error {
	dir := ManagedSettingsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Collect and sort JSON files by name
	var jsonFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			jsonFiles = append(jsonFiles, filepath.Join(dir, name))
		}
	}
	sort.Strings(jsonFiles)

	// Merge each file in sorted order
	for _, path := range jsonFiles {
		if err := mergeSettingsFromFile(target, path); err != nil {
			// Skip invalid files, continue with the rest
			continue
		}
	}
	return nil
}

// mergeSettingsFromFile reads a settings file and merges it into the target.
// If the file does not exist, it is silently skipped.
func mergeSettingsFromFile(target *Settings, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var src Settings
	if err := json.Unmarshal(data, &src); err != nil {
		return err
	}

	mergeSettings(target, &src)
	return nil
}

// mergeSettings merges src into target following Claude Code merge semantics:
// - Scalar fields (string, *bool, *int, *float64): higher priority overrides
// - Array fields: append from each source
// - Map fields: per-key merge (higher priority overrides per-key)
// - json.RawMessage fields: higher priority overrides entirely
func mergeSettings(target, src *Settings) {
	// --- Scalar string fields: override if non-empty ---
	mergeScalarString(&target.Schema, src.Schema)
	mergeScalarString(&target.APIKeyHelper, src.APIKeyHelper)
	mergeScalarString(&target.AWSCredentialExport, src.AWSCredentialExport)
	mergeScalarString(&target.AWSAuthRefresh, src.AWSAuthRefresh)
	mergeScalarString(&target.GCPAuthRefresh, src.GCPAuthRefresh)
	mergeScalarString(&target.PermissionMode, src.PermissionMode)
	mergeScalarString(&target.CustomInstructions, src.CustomInstructions)
	mergeScalarString(&target.Model, src.Model)
	mergeScalarString(&target.DefaultShell, src.DefaultShell)
	mergeScalarString(&target.ForceLoginMethod, src.ForceLoginMethod)
	mergeScalarString(&target.ForceLoginOrgUUID, src.ForceLoginOrgUUID)
	mergeScalarString(&target.OTelHeadersHelper, src.OTelHeadersHelper)
	mergeScalarString(&target.OutputStyle, src.OutputStyle)
	mergeScalarString(&target.Language, src.Language)
	mergeScalarString(&target.EffortLevel, src.EffortLevel)
	mergeScalarString(&target.AdvisorModel, src.AdvisorModel)
	mergeScalarString(&target.Agent, src.Agent)
	mergeScalarString(&target.AutoUpdatesChannel, src.AutoUpdatesChannel)
	mergeScalarString(&target.MinimumVersion, src.MinimumVersion)
	mergeScalarString(&target.PlansDirectory, src.PlansDirectory)
	mergeScalarString(&target.DisableAutoMode, src.DisableAutoMode)
	mergeScalarString(&target.AutoMemoryDirectory, src.AutoMemoryDirectory)
	mergeScalarString(&target.PluginTrustMessage, src.PluginTrustMessage)

	// --- Scalar pointer fields: override if non-nil ---
	mergeBoolPtr(&target.RespectGitignore, src.RespectGitignore)
	mergeBoolPtr(&target.IncludeCoAuthoredBy, src.IncludeCoAuthoredBy)
	mergeBoolPtr(&target.IncludeGitInstructions, src.IncludeGitInstructions)
	mergeBoolPtr(&target.EnableAllProjectMCPServers, src.EnableAllProjectMCPServers)
	mergeBoolPtr(&target.DisableAllHooks, src.DisableAllHooks)
	mergeBoolPtr(&target.AllowManagedHooksOnly, src.AllowManagedHooksOnly)
	mergeBoolPtr(&target.AllowManagedPermissionRulesOnly, src.AllowManagedPermissionRulesOnly)
	mergeBoolPtr(&target.AllowManagedMCPServersOnly, src.AllowManagedMCPServersOnly)
	mergeBoolPtr(&target.SkipWebFetchPreflight, src.SkipWebFetchPreflight)
	mergeBoolPtr(&target.SpinnerTipsEnabled, src.SpinnerTipsEnabled)
	mergeBoolPtr(&target.SyntaxHighlightingDisabled, src.SyntaxHighlightingDisabled)
	mergeBoolPtr(&target.TerminalTitleFromRename, src.TerminalTitleFromRename)
	mergeBoolPtr(&target.AlwaysThinkingEnabled, src.AlwaysThinkingEnabled)
	mergeBoolPtr(&target.FastMode, src.FastMode)
	mergeBoolPtr(&target.FastModePerSessionOptIn, src.FastModePerSessionOptIn)
	mergeBoolPtr(&target.PromptSuggestionEnabled, src.PromptSuggestionEnabled)
	mergeBoolPtr(&target.ShowClearContextOnPlanAccept, src.ShowClearContextOnPlanAccept)
	mergeBoolPtr(&target.VimMode, src.VimMode)
	mergeBoolPtr(&target.ChannelsEnabled, src.ChannelsEnabled)
	mergeBoolPtr(&target.PrefersReducedMotion, src.PrefersReducedMotion)
	mergeBoolPtr(&target.AutoMemoryEnabled, src.AutoMemoryEnabled)
	mergeBoolPtr(&target.AutoDreamEnabled, src.AutoDreamEnabled)
	mergeBoolPtr(&target.ShowThinkingSummaries, src.ShowThinkingSummaries)
	mergeBoolPtr(&target.SkipDangerousModePermissionPrompt, src.SkipDangerousModePermissionPrompt)

	// Int/float pointer fields
	mergeIntPtr(&target.CleanupPeriodDays, src.CleanupPeriodDays)
	mergeFloat64Ptr(&target.FeedbackSurveyRate, src.FeedbackSurveyRate)

	// --- Struct pointer fields: override if non-nil ---
	if src.FileSuggestion != nil {
		target.FileSuggestion = src.FileSuggestion
	}
	if src.Attribution != nil {
		target.Attribution = src.Attribution
	}
	if src.Permissions != nil {
		target.Permissions = src.Permissions
	}
	if src.WorktreeConfig != nil {
		target.WorktreeConfig = src.WorktreeConfig
	}
	if src.StatusLineConfig != nil {
		target.StatusLineConfig = src.StatusLineConfig
	}
	if src.SpinnerVerbsConfig != nil {
		target.SpinnerVerbsConfig = src.SpinnerVerbsConfig
	}
	if src.SpinnerTipsOverride != nil {
		target.SpinnerTipsOverride = src.SpinnerTipsOverride
	}
	if src.Remote != nil {
		target.Remote = src.Remote
	}

	// --- Slice fields: append ---
	target.AllowedTools = append(target.AllowedTools, src.AllowedTools...)
	target.DisallowedTools = append(target.DisallowedTools, src.DisallowedTools...)
	target.EnabledMCPJSONServers = append(target.EnabledMCPJSONServers, src.EnabledMCPJSONServers...)
	target.DisabledMCPJSONServers = append(target.DisabledMCPJSONServers, src.DisabledMCPJSONServers...)
	target.AllowedHTTPHookURLs = append(target.AllowedHTTPHookURLs, src.AllowedHTTPHookURLs...)
	target.HTTPHookAllowedEnvVars = append(target.HTTPHookAllowedEnvVars, src.HTTPHookAllowedEnvVars...)
	target.CompanyAnnouncements = append(target.CompanyAnnouncements, src.CompanyAnnouncements...)
	target.AllowedMCPServerNames = append(target.AllowedMCPServerNames, src.AllowedMCPServerNames...)
	target.DeniedMCPServerNames = append(target.DeniedMCPServerNames, src.DeniedMCPServerNames...)
	target.AvailableModels = append(target.AvailableModels, src.AvailableModels...)
	target.SSHConfigs = append(target.SSHConfigs, src.SSHConfigs...)
	target.ClaudeMDExcludes = append(target.ClaudeMDExcludes, src.ClaudeMDExcludes...)
	target.AllowedChannelPlugins = append(target.AllowedChannelPlugins, src.AllowedChannelPlugins...)

	// --- Map fields: per-key merge ---
	mergeStringMap(&target.Env, src.Env)
	mergeStringMap(&target.KeyBindings, src.KeyBindings)
	mergeStringMap(&target.ModelOverrides, src.ModelOverrides)

	// EnabledPlugins: map[string]json.RawMessage -- per-key merge
	if src.EnabledPlugins != nil {
		if target.EnabledPlugins == nil {
			target.EnabledPlugins = make(map[string]json.RawMessage)
		}
		for k, v := range src.EnabledPlugins {
			target.EnabledPlugins[k] = v
		}
	}

	// --- json.RawMessage fields: higher priority wins entirely ---
	mergeRawJSON(&target.AllowedMCPServersEntries, src.AllowedMCPServersEntries)
	mergeRawJSON(&target.DeniedMCPServersEntries, src.DeniedMCPServersEntries)
	mergeRawJSON(&target.Hooks, src.Hooks)
	mergeRawJSON(&target.MCPServers, src.MCPServers)
	mergeRawJSON(&target.ManagedMCP, src.ManagedMCP)
	mergeRawJSON(&target.Plugins, src.Plugins)
	mergeRawJSON(&target.StrictPluginOnlyCustomization, src.StrictPluginOnlyCustomization)
	mergeRawJSON(&target.ExtraKnownMarketplaces, src.ExtraKnownMarketplaces)
	mergeRawJSON(&target.StrictKnownMarketplaces, src.StrictKnownMarketplaces)
	mergeRawJSON(&target.BlockedMarketplaces, src.BlockedMarketplaces)
	mergeRawJSON(&target.Sandbox, src.Sandbox)
	mergeRawJSON(&target.PluginConfigs, src.PluginConfigs)
	mergeRawJSON(&target.AutoMode, src.AutoMode)
}

// --- Helper functions for merge ---

func mergeScalarString(target *string, src string) {
	if src != "" {
		*target = src
	}
}

func mergeBoolPtr(target **bool, src *bool) {
	if src != nil {
		*target = src
	}
}

func mergeIntPtr(target **int, src *int) {
	if src != nil {
		*target = src
	}
}

func mergeFloat64Ptr(target **float64, src *float64) {
	if src != nil {
		*target = src
	}
}

func mergeStringMap(target *map[string]string, src map[string]string) {
	if src != nil {
		if *target == nil {
			*target = make(map[string]string)
		}
		for k, v := range src {
			(*target)[k] = v
		}
	}
}

func mergeRawJSON(target *json.RawMessage, src json.RawMessage) {
	if len(src) > 0 {
		*target = src
	}
}

package config

import "encoding/json"

// GlobalConfig represents the global config stored at ~/.claude/.config.json.
// Matches the TypeScript GlobalConfig type (~70+ fields).
type GlobalConfig struct {
	// Core
	HasCompletedOnboarding bool   `json:"hasCompletedOnboarding,omitempty"`
	PrimaryAPIKey          string `json:"primaryApiKey,omitempty"`
	NumStartups            int    `json:"numStartups,omitempty"`
	InstallMethod          string `json:"installMethod,omitempty"`
	AutoUpdates            *bool  `json:"autoUpdates,omitempty"`
	AutoUpdatesProtected   *bool  `json:"autoUpdatesProtectedForNative,omitempty"`
	UserID                 string `json:"userID,omitempty"`
	Theme                  string `json:"theme,omitempty"`

	// Onboarding tracking
	LastOnboardingVersion string `json:"lastOnboardingVersion,omitempty"`
	LastReleaseNotesSeen  string `json:"lastReleaseNotesSeen,omitempty"`
	ChangelogLastFetched  *int64 `json:"changelogLastFetched,omitempty"`
	DoctorShownAtSession  *int   `json:"doctorShownAtSession,omitempty"`

	// Projects
	Projects map[string]*ProjectConfig `json:"projects,omitempty"`

	// MCP servers
	MCPServers             json.RawMessage `json:"mcpServers,omitempty"`
	ClaudeAIMCPConnected   []string        `json:"claudeAiMcpEverConnected,omitempty"`
	PreferredNotifChannel  string          `json:"preferredNotifChannel,omitempty"`
	CustomNotifyCommand    string          `json:"customNotifyCommand,omitempty"`
	Verbose                bool            `json:"verbose,omitempty"`

	// API key responses
	CustomAPIKeyResponses json.RawMessage `json:"customApiKeyResponses,omitempty"`

	// OAuth
	OAuthAccount json.RawMessage `json:"oauthAccount,omitempty"`

	// Editor mode
	EditorMode string `json:"editorMode,omitempty"`

	// Permission acceptance
	BypassPermissionsModeAccepted *bool `json:"bypassPermissionsModeAccepted,omitempty"`

	// Usage tracking booleans
	HasAcknowledgedCostThreshold *bool `json:"hasAcknowledgedCostThreshold,omitempty"`
	HasUsedBackslashReturn       *bool `json:"hasUsedBackslashReturn,omitempty"`
	HasSeenTasksHint             *bool `json:"hasSeenTasksHint,omitempty"`
	HasUsedStash                 *bool `json:"hasUsedStash,omitempty"`
	HasUsedBackgroundTask        *bool `json:"hasUsedBackgroundTask,omitempty"`
	HasSeenUndercoverAutoNotice  *bool `json:"hasSeenUndercoverAutoNotice,omitempty"`
	HasSeenUltraplanTerms        *bool `json:"hasSeenUltraplanTerms,omitempty"`

	// Auto-compact and display
	AutoCompactEnabled bool `json:"autoCompactEnabled,omitempty"`
	ShowTurnDuration   bool `json:"showTurnDuration,omitempty"`

	// Environment variables (deprecated, use settings.env)
	Env map[string]string `json:"env,omitempty"`

	// Diff tool
	DiffTool string `json:"diffTool,omitempty"` // "terminal" or "vscode"

	// Terminal setup tracking
	Iterm2SetupInProgress       *bool  `json:"iterm2SetupInProgress,omitempty"`
	Iterm2BackupPath            string `json:"iterm2BackupPath,omitempty"`
	AppleTerminalBackupPath     string `json:"appleTerminalBackupPath,omitempty"`
	AppleTerminalSetupInProgress *bool  `json:"appleTerminalSetupInProgress,omitempty"`

	// Key binding setup
	ShiftEnterKeyBindingInstalled *bool `json:"shiftEnterKeyBindingInstalled,omitempty"`
	OptionAsMetaKeyInstalled      *bool `json:"optionAsMetaKeyInstalled,omitempty"`
	Iterm2KeyBindingInstalled     *bool `json:"iterm2KeyBindingInstalled,omitempty"` // Legacy

	// IDE configurations
	AutoConnectIDE            *bool `json:"autoConnectIde,omitempty"`
	AutoInstallIDEExtension   *bool `json:"autoInstallIdeExtension,omitempty"`

	// IDE dialogs
	HasIDEOnboardingBeenShown     map[string]bool `json:"hasIdeOnboardingBeenShown,omitempty"`
	IDEHintShownCount             *int            `json:"ideHintShownCount,omitempty"`
	HasIDEAutoConnectDialogShown  *bool           `json:"hasIdeAutoConnectDialogBeenShown,omitempty"`

	// Tips
	TipsHistory map[string]int `json:"tipsHistory,omitempty"`

	// Memory usage
	MemoryUsageCount int `json:"memoryUsageCount,omitempty"`

	// Queued command hint count
	QueuedCommandUpHintCount *int `json:"queuedCommandUpHintCount,omitempty"`

	// Feedback survey state
	FeedbackSurveyState json.RawMessage `json:"feedbackSurveyState,omitempty"`

	// Transcript share prompt tracking
	TranscriptShareDismissed *bool `json:"transcriptShareDismissed,omitempty"`

	// File checkpointing
	FileCheckpointingEnabled bool `json:"fileCheckpointingEnabled,omitempty"`

	// Terminal progress bar
	TerminalProgressBarEnabled bool `json:"terminalProgressBarEnabled,omitempty"`

	// Terminal tab status
	ShowStatusInTerminalTab *bool `json:"showStatusInTerminalTab,omitempty"`

	// Push notification toggles
	TaskCompleteNotifEnabled *bool `json:"taskCompleteNotifEnabled,omitempty"`
	InputNeededNotifEnabled  *bool `json:"inputNeededNotifEnabled,omitempty"`
	AgentPushNotifEnabled    *bool `json:"agentPushNotifEnabled,omitempty"`

	// First start time
	FirstStartTime string `json:"firstStartTime,omitempty"`

	// Idle notification threshold
	MessageIdleNotifThresholdMs int `json:"messageIdleNotifThresholdMs,omitempty"`

	// GitHub/Slack setup counts
	GithubActionSetupCount *int `json:"githubActionSetupCount,omitempty"`
	SlackAppInstallCount   *int `json:"slackAppInstallCount,omitempty"`

	// Gitignore behavior
	RespectGitignore bool `json:"respectGitignore,omitempty"`

	// Copy command behavior
	CopyFullResponse bool  `json:"copyFullResponse,omitempty"`
	CopyOnSelect     *bool `json:"copyOnSelect,omitempty"`

	// Todo feature
	TodoFeatureEnabled bool  `json:"todoFeatureEnabled,omitempty"`
	ShowExpandedTodos  *bool `json:"showExpandedTodos,omitempty"`
	ShowSpinnerTree    *bool `json:"showSpinnerTree,omitempty"`

	// Queue/btw usage
	PromptQueueUseCount int `json:"promptQueueUseCount,omitempty"`
	BtwUseCount         int `json:"btwUseCount,omitempty"`

	// Plan mode tracking
	LastPlanModeUse *int64 `json:"lastPlanModeUse,omitempty"`

	// Subscription tracking
	SubscriptionNoticeCount    *int   `json:"subscriptionNoticeCount,omitempty"`
	HasAvailableSubscription   *bool  `json:"hasAvailableSubscription,omitempty"`
	SubscriptionUpsellShownCount *int `json:"subscriptionUpsellShownCount,omitempty"`

	// Voice mode tracking
	VoiceNoticeSeenCount      *int    `json:"voiceNoticeSeenCount,omitempty"`
	VoiceLangHintShownCount   *int    `json:"voiceLangHintShownCount,omitempty"`
	VoiceLangHintLastLanguage string  `json:"voiceLangHintLastLanguage,omitempty"`
	VoiceFooterHintSeenCount  *int    `json:"voiceFooterHintSeenCount,omitempty"`

	// Experiment tracking
	ExperimentNoticesSeenCount map[string]int `json:"experimentNoticesSeenCount,omitempty"`

	// Cached feature gates
	CachedStatsigGates     map[string]bool            `json:"cachedStatsigGates,omitempty"`
	CachedDynamicConfigs   map[string]json.RawMessage `json:"cachedDynamicConfigs,omitempty"`
	CachedGrowthBookFeatures map[string]json.RawMessage `json:"cachedGrowthBookFeatures,omitempty"`
	GrowthBookOverrides    map[string]json.RawMessage `json:"growthBookOverrides,omitempty"`

	// Migration version
	MigrationVersion *int `json:"migrationVersion,omitempty"`

	// GitHub repo paths
	GithubRepoPaths map[string][]string `json:"githubRepoPaths,omitempty"`

	// Deep link terminal
	DeepLinkTerminal string `json:"deepLinkTerminal,omitempty"`

	// iTerm2 CLI setup
	Iterm2It2SetupComplete *bool `json:"iterm2It2SetupComplete,omitempty"`
	PreferTmuxOverIterm2   *bool `json:"preferTmuxOverIterm2,omitempty"`

	// Skill usage tracking
	SkillUsage map[string]json.RawMessage `json:"skillUsage,omitempty"`

	// Marketplace auto-install tracking
	OfficialMarketplaceAutoInstallAttempted *bool   `json:"officialMarketplaceAutoInstallAttempted,omitempty"`
	OfficialMarketplaceAutoInstalled        *bool   `json:"officialMarketplaceAutoInstalled,omitempty"`
	OfficialMarketplaceAutoInstallFailReason string `json:"officialMarketplaceAutoInstallFailReason,omitempty"`

	// LSP plugin recommendation
	LSPRecommendationDisabled    *bool    `json:"lspRecommendationDisabled,omitempty"`
	LSPRecommendationNeverPlugins []string `json:"lspRecommendationNeverPlugins,omitempty"`

	// Permission explainer
	PermissionExplainerEnabled *bool `json:"permissionExplainerEnabled,omitempty"`

	// Teammate configuration
	TeammateMode         string `json:"teammateMode,omitempty"`         // "auto", "tmux", "in-process"
	TeammateDefaultModel string `json:"teammateDefaultModel,omitempty"`

	// Bridge OAuth backoff
	BridgeOAuthDeadExpiresAt *int64 `json:"bridgeOauthDeadExpiresAt,omitempty"`
	BridgeOAuthDeadFailCount *int   `json:"bridgeOauthDeadFailCount,omitempty"`

	// Desktop upsell tracking
	DesktopUpsellSeenCount *int  `json:"desktopUpsellSeenCount,omitempty"`
	DesktopUpsellDismissed *bool `json:"desktopUpsellDismissed,omitempty"`

	// Remote control at startup
	RemoteControlAtStartup *bool `json:"remoteControlAtStartup,omitempty"`

	// Speculation
	SpeculationEnabled *bool `json:"speculationEnabled,omitempty"`

	// Client data cache
	ClientDataCache json.RawMessage `json:"clientDataCache,omitempty"`

	// Sonnet-1M caches
	S1MAccessCache              json.RawMessage `json:"s1mAccessCache,omitempty"`
	S1MNonSubscriberAccessCache json.RawMessage `json:"s1mNonSubscriberAccessCache,omitempty"`

	// Passes eligibility cache
	PassesEligibilityCache json.RawMessage `json:"passesEligibilityCache,omitempty"`

	// Guest passes upsell tracking
	PassesUpsellSeenCount   *int  `json:"passesUpsellSeenCount,omitempty"`
	HasVisitedPasses        *bool `json:"hasVisitedPasses,omitempty"`
	PassesLastSeenRemaining *int  `json:"passesLastSeenRemaining,omitempty"`

	// Overage credit grant
	OverageCreditGrantCache     json.RawMessage `json:"overageCreditGrantCache,omitempty"`
	OverageCreditUpsellSeenCount *int           `json:"overageCreditUpsellSeenCount,omitempty"`
	HasVisitedExtraUsage        *bool           `json:"hasVisitedExtraUsage,omitempty"`

	// Metrics cache
	MetricsStatusCache json.RawMessage `json:"metricsStatusCache,omitempty"`
}

// ProjectConfig represents per-project configuration.
type ProjectConfig struct {
	AllowedTools    []string        `json:"allowedTools,omitempty"`
	MCPContextURIs  []string        `json:"mcpContextUris,omitempty"`
	MCPServers      json.RawMessage `json:"mcpServers,omitempty"`

	// Session metrics
	LastAPIDuration                      *float64                       `json:"lastAPIDuration,omitempty"`
	LastAPIDurationWithoutRetries        *float64                       `json:"lastAPIDurationWithoutRetries,omitempty"`
	LastToolDuration                     *float64                       `json:"lastToolDuration,omitempty"`
	LastCost                             *float64                       `json:"lastCost,omitempty"`
	LastDuration                         *float64                       `json:"lastDuration,omitempty"`
	LastLinesAdded                       *int                           `json:"lastLinesAdded,omitempty"`
	LastLinesRemoved                     *int                           `json:"lastLinesRemoved,omitempty"`
	LastTotalInputTokens                 *int                           `json:"lastTotalInputTokens,omitempty"`
	LastTotalOutputTokens                *int                           `json:"lastTotalOutputTokens,omitempty"`
	LastTotalCacheCreationInputTokens    *int                           `json:"lastTotalCacheCreationInputTokens,omitempty"`
	LastTotalCacheReadInputTokens        *int                           `json:"lastTotalCacheReadInputTokens,omitempty"`
	LastTotalWebSearchRequests           *int                           `json:"lastTotalWebSearchRequests,omitempty"`
	LastFPSAverage                       *float64                       `json:"lastFpsAverage,omitempty"`
	LastFPSLow1Pct                       *float64                       `json:"lastFpsLow1Pct,omitempty"`
	LastSessionID                        string                         `json:"lastSessionId,omitempty"`
	LastModelUsage                       map[string]json.RawMessage     `json:"lastModelUsage,omitempty"`
	LastSessionMetrics                   map[string]float64             `json:"lastSessionMetrics,omitempty"`

	// Example files
	ExampleFiles          []string `json:"exampleFiles,omitempty"`
	ExampleFilesGeneratedAt *int64 `json:"exampleFilesGeneratedAt,omitempty"`

	// Trust dialog
	HasTrustDialogAccepted *bool `json:"hasTrustDialogAccepted,omitempty"`

	// Project onboarding
	HasCompletedProjectOnboarding              *bool `json:"hasCompletedProjectOnboarding,omitempty"`
	ProjectOnboardingSeenCount                 int   `json:"projectOnboardingSeenCount,omitempty"`
	HasClaudeMDExternalIncludesApproved        *bool `json:"hasClaudeMdExternalIncludesApproved,omitempty"`
	HasClaudeMDExternalIncludesWarningShown    *bool `json:"hasClaudeMdExternalIncludesWarningShown,omitempty"`

	// MCP server approval (migrated to settings, kept for backward compat)
	EnabledMCPJSONServers      []string `json:"enabledMcpjsonServers,omitempty"`
	DisabledMCPJSONServers     []string `json:"disabledMcpjsonServers,omitempty"`
	EnableAllProjectMCPServers *bool    `json:"enableAllProjectMcpServers,omitempty"`
	DisabledMCPServers         []string `json:"disabledMcpServers,omitempty"`
	EnabledMCPServers          []string `json:"enabledMcpServers,omitempty"`

	// Worktree session
	ActiveWorktreeSession json.RawMessage `json:"activeWorktreeSession,omitempty"`

	// Remote control spawn mode
	RemoteControlSpawnMode string `json:"remoteControlSpawnMode,omitempty"` // "same-dir" or "worktree"
}

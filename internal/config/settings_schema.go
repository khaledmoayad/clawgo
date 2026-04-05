package config

import "encoding/json"

// PermissionRule represents a single permission rule with tool and optional matcher.
type PermissionRule struct {
	Tool    string `json:"tool"`
	Matcher string `json:"matcher,omitempty"`
}

// Permissions represents the permissions section of settings.
type Permissions struct {
	Allow                    []PermissionRule `json:"allow,omitempty"`
	Deny                     []PermissionRule `json:"deny,omitempty"`
	Ask                      []PermissionRule `json:"ask,omitempty"`
	DefaultMode              string           `json:"defaultMode,omitempty"`
	DisableBypassPermissions string           `json:"disableBypassPermissionsMode,omitempty"`
	AdditionalDirectories    []string         `json:"additionalDirectories,omitempty"`
}

// FileSuggestion configures custom file suggestion for @ mentions.
type FileSuggestion struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// Attribution configures attribution text for commits and PRs.
type Attribution struct {
	Commit string `json:"commit,omitempty"`
	PR     string `json:"pr,omitempty"`
}

// Worktree configures git worktree behavior.
type Worktree struct {
	SymlinkDirectories []string `json:"symlinkDirectories,omitempty"`
	SparsePaths        []string `json:"sparsePaths,omitempty"`
}

// StatusLine configures the custom status line display.
type StatusLine struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Padding *int   `json:"padding,omitempty"`
}

// SpinnerVerbs configures custom spinner verbs.
type SpinnerVerbs struct {
	Mode  string   `json:"mode"`  // "append" or "replace"
	Verbs []string `json:"verbs"`
}

// SpinnerTipsOverride configures custom spinner tips.
type SpinnerTipsOverride struct {
	ExcludeDefault *bool    `json:"excludeDefault,omitempty"`
	Tips           []string `json:"tips"`
}

// RemoteConfig configures remote session settings.
type RemoteConfig struct {
	DefaultEnvironmentID string `json:"defaultEnvironmentId,omitempty"`
}

// SSHConfig represents a single SSH connection configuration.
type SSHConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SSHHost         string `json:"sshHost"`
	SSHPort         *int   `json:"sshPort,omitempty"`
	SSHIdentityFile string `json:"sshIdentityFile,omitempty"`
	StartDirectory  string `json:"startDirectory,omitempty"`
}

// AllowedChannelPlugin represents an allowed channel plugin entry.
type AllowedChannelPlugin struct {
	Marketplace string `json:"marketplace"`
	Plugin      string `json:"plugin"`
}

// Settings represents the settings from settings.json files.
// Matches the TypeScript SettingsSchema (~100+ fields).
type Settings struct {
	// JSON Schema reference
	Schema string `json:"$schema,omitempty"`

	// Authentication helpers
	APIKeyHelper      string `json:"apiKeyHelper,omitempty"`
	AWSCredentialExport string `json:"awsCredentialExport,omitempty"`
	AWSAuthRefresh    string `json:"awsAuthRefresh,omitempty"`
	GCPAuthRefresh    string `json:"gcpAuthRefresh,omitempty"`

	// File suggestion
	FileSuggestion *FileSuggestion `json:"fileSuggestion,omitempty"`

	// Gitignore behavior
	RespectGitignore *bool `json:"respectGitignore,omitempty"`

	// Cleanup period
	CleanupPeriodDays *int `json:"cleanupPeriodDays,omitempty"`

	// Environment variables
	Env map[string]string `json:"env,omitempty"`

	// Attribution
	Attribution       *Attribution `json:"attribution,omitempty"`
	IncludeCoAuthoredBy *bool     `json:"includeCoAuthoredBy,omitempty"` // Deprecated: use Attribution
	IncludeGitInstructions *bool  `json:"includeGitInstructions,omitempty"`

	// Permissions
	Permissions    *Permissions `json:"permissions,omitempty"`
	PermissionMode string       `json:"permissionMode,omitempty"`

	// Custom instructions
	CustomInstructions string `json:"customInstructions,omitempty"`

	// Model configuration
	Model           string            `json:"model,omitempty"`
	AvailableModels []string          `json:"availableModels,omitempty"`
	ModelOverrides  map[string]string `json:"modelOverrides,omitempty"`

	// MCP configuration
	EnableAllProjectMCPServers *bool    `json:"enableAllProjectMcpServers,omitempty"`
	EnabledMCPJSONServers      []string `json:"enabledMcpjsonServers,omitempty"`
	DisabledMCPJSONServers     []string `json:"disabledMcpjsonServers,omitempty"`

	// Enterprise MCP allow/deny lists (structured entries)
	AllowedMCPServersEntries json.RawMessage `json:"allowedMcpServers,omitempty"`
	DeniedMCPServersEntries  json.RawMessage `json:"deniedMcpServers,omitempty"`

	// Hooks
	Hooks json.RawMessage `json:"hooks,omitempty"`

	// Worktree
	WorktreeConfig *Worktree `json:"worktree,omitempty"`

	// Hook controls
	DisableAllHooks       *bool  `json:"disableAllHooks,omitempty"`
	DefaultShell          string `json:"defaultShell,omitempty"` // "bash" or "powershell"
	AllowManagedHooksOnly *bool  `json:"allowManagedHooksOnly,omitempty"`

	// HTTP hook controls
	AllowedHTTPHookURLs     []string `json:"allowedHttpHookUrls,omitempty"`
	HTTPHookAllowedEnvVars  []string `json:"httpHookAllowedEnvVars,omitempty"`

	// Enterprise policy controls
	AllowManagedPermissionRulesOnly *bool `json:"allowManagedPermissionRulesOnly,omitempty"`
	AllowManagedMCPServersOnly      *bool `json:"allowManagedMcpServersOnly,omitempty"`

	// Plugin customization lock
	StrictPluginOnlyCustomization json.RawMessage `json:"strictPluginOnlyCustomization,omitempty"` // bool or []string

	// Status line
	StatusLineConfig *StatusLine `json:"statusLine,omitempty"`

	// Plugins
	EnabledPlugins map[string]json.RawMessage `json:"enabledPlugins,omitempty"` // Can be bool or []string per key
	Plugins        json.RawMessage            `json:"plugins,omitempty"`

	// Marketplace controls
	ExtraKnownMarketplaces json.RawMessage `json:"extraKnownMarketplaces,omitempty"`
	StrictKnownMarketplaces json.RawMessage `json:"strictKnownMarketplaces,omitempty"`
	BlockedMarketplaces    json.RawMessage `json:"blockedMarketplaces,omitempty"`

	// Login controls
	ForceLoginMethod  string `json:"forceLoginMethod,omitempty"` // "claudeai" or "console"
	ForceLoginOrgUUID string `json:"forceLoginOrgUUID,omitempty"`

	// Telemetry
	OTelHeadersHelper string `json:"otelHeadersHelper,omitempty"`

	// Output
	OutputStyle string `json:"outputStyle,omitempty"`
	Language    string `json:"language,omitempty"`

	// Web fetch
	SkipWebFetchPreflight *bool `json:"skipWebFetchPreflight,omitempty"`

	// Sandbox
	Sandbox json.RawMessage `json:"sandbox,omitempty"`

	// Feedback
	FeedbackSurveyRate *float64 `json:"feedbackSurveyRate,omitempty"`

	// Spinner configuration
	SpinnerTipsEnabled  *bool                `json:"spinnerTipsEnabled,omitempty"`
	SpinnerVerbsConfig  *SpinnerVerbs        `json:"spinnerVerbs,omitempty"`
	SpinnerTipsOverride *SpinnerTipsOverride `json:"spinnerTipsOverride,omitempty"`

	// Display controls
	SyntaxHighlightingDisabled *bool `json:"syntaxHighlightingDisabled,omitempty"`
	TerminalTitleFromRename    *bool `json:"terminalTitleFromRename,omitempty"`

	// Thinking/effort
	AlwaysThinkingEnabled *bool  `json:"alwaysThinkingEnabled,omitempty"`
	EffortLevel           string `json:"effortLevel,omitempty"` // "low", "medium", "high", "max"
	AdvisorModel          string `json:"advisorModel,omitempty"`

	// Fast mode
	FastMode                *bool `json:"fastMode,omitempty"`
	FastModePerSessionOptIn *bool `json:"fastModePerSessionOptIn,omitempty"`

	// Prompt suggestions
	PromptSuggestionEnabled    *bool `json:"promptSuggestionEnabled,omitempty"`
	ShowClearContextOnPlanAccept *bool `json:"showClearContextOnPlanAccept,omitempty"`

	// Agent
	Agent string `json:"agent,omitempty"`

	// Company announcements
	CompanyAnnouncements []string `json:"companyAnnouncements,omitempty"`

	// Plugin configs (per-plugin MCP server user configs)
	PluginConfigs json.RawMessage `json:"pluginConfigs,omitempty"`

	// Tool allow/deny (legacy simple string lists)
	AllowedTools    []string `json:"allowedTools,omitempty"`
	DisallowedTools []string `json:"disallowedTools,omitempty"`

	// Key bindings
	KeyBindings map[string]string `json:"keyBindings,omitempty"`

	// Vim mode
	VimMode *bool `json:"vimMode,omitempty"`

	// MCP servers raw config
	MCPServers json.RawMessage `json:"mcpServers,omitempty"`

	// Enterprise MCP (legacy simple string allow/deny lists)
	AllowedMCPServerNames []string `json:"allowedMcpServerNames,omitempty"`
	DeniedMCPServerNames  []string `json:"deniedMcpServerNames,omitempty"`

	// Managed MCP
	ManagedMCP json.RawMessage `json:"managedMcp,omitempty"`

	// Remote session configuration
	Remote *RemoteConfig `json:"remote,omitempty"`

	// Auto-updates channel
	AutoUpdatesChannel string `json:"autoUpdatesChannel,omitempty"` // "latest" or "stable"

	// Minimum version
	MinimumVersion string `json:"minimumVersion,omitempty"`

	// Plans directory
	PlansDirectory string `json:"plansDirectory,omitempty"`

	// Channels
	ChannelsEnabled       *bool                  `json:"channelsEnabled,omitempty"`
	AllowedChannelPlugins []AllowedChannelPlugin `json:"allowedChannelPlugins,omitempty"`

	// Accessibility
	PrefersReducedMotion *bool `json:"prefersReducedMotion,omitempty"`

	// Auto-memory
	AutoMemoryEnabled   *bool  `json:"autoMemoryEnabled,omitempty"`
	AutoMemoryDirectory string `json:"autoMemoryDirectory,omitempty"`
	AutoDreamEnabled    *bool  `json:"autoDreamEnabled,omitempty"`

	// Thinking summaries
	ShowThinkingSummaries *bool `json:"showThinkingSummaries,omitempty"`

	// Bypass permissions acceptance
	SkipDangerousModePermissionPrompt *bool `json:"skipDangerousModePermissionPrompt,omitempty"`

	// Auto mode
	DisableAutoMode string          `json:"disableAutoMode,omitempty"` // "disable"
	AutoMode        json.RawMessage `json:"autoMode,omitempty"`

	// SSH configs
	SSHConfigs []SSHConfig `json:"sshConfigs,omitempty"`

	// CLAUDE.md excludes
	ClaudeMDExcludes []string `json:"claudeMdExcludes,omitempty"`

	// Plugin trust message
	PluginTrustMessage string `json:"pluginTrustMessage,omitempty"`
}

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/app"
	"github.com/khaledmoayad/clawgo/internal/bridge"
	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/khaledmoayad/clawgo/internal/daemon"
	mcppkg "github.com/khaledmoayad/clawgo/internal/mcp"
	"github.com/spf13/cobra"
)

// AppContext holds initialized application state available to all commands.
type AppContext struct {
	Config   *config.Config
	Settings *config.Settings
	APIKey   string
	Flags    *CLIFlags
	Cancel   context.CancelFunc
}

// NewRootCmd creates the root Cobra command for the ClawGo CLI.
// It registers all flags matching the TypeScript version's main.tsx.
func NewRootCmd(version string) *cobra.Command {
	flags := &CLIFlags{}
	appCtx := &AppContext{Flags: flags}

	cmd := &cobra.Command{
		Use:     "clawgo [prompt]",
		Short:   "ClawGo - Claude Code in Go",
		Long:    "ClawGo is a drop-in replacement for Claude Code, built in Go.",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		// SilenceUsage prevents printing usage on errors from RunE
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				flags.Prompt = args[0]
			}

			// Create cancellable context
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Set up graceful shutdown with the new cancel
			app.SetupGracefulShutdown(cancel)

			// Convert CLIFlags to RunParams (breaks import cycle)
			runParams := &app.RunParams{
				// Core flags
				Model:           flags.Model,
				PermissionMode:  flags.PermissionMode,
				Resume:          flags.Resume,
				SessionID:       flags.SessionID,
				MaxTurns:        flags.MaxTurns,
				SystemPrompt:    flags.SystemPrompt,
				OutputFormat:    flags.OutputFormat,
				AllowedTools:    flags.AllowedTools,
				DisallowedTools: flags.DisallowedTools,
				Prompt:          flags.Prompt,
				Version:         version,

				// CLI-01: Print mode
				Print: flags.Print,

				// CLI-02: Session flags
				Continue:             flags.Continue,
				ForkSession:          flags.ForkSession,
				ResumeSessionAt:      flags.ResumeSessionAt,
				Name:                 flags.Name,
				Prefill:              flags.Prefill,
				FromPR:               flags.FromPR,
				NoSessionPersistence: flags.NoSessionPersistence,

				// CLI-03: Model/performance
				Effort:        flags.Effort,
				Thinking:      flags.Thinking,
				MaxBudgetUSD:  flags.MaxBudgetUSD,
				FallbackModel: flags.FallbackModel,
				Betas:         flags.Betas,
				TaskBudget:    flags.TaskBudget,

				// CLI-04: Debug
				Debug:     flags.Debug,
				DebugFile: flags.DebugFile,
				Bare:      flags.Bare,

				// CLI-05: Output
				JSONSchema:             flags.JSONSchema,
				InputFormat:            flags.InputFormat,
				IncludeHookEvents:      flags.IncludeHookEvents,
				IncludePartialMessages: flags.IncludePartialMessages,
				ReplayUserMessages:     flags.ReplayUserMessages,

				// CLI-06: System prompt
				AppendSystemPrompt:     flags.AppendSystemPrompt,
				SystemPromptFile:       flags.SystemPromptFile,
				AppendSystemPromptFile: flags.AppendSystemPromptFile,

				// CLI-07: Agent
				Agent:            flags.Agent,
				Agents:           flags.Agents,
				AgentID:          flags.AgentID,
				AgentName:        flags.AgentName,
				PlanModeRequired: flags.PlanModeRequired,
				Proactive:        flags.Proactive,
				Brief:            flags.Brief,

				// CLI-08: Permissions
				DangerouslySkipPermissions:      flags.DangerouslySkipPermissions,
				AllowDangerouslySkipPermissions: flags.AllowDangerouslySkipPermissions,
				PermissionPromptTool:            flags.PermissionPromptTool,

				// CLI-09: Tool/plugin/settings
				Tools:                flags.Tools,
				StrictMCPConfig:      flags.StrictMCPConfig,
				PluginDir:            flags.PluginDir,
				DisableSlashCommands: flags.DisableSlashCommands,
				Settings:             flags.Settings,
				AddDir:               flags.AddDir,
				IDE:                  flags.IDE,
			}

			return app.Run(ctx, runParams, appCtx.Config, appCtx.Settings)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate --effort if provided
			if flags.Effort != "" {
				allowed := map[string]bool{"low": true, "medium": true, "high": true, "max": true}
				v := strings.ToLower(flags.Effort)
				if !allowed[v] {
					return fmt.Errorf("--effort must be one of: low, medium, high, max (got %q)", flags.Effort)
				}
				flags.Effort = v
			}

			// Validate --max-budget-usd if provided
			if flags.MaxBudgetUSD < 0 {
				return fmt.Errorf("--max-budget-usd must be a positive number (got %f)", flags.MaxBudgetUSD)
			}

			// Validate --task-budget if provided
			if flags.TaskBudget < 0 {
				return fmt.Errorf("--task-budget must be a positive integer (got %d)", flags.TaskBudget)
			}

			// Load global config from ~/.claude/.config.json
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			appCtx.Config = cfg

			// Load and merge settings (user < project < enterprise)
			// Project root detection is a placeholder -- will be enhanced in later plans
			settings, err := config.LoadSettings(config.ConfigDir(), "")
			if err != nil {
				return fmt.Errorf("failed to load settings: %w", err)
			}
			appCtx.Settings = settings

			// Resolve API key from env vars, config, or credentials file
			appCtx.APIKey = config.ResolveAPIKey(cfg)

			// Setup graceful shutdown
			_, cancel := context.WithCancel(cmd.Context())
			appCtx.Cancel = cancel
			app.SetupGracefulShutdown(cancel)

			return nil
		},
	}

	// Register MCP subcommand group
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP protocol commands",
		Long:  "Commands for the Model Context Protocol (MCP) server and client.",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start MCP server on stdio",
		Long:  "Start an MCP server that exposes all ClawGo tools over stdio transport.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			app.SetupGracefulShutdown(cancel)

			// Build the tool registry using the shared app function
			registry := app.BuildRegistry(nil)
			return mcppkg.StartServer(ctx, registry, version)
		},
	}

	mcpCmd.AddCommand(serveCmd)

	// MCP management subcommands (add/remove/list/get/add-json/add-from-claude-desktop/reset-project-choices)
	mcpCmd.AddCommand(newMCPAddCmd())
	mcpCmd.AddCommand(newMCPRemoveCmd())
	mcpCmd.AddCommand(newMCPListCmd())
	mcpCmd.AddCommand(newMCPGetCmd())
	mcpCmd.AddCommand(newMCPAddJSONCmd())
	mcpCmd.AddCommand(newMCPAddFromDesktopCmd())
	mcpCmd.AddCommand(newMCPResetProjectChoicesCmd())

	cmd.AddCommand(mcpCmd)
	cmd.AddCommand(newCompletionCmd())

	// Auth, update, and server subcommands
	cmd.AddCommand(newAuthCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newServerCmd())

	// Plugin and task subcommands
	cmd.AddCommand(newPluginCmd())
	cmd.AddCommand(newTaskCmd())

	// Bridge / remote control subcommand
	var bridgeEnvName string
	bridgeCmd := &cobra.Command{
		Use:   "remote-control",
		Short: "Start bridge/remote control mode",
		Long:  "Register this machine as a bridge environment, poll for work from claude.ai, and spawn child sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			app.SetupGracefulShutdown(cancel)

			cfg := bridge.Config{
				APIBaseURL:      config.ResolveAPIBaseURL(appCtx.Config),
				GetToken:        func() string { return appCtx.APIKey },
				EnvironmentName: bridgeEnvName,
			}
			return bridge.NewBridge(cfg).Start(ctx)
		},
	}
	bridgeCmd.Flags().StringVar(&bridgeEnvName, "name", "", "Environment name for this bridge")
	cmd.AddCommand(bridgeCmd)

	// Daemon worker subcommand
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start daemon worker",
		Long:  "Start the background daemon scheduler that checks and fires cron tasks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			app.SetupGracefulShutdown(cancel)

			configDir := config.ConfigDir()
			return daemon.NewScheduler(configDir).Start(ctx)
		},
	}
	cmd.AddCommand(daemonCmd)

	// -------------------------------------------------------------------
	// Register all flags matching TypeScript main.tsx (~55 flags)
	// -------------------------------------------------------------------
	f := cmd.Flags()

	// CLI-11: Register --verbose/-v BEFORE Cobra's InitDefaultVersionFlag() runs.
	// Cobra checks ShorthandLookup("v") when creating the version flag; if -v
	// is already taken, --version gets registered WITHOUT the -v shorthand.
	// This matches Claude Code behavior where -v means --verbose, not --version.
	f.BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose output")

	// Core flags
	f.StringVarP(&flags.Model, "model", "m", "", "Model to use (overrides ANTHROPIC_MODEL)")
	f.StringVar(&flags.PermissionMode, "permission-mode", "default", "Permission mode: default, plan, auto, bypass")
	f.BoolVarP(&flags.Resume, "resume", "r", false, "Resume a conversation by session ID")
	f.StringVar(&flags.SessionID, "session-id", "", "Specific session ID to resume")
	f.IntVar(&flags.MaxTurns, "max-turns", 0, "Maximum conversation turns (0 = unlimited)")
	f.StringVar(&flags.SystemPrompt, "system-prompt", "", "System prompt override")
	f.StringVar(&flags.OutputFormat, "output-format", "text", "Output format: text, json, stream-json")
	f.StringSliceVar(&flags.AllowedTools, "allowed-tools", nil, "Tool allowlist (comma-separated)")
	f.StringSliceVar(&flags.DisallowedTools, "disallowed-tools", nil, "Tool denylist (comma-separated)")
	f.StringVar(&flags.MCPConfig, "mcp-config", "", "MCP configuration file path")

	// CLI-12: camelCase aliases for --allowed-tools and --disallowed-tools
	f.StringSliceVar(&flags.AllowedTools, "allowedTools", nil, "Alias for --allowed-tools")
	f.StringSliceVar(&flags.DisallowedTools, "disallowedTools", nil, "Alias for --disallowed-tools")

	// CLI-01: Print mode
	f.BoolVarP(&flags.Print, "print", "p", false, "Print response and exit (useful for pipes)")

	// CLI-02: Session flags
	f.BoolVarP(&flags.Continue, "continue", "c", false, "Continue the most recent conversation")
	f.BoolVar(&flags.ForkSession, "fork-session", false, "When resuming, create a new session ID instead of reusing the original")
	f.StringVar(&flags.ResumeSessionAt, "resume-session-at", "", "When resuming, only messages up to specified assistant message ID")
	f.StringVarP(&flags.Name, "name", "n", "", "Set a display name for this session")
	f.StringVar(&flags.Prefill, "prefill", "", "Pre-fill the prompt input with text without submitting it")
	f.StringVar(&flags.FromPR, "from-pr", "", "Resume a session linked to a PR by PR number/URL")
	f.BoolVar(&flags.NoSessionPersistence, "no-session-persistence", false, "Disable session persistence")

	// CLI-03: Model/performance flags
	f.StringVar(&flags.Effort, "effort", "", "Effort level: low, medium, high, max")
	f.StringVar(&flags.Thinking, "thinking", "", "Thinking mode: enabled, adaptive, disabled")
	f.Float64Var(&flags.MaxBudgetUSD, "max-budget-usd", 0, "Maximum dollar amount to spend on API calls")
	f.StringVar(&flags.FallbackModel, "fallback-model", "", "Fallback model when default is overloaded")
	f.StringSliceVar(&flags.Betas, "betas", nil, "Beta headers to include in API requests")
	f.IntVar(&flags.TaskBudget, "task-budget", 0, "API-side task budget in tokens")

	// CLI-04: Debug flags
	f.BoolVarP(&flags.Debug, "debug", "d", false, "Enable debug mode")
	f.StringVar(&flags.DebugFile, "debug-file", "", "Write debug logs to a specific file path")
	f.BoolVar(&flags.Bare, "bare", false, "Minimal mode: skip hooks, LSP, plugin sync, CLAUDE.md auto-discovery")

	// CLI-05: Output flags
	f.StringVar(&flags.JSONSchema, "json-schema", "", "JSON Schema for structured output validation")
	f.StringVar(&flags.InputFormat, "input-format", "text", "Input format: text, stream-json")
	f.BoolVar(&flags.IncludeHookEvents, "include-hook-events", false, "Include hook lifecycle events in output stream")
	f.BoolVar(&flags.IncludePartialMessages, "include-partial-messages", false, "Include partial message chunks as they arrive")
	f.BoolVar(&flags.ReplayUserMessages, "replay-user-messages", false, "Re-emit user messages from stdin back on stdout")

	// CLI-06: System prompt flags
	f.StringVar(&flags.SystemPromptFile, "system-prompt-file", "", "Read system prompt from a file")
	f.StringVar(&flags.AppendSystemPrompt, "append-system-prompt", "", "Append to the default system prompt")
	f.StringVar(&flags.AppendSystemPromptFile, "append-system-prompt-file", "", "Read system prompt append from a file")

	// CLI-07: Agent flags
	f.StringVar(&flags.Agent, "agent", "", "Agent for the current session")
	f.StringVar(&flags.Agents, "agents", "", "JSON object defining custom agents")
	f.StringVar(&flags.AgentID, "agent-id", "", "Agent ID for the session")
	f.StringVar(&flags.AgentName, "agent-name", "", "Agent name for the session")
	f.BoolVar(&flags.PlanModeRequired, "plan-mode-required", false, "Require plan mode for the session")
	f.BoolVar(&flags.Proactive, "proactive", false, "Enable proactive mode")
	f.BoolVar(&flags.Brief, "brief", false, "Enable brief mode")

	// CLI-08: Permission flags
	f.BoolVar(&flags.DangerouslySkipPermissions, "dangerously-skip-permissions", false, "Bypass all permission checks (sandboxes only)")
	f.BoolVar(&flags.AllowDangerouslySkipPermissions, "allow-dangerously-skip-permissions", false, "Enable option to bypass permission checks")
	f.StringVar(&flags.PermissionPromptTool, "permission-prompt-tool", "", "MCP tool to use for permission prompts")

	// CLI-09: Tool/plugin/settings flags
	f.StringSliceVar(&flags.Tools, "tools", nil, "Specify available tools from built-in set")
	f.BoolVar(&flags.StrictMCPConfig, "strict-mcp-config", false, "Only use MCP servers from --mcp-config")
	f.StringArrayVar(&flags.PluginDir, "plugin-dir", nil, "Load plugins from directory (repeatable)")
	f.BoolVar(&flags.DisableSlashCommands, "disable-slash-commands", false, "Disable all slash commands/skills")
	f.StringVar(&flags.Settings, "settings", "", "Path to settings JSON file or JSON string")
	f.StringSliceVar(&flags.AddDir, "add-dir", nil, "Additional directories to allow tool access to")
	f.BoolVar(&flags.IDE, "ide", false, "Automatically connect to IDE on startup")

	return cmd
}

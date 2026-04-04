package cli

import (
	"context"
	"fmt"

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
			}

			return app.Run(ctx, runParams, appCtx.Config, appCtx.Settings)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
	cmd.AddCommand(mcpCmd)
	cmd.AddCommand(newCompletionCmd())

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

	// Register all flags matching TypeScript main.tsx
	f := cmd.Flags()
	f.StringVarP(&flags.Model, "model", "m", "", "Model to use (overrides ANTHROPIC_MODEL)")
	f.StringVar(&flags.PermissionMode, "permission-mode", "default", "Permission mode: default, plan, auto, bypass")
	f.BoolVarP(&flags.Resume, "resume", "r", false, "Resume previous conversation")
	f.StringVar(&flags.SessionID, "session-id", "", "Specific session ID to resume")
	f.BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose output")
	f.IntVar(&flags.MaxTurns, "max-turns", 0, "Maximum conversation turns (0 = unlimited)")
	f.StringVar(&flags.SystemPrompt, "system-prompt", "", "System prompt override")
	f.StringVar(&flags.OutputFormat, "output-format", "text", "Output format: text, json, stream-json")
	f.StringSliceVar(&flags.AllowedTools, "allowed-tools", nil, "Tool allowlist (comma-separated)")
	f.StringSliceVar(&flags.DisallowedTools, "disallowed-tools", nil, "Tool denylist (comma-separated)")
	f.StringVar(&flags.MCPConfig, "mcp-config", "", "MCP configuration file path")

	return cmd
}

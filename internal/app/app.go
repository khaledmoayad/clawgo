// Package app wires together all ClawGo subsystems: CLI flags, config,
// API client, tool registry, permissions, cost tracking, session management,
// and the query loop. It provides the main Run() entry point and both
// interactive REPL and non-interactive execution modes.
package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cli/input"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/commands/all"
	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/git"
	"github.com/khaledmoayad/clawgo/internal/mcp"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/session"
	"github.com/khaledmoayad/clawgo/internal/systemprompt"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tools/agent"
	"github.com/khaledmoayad/clawgo/internal/tools/askuser"
	"github.com/khaledmoayad/clawgo/internal/tools/bash"
	"github.com/khaledmoayad/clawgo/internal/tools/brief"
	"github.com/khaledmoayad/clawgo/internal/tools/configtool"
	"github.com/khaledmoayad/clawgo/internal/tools/croncreate"
	"github.com/khaledmoayad/clawgo/internal/tools/crondelete"
	"github.com/khaledmoayad/clawgo/internal/tools/cronlist"
	"github.com/khaledmoayad/clawgo/internal/tools/edit"
	"github.com/khaledmoayad/clawgo/internal/tools/enterplanmode"
	"github.com/khaledmoayad/clawgo/internal/tools/enterworktree"
	"github.com/khaledmoayad/clawgo/internal/tools/exitplanmode"
	"github.com/khaledmoayad/clawgo/internal/tools/exitworktree"
	"github.com/khaledmoayad/clawgo/internal/tools/glob"
	"github.com/khaledmoayad/clawgo/internal/tools/grep"
	"github.com/khaledmoayad/clawgo/internal/tools/listmcpresources"
	"github.com/khaledmoayad/clawgo/internal/tools/lsp"
	"github.com/khaledmoayad/clawgo/internal/tools/notebookedit"
	"github.com/khaledmoayad/clawgo/internal/tools/powershell"
	"github.com/khaledmoayad/clawgo/internal/tools/read"
	"github.com/khaledmoayad/clawgo/internal/tools/readmcpresource"
	"github.com/khaledmoayad/clawgo/internal/tools/sendmessage"
	"github.com/khaledmoayad/clawgo/internal/tools/skill"
	"github.com/khaledmoayad/clawgo/internal/tools/sleep"
	"github.com/khaledmoayad/clawgo/internal/tools/syntheticoutput"
	"github.com/khaledmoayad/clawgo/internal/tools/taskcreate"
	"github.com/khaledmoayad/clawgo/internal/tools/taskget"
	"github.com/khaledmoayad/clawgo/internal/tools/tasklist"
	"github.com/khaledmoayad/clawgo/internal/tools/taskoutput"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
	"github.com/khaledmoayad/clawgo/internal/tools/taskstop"
	"github.com/khaledmoayad/clawgo/internal/tools/taskupdate"
	"github.com/khaledmoayad/clawgo/internal/tools/teamcreate"
	"github.com/khaledmoayad/clawgo/internal/tools/teamdelete"
	"github.com/khaledmoayad/clawgo/internal/tools/todowrite"
	"github.com/khaledmoayad/clawgo/internal/tools/toolsearch"
	"github.com/khaledmoayad/clawgo/internal/tools/webfetch"
	"github.com/khaledmoayad/clawgo/internal/tools/websearch"
	"github.com/khaledmoayad/clawgo/internal/tools/write"
)

// BuildRegistry creates a tool registry with all built-in tools.
// The client parameter may be nil when no API client is available
// (e.g., MCP server mode). When nil, tools that require a client
// (like AgentTool) are omitted.
func BuildRegistry(client *api.Client) *tools.Registry {
	taskStore := tasks.NewStore()
	registry := tools.NewRegistry(
		// Core 6 tools
		bash.New(),
		read.New(),
		write.New(),
		edit.New(),
		grep.New(),
		glob.New(),
		// Web tools
		webfetch.New(),
		websearch.New(),
		// Notebook
		notebookedit.New(),
		// Utility tools
		todowrite.New(),
		askuser.New(),
		brief.New(),
		configtool.New(),
		skill.New(),
		enterplanmode.New(),
		exitplanmode.New(),
		sleep.New(),
		// Task management tools (share a task store)
		taskcreate.New(taskStore),
		taskget.New(taskStore),
		taskupdate.New(taskStore),
		tasklist.New(taskStore),
		taskstop.New(taskStore),
		taskoutput.New(taskStore),
		// Communication / MCP tools
		lsp.New(),
		sendmessage.New(nil),
		listmcpresources.New(),
		readmcpresource.New(),
		// Worktree tools
		enterworktree.New(),
		exitworktree.New(),
		// Team tools
		teamcreate.New(nil),
		teamdelete.New(nil),
		// Platform tools
		powershell.New(),
		syntheticoutput.New(),
		// Cron tools
		croncreate.New(),
		crondelete.New(),
		cronlist.New(),
	)
	// Tools that need registry/client references are registered after construction.
	// Agent tool requires an API client -- skip in headless modes (MCP server).
	if client != nil {
		registry.Register(agent.New(registry, client))
	}
	registry.Register(toolsearch.New(registry))
	return registry
}

// RunParams holds parameters for the main Run entry point.
// These are populated by the CLI layer from CLIFlags.
type RunParams struct {
	// Core flags
	Model              string
	PermissionMode     string
	Resume             bool
	SessionID          string
	MaxTurns           int
	SystemPrompt       string // --system-prompt flag (overrides default prompt)
	AppendSystemPrompt string // --append-system-prompt flag (appended after default)
	OutputFormat       string
	AllowedTools       []string
	DisallowedTools    []string
	Prompt             string
	Version            string

	// CLI-01: Print mode
	Print bool

	// CLI-02: Session flags
	Continue             bool
	ForkSession          bool
	ResumeSessionAt      string
	Name                 string
	Prefill              string
	FromPR               string
	NoSessionPersistence bool

	// CLI-03: Model/performance
	Effort        string
	Thinking      string
	MaxBudgetUSD  float64
	FallbackModel string
	Betas         []string
	TaskBudget    int

	// CLI-04: Debug
	Debug     bool
	DebugFile string
	Bare      bool

	// CLI-05: Output
	JSONSchema             string
	InputFormat            string
	IncludeHookEvents      bool
	IncludePartialMessages bool
	ReplayUserMessages     bool

	// CLI-06: System prompt
	SystemPromptFile       string
	AppendSystemPromptFile string

	// CLI-07: Agent
	Agent            string
	Agents           string
	AgentID          string
	AgentName        string
	PlanModeRequired bool
	Proactive        bool
	Brief            bool

	// CLI-08: Permissions
	DangerouslySkipPermissions      bool
	AllowDangerouslySkipPermissions bool
	PermissionPromptTool            string

	// CLI-09: Tool/plugin/settings
	Tools                []string
	StrictMCPConfig      bool
	PluginDir            []string
	DisableSlashCommands bool
	Settings             string
	AddDir               []string
	IDE                  bool
}

// Run is the main application entry point called from CLI.
// It initializes all subsystems and dispatches to either interactive
// REPL mode or non-interactive single-query mode.
func Run(ctx context.Context, params *RunParams, cfg *config.Config, settings *config.Settings) error {
	// --bare: set CLAUDE_CODE_SIMPLE=1 so all simple-mode gates fire
	if params.Bare {
		os.Setenv("CLAUDE_CODE_SIMPLE", "1")
	}

	// 1. Resolve API key
	apiKey := config.ResolveAPIKey(cfg)
	if apiKey == "" {
		return fmt.Errorf("no API key found. Set ANTHROPIC_API_KEY env var or add to ~/.claude/.credentials.json")
	}

	// 2. Create API client
	baseURL := config.Env(config.EnvBaseURL)
	if alt := config.Env(config.EnvAPIBaseURL); alt != "" {
		baseURL = alt
	}
	client, err := api.NewClient(apiKey, baseURL)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Override model from params or settings
	if params.Model != "" {
		client.Model = params.Model
	} else if settings.Model != "" {
		client.Model = settings.Model
	}

	// --fallback-model: override the auto-detected fallback
	if params.FallbackModel != "" {
		client.FallbackModel = params.FallbackModel
	}

	// 3. Build tool registry (37 tools)
	registry := BuildRegistry(client)

	// 4. Build command registry (44 commands)
	cmdRegistry := commands.NewCommandRegistry()
	all.RegisterAll(cmdRegistry)

	// 5. Set up permissions
	mode := permissions.ModeFromString(params.PermissionMode)
	if settings.PermissionMode != "" && params.PermissionMode == "default" {
		mode = permissions.ModeFromString(settings.PermissionMode)
	}

	// --dangerously-skip-permissions: force bypass mode
	if params.DangerouslySkipPermissions {
		mode = permissions.ModeBypass
	}

	// Merge allowed/disallowed tools from params and settings
	allowedTools := settings.AllowedTools
	if len(params.AllowedTools) > 0 {
		allowedTools = append(allowedTools, params.AllowedTools...)
	}
	disallowedTools := settings.DisallowedTools
	if len(params.DisallowedTools) > 0 {
		disallowedTools = append(disallowedTools, params.DisallowedTools...)
	}

	permCtx := permissions.NewPermissionContext(mode, allowedTools, disallowedTools)

	// Build ToolPermissionRules from settings (PERM-05)
	// These rules integrate alwaysAllow/alwaysDeny/alwaysAsk from settings.json
	// and are evaluated before the standard mode-based permission check.
	toolRules := &permissions.ToolPermissionRules{
		AlwaysAllow: allowedTools,
		AlwaysDeny:  disallowedTools,
		AlwaysAsk:   []string{}, // default empty
	}
	permCtx.ToolRules = toolRules

	// 6. Set up cost tracker
	costTracker := cost.NewTracker(client.Model)

	// 6b. Initialize MCP manager from settings
	mcpManager := mcp.NewManager()
	if len(settings.MCPServers) > 0 {
		mcpConfigs, err := mcp.ParseMCPServers(settings.MCPServers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse MCP server configs: %v\n", err)
		} else {
			// Filter by enterprise policy before connecting
			var allowed []mcp.MCPServerConfig
			for _, cfg := range mcpConfigs {
				decision := mcp.EvaluateServerPolicy(cfg.Name, settings)
				switch decision {
				case mcp.PolicyAllowed:
					allowed = append(allowed, cfg)
				case mcp.PolicyDenied:
					fmt.Fprintf(os.Stderr, "MCP server %q blocked by enterprise policy\n", cfg.Name)
				case mcp.PolicyDisabled:
					// silently skip
				}
			}
			mcpManager.ConnectAll(ctx, allowed)
		}
	}
	defer mcpManager.Close()

	// 7. Determine working directory
	workDir, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()

	// 8. Build effective system prompt using the multi-section prompt builder (SYS-01, SYS-04)
	//
	// --system-prompt-file: read file contents as system prompt override
	systemPromptOverride := params.SystemPrompt
	if params.SystemPromptFile != "" {
		data, err := os.ReadFile(params.SystemPromptFile)
		if err != nil {
			return fmt.Errorf("failed to read --system-prompt-file %q: %w", params.SystemPromptFile, err)
		}
		systemPromptOverride = string(data)
	}

	// --append-system-prompt-file: read file contents to append
	appendPrompt := params.AppendSystemPrompt
	if params.AppendSystemPromptFile != "" {
		data, err := os.ReadFile(params.AppendSystemPromptFile)
		if err != nil {
			return fmt.Errorf("failed to read --append-system-prompt-file %q: %w", params.AppendSystemPromptFile, err)
		}
		appendPrompt = string(data)
	}

	promptCfg := systemprompt.EffectivePromptConfig{
		OverridePrompt: systemPromptOverride,
		AppendPrompt:   appendPrompt,
		MemoryWorkDir:  workDir,
		MemoryHomeDir:  homeDir,
	}

	// Fill environment info for dynamic sections
	promptCfg.BaseConfig = systemprompt.SystemPromptConfig{
		EnvInfo: systemprompt.EnvInfoConfig{
			WorkDir:  workDir,
			Platform: runtime.GOOS,
			Shell:    os.Getenv("SHELL"),
			ModelID:  client.Model,
		},
		KeepCodingInstr: true,
		UseGlobalCache:  true,
		SimpleMode:      os.Getenv("CLAUDE_CODE_SIMPLE") == "true" || os.Getenv("CLAUDE_CODE_SIMPLE") == "1",
	}

	// Fill git info if available
	if gitInfo, err := git.Status(context.Background(), workDir); err == nil {
		promptCfg.BaseConfig.EnvInfo.IsGitRepo = true
		_ = gitInfo // Branch info included in IsGitRepo flag for env section
	}

	systemPromptSections := systemprompt.BuildEffectiveSystemPrompt(promptCfg)

	// Also compute a joined string for subsystems that need a single string (compact, commands)
	systemPromptJoined := strings.Join(systemPromptSections, "\n\n")

	// 9. Session setup
	sessionID := params.SessionID
	var existingMessages []api.Message

	// --continue: resume the most recent conversation (like Resume with no session ID)
	if params.Continue {
		entries, sid, err := session.Resume(workDir, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resume latest session: %v\n", err)
		} else {
			sessionID = sid
			existingMessages = session.EntriesToMessages(entries)
			// --fork-session: use a new session ID instead of reusing the original
			if params.ForkSession {
				sessionID = session.NewSessionID()
			}
		}
	} else if params.Resume {
		entries, sid, err := session.Resume(workDir, sessionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resume session: %v\n", err)
		} else {
			sessionID = sid
			existingMessages = session.EntriesToMessages(entries)
			// --fork-session: use a new session ID instead of reusing the original
			if params.ForkSession {
				sessionID = session.NewSessionID()
			}
		}
	}
	if sessionID == "" {
		sessionID = session.NewSessionID()
	}

	// 10. Build StreamConfig for API request augmentation (API-01, API-02, API-04)
	streamCfg := api.StreamConfig{
		Provider:     api.GetProvider(),
		CacheControl: true,
	}

	// --effort: set effort level on stream config
	if params.Effort != "" {
		streamCfg.Effort = api.EffortLevel(params.Effort)
	}

	// --thinking: configure thinking mode
	// Only enable thinking for models that support it (Opus family)
	modelSupportsThinking := strings.Contains(client.Model, "opus")
	switch params.Thinking {
	case "disabled":
		streamCfg.Thinking = nil
	case "enabled", "adaptive":
		if modelSupportsThinking {
			streamCfg.Thinking = &api.ThinkingConfig{Adaptive: true}
		}
	default:
		// Default: enable adaptive thinking for Opus models unless disabled
		if modelSupportsThinking && os.Getenv("CLAUDE_CODE_DISABLE_THINKING") != "true" {
			streamCfg.Thinking = &api.ThinkingConfig{
				Adaptive: true,
			}
		}
	}

	// Set request headers for correlation and identification
	streamCfg.Headers = &api.RequestHeaders{
		UserAgent: "ClawGo/" + params.Version,
		SessionID: sessionID,
	}

	// 11. Dispatch to interactive or non-interactive mode
	// --print or prompt arg triggers non-interactive mode
	isNonInteractive := params.Print || params.Prompt != ""
	if isNonInteractive {
		prompt := params.Prompt
		if prompt == "" && params.Print {
			// Read from stdin based on input format
			promptText, err := input.ReadPrompt(os.Stdin, params.InputFormat)
			if err != nil {
				return fmt.Errorf("read prompt from stdin: %w", err)
			}
			prompt = strings.TrimSpace(promptText)
		}
		if prompt == "" {
			return fmt.Errorf("no prompt provided. Use: clawgo -p 'your question' or pipe input")
		}

		return RunNonInteractive(ctx, &NonInteractiveParams{
			Client:               client,
			Registry:             registry,
			PermCtx:              permCtx,
			CostTracker:          costTracker,
			Messages:             existingMessages,
			SystemPromptSections: systemPromptSections,
			SystemPrompt:         systemPromptJoined,
			StreamConfig:         streamCfg,
			MaxTurns:             params.MaxTurns,
			WorkingDir:           workDir,
			SessionID:            sessionID,
			Prompt:               prompt,
			OutputFormat:         params.OutputFormat,
			CmdRegistry:          cmdRegistry,
			ToolRules:            toolRules,
			MCPManager:           mcpManager,
			// CLI-05 output control
			IncludeHookEvents:      params.IncludeHookEvents,
			IncludePartialMessages: params.IncludePartialMessages,
			ReplayUserMessages:     params.ReplayUserMessages,
			JSONSchema:             params.JSONSchema,
			// Budget control
			MaxBudgetUSD: params.MaxBudgetUSD,
			// Session persistence (SDK-03)
			NoSessionPersistence: params.NoSessionPersistence,
			// Model info
			Model: client.Model,
		})
	}

	return LaunchREPL(ctx, &REPLParams{
		Client:               client,
		Registry:             registry,
		PermCtx:              permCtx,
		CostTracker:          costTracker,
		Messages:             existingMessages,
		SystemPromptSections: systemPromptSections,
		SystemPrompt:         systemPromptJoined,
		StreamConfig:         streamCfg,
		MaxTurns:             params.MaxTurns,
		WorkingDir:           workDir,
		ProjectRoot:          workDir,
		SessionID:            sessionID,
		Version:              params.Version,
		Model:                client.Model,
		CmdRegistry:          cmdRegistry,
		ToolRules:            toolRules,
		MCPManager:           mcpManager,
	})
}

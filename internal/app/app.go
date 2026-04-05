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
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/commands/all"
	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/git"
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
}

// Run is the main application entry point called from CLI.
// It initializes all subsystems and dispatches to either interactive
// REPL mode or non-interactive single-query mode.
func Run(ctx context.Context, params *RunParams, cfg *config.Config, settings *config.Settings) error {
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

	// 7. Determine working directory
	workDir, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()

	// 8. Build effective system prompt using the multi-section prompt builder (SYS-01, SYS-04)
	promptCfg := systemprompt.EffectivePromptConfig{
		OverridePrompt: params.SystemPrompt,       // --system-prompt flag
		AppendPrompt:   params.AppendSystemPrompt,  // --append-system-prompt flag
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
		SimpleMode:      os.Getenv("CLAUDE_CODE_SIMPLE") == "true",
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

	if params.Resume {
		entries, sid, err := session.Resume(workDir, sessionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not resume session: %v\n", err)
		} else {
			sessionID = sid
			existingMessages = session.EntriesToMessages(entries)
		}
	}
	if sessionID == "" {
		sessionID = session.NewSessionID()
	}

	// 10. Dispatch to interactive or non-interactive mode
	if params.Prompt != "" {
		return RunNonInteractive(ctx, &NonInteractiveParams{
			Client:               client,
			Registry:             registry,
			PermCtx:              permCtx,
			CostTracker:          costTracker,
			Messages:             existingMessages,
			SystemPromptSections: systemPromptSections,
			SystemPrompt:         systemPromptJoined,
			MaxTurns:             params.MaxTurns,
			WorkingDir:           workDir,
			SessionID:            sessionID,
			Prompt:               params.Prompt,
			OutputFormat:         params.OutputFormat,
			CmdRegistry:          cmdRegistry,
			ToolRules:            toolRules,
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
		MaxTurns:             params.MaxTurns,
		WorkingDir:           workDir,
		ProjectRoot:          workDir,
		SessionID:            sessionID,
		Version:              params.Version,
		Model:                client.Model,
		CmdRegistry:          cmdRegistry,
		ToolRules:            toolRules,
	})
}

// Package query implements the agentic conversation loop for ClawGo.
// It orchestrates streaming API calls, tool execution, and conversation
// state management, matching the TypeScript query.ts behavior.
package query

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/compact"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/filestate"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// LoopParams configures the agentic conversation loop.
type LoopParams struct {
	Client               *api.Client
	Registry             *tools.Registry
	PermCtx              *permissions.PermissionContext
	CostTracker          *cost.Tracker
	Messages             []api.Message  // Conversation history
	SystemPromptSections []string       // Multi-section system prompt (sent as separate content blocks)
	SystemPrompt         string         // Joined system prompt string (for compact, backward compat)
	MaxTurns             int            // 0 = unlimited
	WorkingDir           string
	ProjectRoot          string
	SessionID            string

	// Command registry for slash command dispatch
	CmdRegistry *commands.CommandRegistry

	// Per-tool permission rules from settings
	ToolRules *permissions.ToolPermissionRules

	// TUI communication
	Program *tea.Program // For sending messages to TUI

	// Permission communication
	PermissionCh chan permissions.PermissionResult // Receives permission decisions from TUI

	// Non-interactive text output callback
	TextCallback func(string) // Called for each "text" stream event (used by non-interactive mode)

	// API request augmentation
	StreamConfig api.StreamConfig // Betas, thinking, headers, effort, cache control

	// Compaction settings
	AutoCompactEnabled         bool   // Enable auto-compaction when context window fills up
	CompactCustomInstructions  string // Custom instructions included in compaction prompts
	ConsecutiveCompactFailures int    // Circuit breaker state for auto-compaction

	// Hook runner for stop hooks (injected for testability).
	// Nil means no hooks are configured.
	HookRunner HookRunner

	// FallbackModel is the model to switch to when the primary model fails
	// (e.g., FallbackTriggeredError). Empty means no fallback available.
	FallbackModel string

	// TokenBudget is the per-turn output token budget (0 = disabled).
	// When set, the loop auto-continues until the budget is exhausted.
	TokenBudget int

	// AgentID identifies the current agent (empty for main session).
	// Sub-agents skip certain features like token budgets and tool
	// use summaries.
	AgentID string

	// UseStreamingToolExecution enables the StreamingToolExecutor
	// (tools start as soon as their input arrives during streaming).
	// When false, tools are executed after the full response completes.
	UseStreamingToolExecution bool

	// MaxOutputTokensOverride is the initial max output tokens override
	// (e.g., set by a previous session or resume).
	MaxOutputTokensOverride int

	// SmallFastModel overrides the model used for tool use summary
	// generation. Empty uses DefaultSmallFastModel.
	SmallFastModel string

	// FileStateCache tracks file reads for read-before-edit enforcement.
	// If nil, a default cache is created in toolUseContext().
	FileStateCache *filestate.FileStateCache

	// Collapser manages staged context collapses drained before reactive compact.
	// If nil, context collapse is disabled.
	Collapser *compact.ContextCollapser
}

// toolUseContext creates a ToolUseContext for tool execution.
func (p *LoopParams) toolUseContext(ctx context.Context) *tools.ToolUseContext {
	fsc := p.FileStateCache
	if fsc == nil {
		fsc = filestate.NewDefaultFileStateCache()
		p.FileStateCache = fsc
	}
	return &tools.ToolUseContext{
		WorkingDir:     p.WorkingDir,
		ProjectRoot:    p.ProjectRoot,
		SessionID:      p.SessionID,
		AbortCtx:       ctx,
		PermCtx:        p.PermCtx,
		FileStateCache: fsc,
	}
}

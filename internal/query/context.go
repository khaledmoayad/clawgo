// Package query implements the agentic conversation loop for ClawGo.
// It orchestrates streaming API calls, tool execution, and conversation
// state management, matching the TypeScript query.ts behavior.
package query

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/cost"
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
}

// toolUseContext creates a ToolUseContext for tool execution.
func (p *LoopParams) toolUseContext(ctx context.Context) *tools.ToolUseContext {
	return &tools.ToolUseContext{
		WorkingDir:  p.WorkingDir,
		ProjectRoot: p.ProjectRoot,
		SessionID:   p.SessionID,
		AbortCtx:    ctx,
		PermCtx:     p.PermCtx,
	}
}

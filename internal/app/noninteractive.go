package app

import (
	"context"
	"fmt"
	"os"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// NonInteractiveParams holds parameters for non-interactive single-query mode.
type NonInteractiveParams struct {
	Client               *api.Client
	Registry             *tools.Registry
	PermCtx              *permissions.PermissionContext
	CostTracker          *cost.Tracker
	Messages             []api.Message
	SystemPromptSections []string        // Multi-section system prompt (sent as separate content blocks)
	SystemPrompt         string          // Joined system prompt string (for compact)
	StreamConfig         api.StreamConfig // API request augmentation (betas, thinking, headers)
	MaxTurns             int
	WorkingDir           string
	SessionID            string
	Prompt               string
	OutputFormat         string // "text", "json", "stream-json"
	CmdRegistry          *commands.CommandRegistry
	ToolRules            *permissions.ToolPermissionRules
}

// RunNonInteractive executes a single query and prints the result.
// Uses TextCallback for streaming output to stdout (no TUI needed).
func RunNonInteractive(ctx context.Context, params *NonInteractiveParams) error {
	// Add user message
	params.Messages = append(params.Messages, api.UserMessage(params.Prompt))

	// For non-interactive, stream text tokens directly to stdout via TextCallback
	loopParams := &query.LoopParams{
		Client:               params.Client,
		Registry:             params.Registry,
		PermCtx:              params.PermCtx,
		CostTracker:          params.CostTracker,
		Messages:             params.Messages,
		SystemPromptSections: params.SystemPromptSections,
		SystemPrompt:         params.SystemPrompt,
		StreamConfig:         params.StreamConfig,
		MaxTurns:             params.MaxTurns,
		WorkingDir:           params.WorkingDir,
		SessionID:            params.SessionID,
		CmdRegistry:          params.CmdRegistry,
		ToolRules:            params.ToolRules,
		TextCallback: func(text string) { fmt.Print(text) }, // Stream text tokens directly to stdout
		// No Program -- non-interactive mode
		// No PermissionCh -- auto-approve or deny based on mode
	}

	err := query.RunLoop(ctx, loopParams)
	if err != nil {
		return err
	}

	// Newline after streaming output
	fmt.Println()

	// Print cost to stderr
	fmt.Fprintf(os.Stderr, "\n%s\n", cost.FormatUsage(params.CostTracker))

	return nil
}

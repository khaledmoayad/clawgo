package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cli/output"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/session"
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
	MCPManager           any // *mcp.Manager, typed as any to avoid circular imports

	// Output format control (CLI-05, CLI-13)
	Verbose                bool
	IncludeHookEvents      bool
	IncludePartialMessages bool
	ReplayUserMessages     bool
	JSONSchema             string

	// Budget control (CLI-03)
	MaxBudgetUSD float64

	// Session persistence (SDK-03, SDK-04)
	NoSessionPersistence bool // When true, do not persist session to JSONL

	// Model info for output messages
	Model string
}

// RunNonInteractive executes a single query and formats output based on
// OutputFormat. Supports text (default), json, and stream-json formats.
func RunNonInteractive(ctx context.Context, params *NonInteractiveParams) error {
	startTime := time.Now()

	// Load prior session messages if SessionID is provided and session file exists.
	// This enables multi-turn non-interactive usage (e.g., piped scripts).
	if params.SessionID != "" && params.WorkingDir != "" && len(params.Messages) == 0 {
		sessionPath := session.GetSessionPath(params.WorkingDir, params.SessionID)
		if _, err := os.Stat(sessionPath); err == nil {
			entries, err := session.LoadSession(sessionPath)
			if err == nil {
				params.Messages = session.EntriesToMessages(entries)
			}
		}
	}

	// Add user message
	params.Messages = append(params.Messages, api.UserMessage(params.Prompt))

	// Determine output format
	format := params.OutputFormat
	if format == "" {
		format = "text"
	}

	// State tracked across callbacks for building the result
	var lastAssistantText string
	var allMessages []any // For json verbose mode
	var streamWriter *output.StreamJSONWriter
	numTurns := 0
	var lastStopReason string
	var accumulatedErrors []string

	// Build the LoopParams
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
		MCPManager:           params.MCPManager,
	}

	switch format {
	case "text":
		// Stream text tokens directly to stdout (existing behavior)
		loopParams.TextCallback = func(text string) {
			fmt.Print(text)
		}

	case "json":
		// Suppress streaming output; capture everything for final JSON result.
		// TextCallback accumulates the assistant's text for the result field.
		loopParams.TextCallback = func(text string) {
			lastAssistantText += text
		}

		// MessageCallback captures structured messages for verbose mode
		loopParams.MessageCallback = func(role string, content []api.ContentBlock, stopReason string) {
			if role == "assistant" {
				numTurns++
				lastStopReason = stopReason

				// Build content blocks for the verbose messages array
				blocks := apiContentToOutputBlocks(content)
				msg := output.AssistantMessage{
					Type:      output.TypeAssistant,
					SessionID: params.SessionID,
					Message: output.ContentMessage{
						Role:    "assistant",
						Content: blocks,
						Model:   params.Model,
					},
				}
				allMessages = append(allMessages, msg)
			} else if role == "user" {
				// Tool results
				for _, cb := range content {
					if cb.Type == api.ContentToolResult {
						msg := output.ToolResultMsg{
							Type:      output.TypeToolResult,
							ToolUseID: cb.ToolUseID,
							Content:   cb.Content,
							IsError:   cb.IsError,
							SessionID: params.SessionID,
						}
						allMessages = append(allMessages, msg)
					}
				}
			}
		}

	case "stream-json":
		// Write NDJSON messages to stdout during streaming
		streamWriter = output.NewStreamJSONWriter(os.Stdout)

		// Text callback accumulates for the current assistant turn
		var currentText string
		loopParams.TextCallback = func(text string) {
			currentText += text
			lastAssistantText = currentText
		}

		// MessageCallback fires on each complete message
		loopParams.MessageCallback = func(role string, content []api.ContentBlock, stopReason string) {
			if role == "assistant" {
				numTurns++
				lastStopReason = stopReason

				// Write assistant message with all content blocks
				blocks := apiContentToOutputBlocks(content)
				_ = streamWriter.WriteAssistant(blocks, params.SessionID, params.Model)

				// Also emit individual tool_use blocks for structured consumers
				for _, cb := range content {
					if cb.Type == api.ContentToolUse {
						var inputData any
						if cb.Input != nil {
							_ = json.Unmarshal(cb.Input, &inputData)
						}
						_ = streamWriter.WriteToolUse(cb.ID, cb.Name, inputData, params.SessionID)
					}
				}

				// Reset text accumulator for next turn
				currentText = ""
			} else if role == "user" {
				// Tool results
				for _, cb := range content {
					if cb.Type == api.ContentToolResult {
						_ = streamWriter.WriteToolResult(cb.ToolUseID, cb.Content, cb.IsError, params.SessionID)
					}
				}
			}
		}
	}

	// Execute the query loop
	loopErr := query.RunLoop(ctx, loopParams)

	// Calculate duration
	durationMS := time.Since(startTime).Milliseconds()

	// Build usage info from cost tracker
	usage := &output.UsageInfo{
		InputTokens:  params.CostTracker.TotalInputTokens,
		OutputTokens: params.CostTracker.TotalOutputTokens,
	}

	// Determine result subtype and error state
	var subtype output.ResultSubtype
	var isError bool

	switch {
	case loopErr != nil:
		subtype = output.SubtypeError
		isError = true
		accumulatedErrors = append(accumulatedErrors, loopErr.Error())
	case params.MaxTurns > 0 && numTurns >= params.MaxTurns:
		subtype = output.SubtypeErrorMaxTurns
		isError = true
		accumulatedErrors = append(accumulatedErrors, fmt.Sprintf("reached max turns (%d)", params.MaxTurns))
	default:
		subtype = output.SubtypeSuccess
		isError = false
	}

	// For text format, extract last assistant text from accumulated messages
	if format == "text" {
		// Extract text from the last assistant message in the conversation
		for i := len(loopParams.Messages) - 1; i >= 0; i-- {
			msg := loopParams.Messages[i]
			if msg.Role == "assistant" {
				for _, cb := range msg.Content {
					if cb.Type == api.ContentText {
						lastAssistantText = cb.Text
						break
					}
				}
				break
			}
		}
	}

	// Build stop reason pointer
	var stopReasonPtr *string
	if lastStopReason != "" {
		stopReasonPtr = &lastStopReason
	}

	// Build the result message
	result := &output.ResultMessage{
		Type:          output.TypeResult,
		Subtype:       subtype,
		SessionID:     params.SessionID,
		DurationMS:    durationMS,
		DurationAPIMS: durationMS, // Approximation; dedicated API timing tracked in future
		IsError:       isError,
		NumTurns:      numTurns,
		TotalCostUSD:  params.CostTracker.Cost(),
		Usage:         usage,
		StopReason:    stopReasonPtr,
	}

	if isError {
		result.Errors = accumulatedErrors
	} else {
		result.Result = lastAssistantText
	}

	// Format-specific output
	switch format {
	case "text":
		if loopErr != nil {
			return loopErr
		}
		// Newline after streaming output
		fmt.Println()
		// Print cost to stderr
		fmt.Fprintf(os.Stderr, "\n%s\n", cost.FormatUsage(params.CostTracker))

	case "json":
		// Add result message to allMessages for verbose
		allMessages = append(allMessages, result)

		data, err := output.FormatJSON(result, params.Verbose, allMessages)
		if err != nil {
			return fmt.Errorf("format json output: %w", err)
		}
		fmt.Fprintf(os.Stdout, "%s\n", data)

	case "stream-json":
		// Write the final result message to the stream
		if err := streamWriter.WriteResult(result); err != nil {
			return fmt.Errorf("write stream-json result: %w", err)
		}
	}

	// Return loop error for non-text formats too (after output is written)
	if loopErr != nil && format != "text" {
		return nil // Error is captured in the result message; don't double-report
	}

	// Persist session to JSONL if enabled (SDK-03, SDK-04).
	// Uses session.WriteEntry directly to avoid importing the sdk package (circular dep).
	if !params.NoSessionPersistence && params.SessionID != "" && params.WorkingDir != "" {
		sessionPath := session.GetSessionPath(params.WorkingDir, params.SessionID)
		ct := session.NewChainTracker()
		now := time.Now().UTC().Format(time.RFC3339)
		meta := session.SerializedMessage{
			SessionID: params.SessionID,
			Timestamp: now,
			Version:   "1.0.0",
		}
		for _, msg := range loopParams.Messages {
			tm := session.TranscriptFromMessage(msg, ct, meta)
			// Best-effort: don't fail the entire operation if session save fails.
			_ = session.AppendTranscriptMessage(sessionPath, tm)
		}
	}

	return nil
}

// apiContentToOutputBlocks converts api.ContentBlock slices to output.ContentBlock slices.
func apiContentToOutputBlocks(content []api.ContentBlock) []output.ContentBlock {
	blocks := make([]output.ContentBlock, 0, len(content))
	for _, cb := range content {
		block := output.ContentBlock{
			Type: string(cb.Type),
		}
		switch cb.Type {
		case api.ContentText:
			block.Text = cb.Text
		case api.ContentToolUse:
			block.ID = cb.ID
			block.Name = cb.Name
			if cb.Input != nil {
				var inputData any
				_ = json.Unmarshal(cb.Input, &inputData)
				block.Input = inputData
			}
		case api.ContentToolResult:
			block.Text = cb.Content
		}
		blocks = append(blocks, block)
	}
	return blocks
}

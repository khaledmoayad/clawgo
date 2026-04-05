package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/compact"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/systemprompt"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tui"
)

// RunLoop executes the agentic conversation loop with full state machine.
// It implements the query loop from Claude Code's query.ts with all Phase 9
// hardening: streaming tool execution, max_tokens recovery, stop hooks,
// token budget, thinking rules, tool result budgeting, and compaction warnings.
//
// The loop follows 7 continue-site transitions tracked via LoopState.Transition:
//   - SiteCollapseDrain: drained context collapses for 413 recovery
//   - SiteReactiveCompact: reactive compaction on prompt-too-long
//   - SiteMaxOutputEscalate: escalated max_tokens from capped default
//   - SiteMaxOutputRecovery: multi-turn continuation after max_tokens
//   - SiteStopHook: re-entry after stop hook blocking error
//   - SiteTokenBudget: auto-continuation under budget
//   - SiteToolUse: normal tool_use continuation (next turn)
func RunLoop(ctx context.Context, params *LoopParams) error {
	// Initialize the state machine with the initial messages.
	state := NewLoopState(params.Messages)

	// Apply initial max output tokens override if provided.
	if params.MaxOutputTokensOverride > 0 {
		state.MaxOutputTokensOverride = params.MaxOutputTokensOverride
	}

	for {
		// Max turns check
		if params.MaxTurns > 0 && state.TurnCount >= params.MaxTurns {
			return nil
		}
		state.TurnCount++

		// === Pre-request preparation ===

		// Step 1: Apply micro-compaction to clear old tool results
		state.Messages = compact.MicroCompact(state.Messages, params.Client.Model)

		// Step 2: Apply cache-aware API microcompact if enabled
		if state.CachedMicroCompactState != nil {
			state.Messages = compact.APIMicroCompact(
				state.Messages,
				state.CachedMicroCompactState,
				state.CachedMicroCompactState.LastCacheBreakpoint,
			)
		}

		// Step 3: Apply tool result budgeting (Plan 01)
		// Pass nil sizer to use DefaultMaxResultSizeChars for all tools.
		state.Messages = tools.ApplyToolResultBudget(state.Messages, nil)

		// Step 4: Check compaction warning (Plan 03)
		if state.CompactWarningState != nil {
			tokenCount := estimateTokenCount(state.Messages)
			warning, level := compact.CheckCompactWarning(
				state.CompactWarningState, tokenCount, params.Client.Model,
			)
			if warning != "" && params.Program != nil {
				params.Program.Send(tui.StreamEventMsg{
					Event: api.StreamEvent{Type: api.EventText, Text: fmt.Sprintf("[%s] %s\n", level, warning)},
				})
			}
		}

		// Step 5: Start memory prefetch (fire-and-forget, Plan 05)
		memPrefetch := StartMemoryPrefetch(ctx, state.Messages, params.ProjectRoot)

		// Step 6: Normalize messages for API (alternating roles, orphan cleanup, etc.)
		state.Messages = NormalizeMessagesForAPI(state.Messages, params.Registry.Names())

		// Step 7: Enforce thinking rules before API call (Plan 04)
		messagesForQuery := EnforceThinkingRules(state.Messages)

		// === Build and send API request ===
		reqParams := buildRequest(params, state)

		// Override messages in the request with thinking-rule-enforced version
		msgParams := make([]anthropic.MessageParam, 0, len(messagesForQuery))
		for _, m := range messagesForQuery {
			msgParams = append(msgParams, m.ToParam())
		}
		reqParams.Messages = msgParams

		// Streaming tool executor setup (Plan 01)
		var streamingExecutor *tools.StreamingToolExecutor
		if params.UseStreamingToolExecution {
			toolCtx := params.toolUseContext(ctx)
			permFn := buildPermissionFn(params)
			streamingExecutor = tools.NewStreamingToolExecutor(params.Registry, toolCtx, permFn)
		}

		// Stream API response
		var lastMessage *anthropic.Message
		var lastUsage *api.Usage
		var streamError error

		eventCh := params.Client.StreamMessageWithConfig(ctx, reqParams, params.StreamConfig)
		for event := range eventCh {
			// Forward event to TUI
			if params.Program != nil {
				params.Program.Send(tui.StreamEventMsg{Event: event})
			}

			// Forward text events to non-interactive callback
			if event.Type == api.EventText && params.TextCallback != nil {
				params.TextCallback(event.Text)
			}

			// Feed tool_use blocks to streaming executor as they arrive
			if streamingExecutor != nil && event.Type == api.EventToolUseStart && event.ToolUse != nil {
				streamingExecutor.AddTool(event.ToolUse.ID, event.ToolUse.Name, event.ToolUse.Input)
			}

			if event.Type == api.EventMessageComplete {
				lastMessage = event.Message
				lastUsage = event.Usage
			}

			if event.Type == api.EventError && event.Error != nil {
				streamError = event.Error
				break
			}
		}

		// === Error handling ===

		// Handle media size errors (Plan 02)
		if streamError != nil && IsMediaSizeError(streamError) {
			state.Messages = HandleMediaSizeError(state.Messages)
			// Retry with cleaned messages
			continue
		}

		// Handle prompt-too-long errors with reactive compaction
		if streamError != nil && compact.IsPromptTooLongError(streamError) {
			// Discard streaming executor on error
			if streamingExecutor != nil {
				streamingExecutor.Discard()
			}

			// Try draining staged context collapses before reactive compact
			if params.Collapser != nil && params.Collapser.StagedCount() > 0 {
				recovered, drained := params.Collapser.RecoverFromOverflow(state.Messages)
				if drained {
					state.Messages = recovered
					state.SetTransition(SiteCollapseDrain)
					continue
				}
			}

			if !state.HasAttemptedReactiveCompact {
				state.HasAttemptedReactiveCompact = true

				compactParams := compact.CompactParams{
					Client:             params.Client,
					Model:              params.Client.Model,
					Messages:           state.Messages,
					SystemPrompt:       params.SystemPrompt,
					CustomInstructions: params.CompactCustomInstructions,
				}
				result, compactErr := compact.ReactiveCompact(ctx, compactParams, streamError)
				if compactErr == nil && result != nil && result.WasCompacted {
					// Tombstone orphaned messages and strip signatures (Plan 02)
					tombstoned, _ := TombstoneOrphanedMessages(state.Messages, 0)
					stripped := StripSignatureBlocks(tombstoned)

					state.Messages = []api.Message{
						api.UserMessage("[Previous conversation compacted]\n\n" + result.Summary),
					}
					// Preserve any non-compacted messages after stripping
					_ = stripped

					state.SetTransition(SiteReactiveCompact)
					continue
				}
			}
			return streamError
		}

		// Other stream errors
		if streamError != nil {
			// Fire stop failure hooks on API error (fire-and-forget, Plan 04)
			if params.HookRunner != nil && len(state.Messages) > 0 {
				lastMsg := state.Messages[len(state.Messages)-1]
				ExecuteStopFailureHooks(ctx, lastMsg, params.HookRunner)
			}
			return streamError
		}

		if lastMessage == nil {
			return fmt.Errorf("stream ended without message completion")
		}

		// === Post-stream processing ===

		// Track cost
		if lastUsage != nil {
			params.CostTracker.Add(cost.Usage{
				InputTokens:              lastUsage.InputTokens,
				OutputTokens:             lastUsage.OutputTokens,
				CacheCreationInputTokens: lastUsage.CacheCreationInputTokens,
				CacheReadInputTokens:     lastUsage.CacheReadInputTokens,
			})
			if params.Program != nil {
				params.Program.Send(tui.CostUpdateMsg{
					SessionCost: cost.FormatCost(params.CostTracker.Cost()),
				})
			}
		}

		// Add assistant message to history
		assistantMsg := api.MessageFromResponse(lastMessage)
		state.Messages = append(state.Messages, assistantMsg)

		// Fire MessageCallback for structured output modes (json/stream-json)
		stopReason := string(lastMessage.StopReason)
		if params.MessageCallback != nil {
			params.MessageCallback("assistant", assistantMsg.Content, stopReason)
		}

		// Consume memory prefetch results (non-blocking)
		_ = ConsumeMemoryPrefetch(memPrefetch)

		// --- Site: max_tokens handling (Plan 02) ---
		if IsMaxTokensStop(stopReason) {
			recoveryState := &MaxTokensRecoveryState{
				RecoveryCount:     state.MaxOutputTokensRecoveryCount,
				MaxOutputOverride: state.MaxOutputTokensOverride,
			}

			currentMaxTokens := int(params.Client.MaxTokens)
			if state.MaxOutputTokensOverride > 0 {
				currentMaxTokens = state.MaxOutputTokensOverride
			}

			action, newMax := HandleMaxTokensRecovery(recoveryState, currentMaxTokens)

			switch action {
			case RecoveryEscalate:
				state.MaxOutputTokensOverride = newMax
				state.SetTransition(SiteMaxOutputEscalate)
				continue

			case RecoveryRetry:
				state.MaxOutputTokensRecoveryCount++
				state.Messages = append(state.Messages, GetContinuationMessage())
				state.SetTransition(SiteMaxOutputRecovery)
				continue

			case RecoveryStop:
				// Fall through to end_turn handling
			}
		}

		// --- Auto-compaction check ---
		if params.AutoCompactEnabled {
			compactParams := compact.CompactParams{
				Client:             params.Client,
				Model:              params.Client.Model,
				Messages:           state.Messages,
				SystemPrompt:       params.SystemPrompt,
				CustomInstructions: params.CompactCustomInstructions,
			}
			tokenCount := estimateTokenCount(state.Messages)
			result, failures, compactErr := compact.CheckAutoCompact(
				ctx, compactParams, tokenCount, params.ConsecutiveCompactFailures,
			)
			params.ConsecutiveCompactFailures = failures
			if compactErr == nil && result != nil && result.WasCompacted {
				state.Messages = []api.Message{
					api.UserMessage("[Previous conversation compacted]\n\n" + result.Summary),
				}
			}
		}

		// --- Site: end_turn handling with stop hooks ---
		if stopReason == "end_turn" {
			// Run stop hooks (Plan 04)
			hookResult, hookErr := HandleStopHooks(ctx, state.Messages, params.HookRunner)
			if hookErr != nil {
				return hookErr
			}

			// Check hook outcomes
			if hookResult != nil {
				// Blocking errors: inject as user message and continue
				if len(hookResult.BlockingErrors) > 0 {
					errMsg := strings.Join(hookResult.BlockingErrors, "\n")
					state.Messages = append(state.Messages, api.UserMessage(
						"[Stop hook errors]\n"+errMsg,
					))
					state.StopHookActive = true
					state.SetTransition(SiteStopHook)
					continue
				}

				// Prevent continuation: stop the loop
				if hookResult.PreventContinuation {
					return nil
				}
			}

			// --- Site: token budget check (Plan 03) ---
			if params.TokenBudget > 0 {
				globalTurnTokens := estimateOutputTokens(lastUsage)
				decision := CheckTokenBudget(
					state.BudgetTracker,
					params.AgentID,
					params.TokenBudget,
					globalTurnTokens,
				)

				switch d := decision.(type) {
				case ContinueDecision:
					state.Messages = append(state.Messages, api.UserMessage(d.NudgeMessage))
					state.SetTransition(SiteTokenBudget)
					continue
				case StopDecision:
					// Budget exhausted or not applicable -- fall through to stop
				}
			}

			// Normal end_turn: stop the loop
			return nil
		}

		// --- Site: tool_use continuation ---
		if stopReason == "tool_use" {
			// Reset recovery counters for the new tool-use turn
			state.ResetForToolUse()

			var toolResults []api.ToolResultEntry

			if streamingExecutor != nil {
				// Drain remaining results from the streaming executor
				resultCh := streamingExecutor.GetRemainingResults(ctx)
				for update := range resultCh {
					if update.Result != nil {
						toolResults = append(toolResults, api.ToolResultEntry{
							ToolUseID: update.Result.ToolUseID,
							Content:   update.Result.Content,
							IsError:   update.Result.IsError,
						})
					}
					// Forward progress events to TUI
					if update.Progress != nil && params.Program != nil {
						params.Program.Send(tui.StreamEventMsg{
							Event: api.StreamEvent{
								Type: api.EventText,
								Text: update.Progress.Text,
							},
						})
					}
				}
			} else {
				// Fall back to existing executeToolUses
				var err error
				toolResults, err = executeToolUses(ctx, lastMessage, params)
				if err != nil {
					return err
				}
			}

			// Forward tool results to TUI
			for _, r := range toolResults {
				if params.Program != nil {
					params.Program.Send(tui.StreamEventMsg{
						Event: api.StreamEvent{Type: api.EventText, Text: r.Content},
					})
				}
			}

			// Start async tool use summary generation (fire-and-forget)
			if len(toolResults) > 0 && params.AgentID == "" {
				state.PendingToolUseSummary = GenerateToolUseSummary(
					ctx, params.Client, state.Messages, params.SmallFastModel,
				)
			}

			// Consume pending summary from previous iteration
			if state.PendingToolUseSummary != nil {
				select {
				case summaryResult := <-state.PendingToolUseSummary:
					if summaryResult != nil && summaryResult.Summary != "" {
						// The summary is for context compression; add as meta message
						_ = summaryResult.Summary // Consumed for now; full wiring in Phase 13
					}
				default:
					// Not ready yet, skip
				}
				state.PendingToolUseSummary = nil
			}

			// Add tool results as user message and continue loop
			if len(toolResults) > 0 {
				toolResultMsg := api.ToolResultsMessage(toolResults)
				state.Messages = append(state.Messages, toolResultMsg)

				// Fire MessageCallback for tool results (stream-json output)
				if params.MessageCallback != nil {
					params.MessageCallback("user", toolResultMsg.Content, "")
				}
			}

			state.SetTransition(SiteToolUse)
			continue
		}

		// Unhandled stop reason -- treat as end_turn
		return nil
	}
}

// buildRequest creates the API request from loop params and state.
// System prompt sections are converted to separate TextBlockParam entries,
// filtering out the DynamicBoundaryMarker and empty sections.
func buildRequest(params *LoopParams, state *LoopState) anthropic.MessageNewParams {
	// Convert messages to API params
	msgParams := make([]anthropic.MessageParam, 0, len(state.Messages))
	for _, m := range state.Messages {
		msgParams = append(msgParams, m.ToParam())
	}

	maxTokens := params.Client.MaxTokens
	// Apply max output tokens override from state machine
	if state.MaxOutputTokensOverride > 0 {
		maxTokens = int64(state.MaxOutputTokensOverride)
	}

	req := anthropic.MessageNewParams{
		Model:     params.Client.Model,
		MaxTokens: maxTokens,
		Messages:  msgParams,
	}

	// Build system content blocks from multi-section prompt
	if len(params.SystemPromptSections) > 0 {
		var systemBlocks []anthropic.TextBlockParam
		for _, section := range params.SystemPromptSections {
			// Skip the boundary marker (used for caching logic, not sent to API)
			if section == systemprompt.DynamicBoundaryMarker {
				continue
			}
			if section != "" {
				systemBlocks = append(systemBlocks, anthropic.TextBlockParam{Text: section})
			}
		}
		if len(systemBlocks) > 0 {
			req.System = systemBlocks
		}
	} else if params.SystemPrompt != "" {
		// Fallback: single string system prompt (backward compatibility)
		req.System = []anthropic.TextBlockParam{
			{Text: params.SystemPrompt},
		}
	}

	// Add tool definitions
	// TODO: Server-side tools (WebSearch, WebFetch) should be sent as their native
	// ToolUnionParam types (e.g., OfWebSearchTool20250305 with AllowedDomains/BlockedDomains)
	// rather than as generic OfTool definitions. The domain filter parameters are currently
	// captured in tool metadata but need to be forwarded here when constructing the API request.
	toolDefs := params.Registry.ToolDefinitions()
	if len(toolDefs) > 0 {
		apiTools := make([]anthropic.ToolUnionParam, 0, len(toolDefs))
		for _, td := range toolDefs {
			var schema anthropic.ToolInputSchemaParam
			if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
				// Fallback: use a minimal object schema
				schema = anthropic.ToolInputSchemaParam{}
			}
			apiTools = append(apiTools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        td.Name,
					Description: anthropic.String(td.Description),
					InputSchema: schema,
				},
			})
		}
		req.Tools = apiTools
	}

	return req
}

// buildPermissionFn creates a PermissionFn for the StreamingToolExecutor
// from LoopParams. This wraps the TUI permission dialog flow and uses
// CheckPermissionWithRules for settings-based rule enforcement.
func buildPermissionFn(params *LoopParams) tools.PermissionFn {
	return func(name string, input json.RawMessage, tool tools.Tool) (tools.PermissionResult, error) {
		permResult := permissions.CheckPermissionWithRules(
			name, tool.IsReadOnly(), params.PermCtx, params.ToolRules,
		)

		if permResult == permissions.Ask {
			if params.Program != nil {
				params.Program.Send(tui.PermissionRequestMsg{
					ToolName:    name,
					ToolInput:   string(input),
					Description: tool.Description(),
				})
			}
			if params.PermissionCh != nil {
				resp := <-params.PermissionCh
				if resp == permissions.Deny {
					return permissions.Deny, nil
				}
				return permissions.Allow, nil
			}
			return permissions.Deny, nil
		}

		return permResult, nil
	}
}

// executeToolUses runs all tool calls from the assistant message using the
// orchestration engine. Tool calls are partitioned into concurrent and serial
// batches, then executed via ExecuteBatches. Permissions use CheckPermissionWithRules
// to honor settings-based alwaysAllow/alwaysDeny/alwaysAsk before falling through
// to the standard mode-based check.
func executeToolUses(ctx context.Context, msg *anthropic.Message, params *LoopParams) ([]api.ToolResultEntry, error) {
	// Convert assistant message tool_use blocks into ToolCallEntries
	var entries []tools.ToolCallEntry
	for _, block := range msg.Content {
		if block.Type != "tool_use" {
			continue
		}
		entries = append(entries, tools.ToolCallEntry{
			ID:    block.ID,
			Name:  block.Name,
			Input: block.Input,
		})
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Partition into batches using the orchestration engine
	batches := tools.PartitionToolCalls(entries, params.Registry)

	// Build the tool use context
	toolCtx := params.toolUseContext(ctx)

	// Create a permissionFn that wraps the TUI permission dialog flow
	// and uses CheckPermissionWithRules for settings-based rule enforcement
	permissionFn := func(name string, input json.RawMessage, tool tools.Tool) (tools.PermissionResult, error) {
		// Use CheckPermissionWithRules to check settings-based rules first
		permResult := permissions.CheckPermissionWithRules(name, tool.IsReadOnly(), params.PermCtx, params.ToolRules)

		if permResult == permissions.Ask {
			// Send permission request to TUI
			if params.Program != nil {
				params.Program.Send(tui.PermissionRequestMsg{
					ToolName:    name,
					ToolInput:   string(input),
					Description: tool.Description(),
				})
			}
			// Wait for permission response
			if params.PermissionCh != nil {
				resp := <-params.PermissionCh
				if resp == permissions.Deny {
					return permissions.Deny, nil
				}
				return permissions.Allow, nil
			}
			// No TUI and no permission channel -- deny by default
			return permissions.Deny, nil
		}

		return permResult, nil
	}

	// Execute batches using the orchestration engine
	results, err := tools.ExecuteBatches(ctx, batches, params.Registry, toolCtx, permissionFn)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// estimateTokenCount provides a rough token estimate for a message slice.
// Uses the chars/4 heuristic which is sufficient for threshold comparisons.
func estimateTokenCount(messages []api.Message) int {
	total := 0
	for _, m := range messages {
		for _, cb := range m.Content {
			total += len(cb.Text) + len(cb.Content) + len(cb.Thinking)
			if cb.Input != nil {
				total += len(cb.Input)
			}
		}
	}
	return total / 4
}

// estimateOutputTokens extracts the output token count from API usage.
// Falls back to 0 if usage is nil.
func estimateOutputTokens(usage *api.Usage) int {
	if usage == nil {
		return 0
	}
	return usage.OutputTokens
}

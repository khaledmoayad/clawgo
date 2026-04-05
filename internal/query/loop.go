package query

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/compact"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/systemprompt"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tui"
)

// RunLoop executes the agentic conversation loop.
// It streams API responses, executes tool calls, and loops until
// end_turn stop reason or MaxTurns is reached.
func RunLoop(ctx context.Context, params *LoopParams) error {
	turns := 0
	for {
		if params.MaxTurns > 0 && turns >= params.MaxTurns {
			return nil
		}
		turns++

		// Apply micro-compaction to clear old tool results before API call
		params.Messages = compact.MicroCompact(params.Messages, params.Client.Model)

		// Build API request parameters
		reqParams := buildRequest(params)

		// Stream API response
		var lastMessage *anthropic.Message
		var lastUsage *api.Usage

		eventCh := params.Client.StreamMessageWithConfig(ctx, reqParams, params.StreamConfig)
		for event := range eventCh {
			// Forward event to TUI
			if params.Program != nil {
				params.Program.Send(tui.StreamEventMsg{Event: event})
			}
			// Forward text events to non-interactive callback (for streaming stdout)
			if event.Type == api.EventText && params.TextCallback != nil {
				params.TextCallback(event.Text)
			}

			if event.Type == api.EventMessageComplete {
				lastMessage = event.Message
				lastUsage = event.Usage
			}
			if event.Type == api.EventError && event.Error != nil {
				// Check for prompt-too-long errors and attempt reactive compaction
				if compact.IsPromptTooLongError(event.Error) {
					compactParams := compact.CompactParams{
						Client:             params.Client,
						Model:              params.Client.Model,
						Messages:           params.Messages,
						SystemPrompt:       params.SystemPrompt,
						CustomInstructions: params.CompactCustomInstructions,
					}
					result, compactErr := compact.ReactiveCompact(ctx, compactParams, event.Error)
					if compactErr == nil && result != nil && result.WasCompacted {
						params.Messages = []api.Message{
							api.UserMessage("[Previous conversation compacted]\n\n" + result.Summary),
						}
						break // Break out of event loop to retry with compacted messages
					}
				}
				return event.Error
			}
		}

		if lastMessage == nil {
			return fmt.Errorf("stream ended without message completion")
		}

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
		params.Messages = append(params.Messages, assistantMsg)

		// Auto-compaction: check if context window usage exceeds threshold
		if params.AutoCompactEnabled {
			compactParams := compact.CompactParams{
				Client:             params.Client,
				Model:              params.Client.Model,
				Messages:           params.Messages,
				SystemPrompt:       params.SystemPrompt,
				CustomInstructions: params.CompactCustomInstructions,
			}
			tokenCount := estimateTokenCount(params.Messages)
			result, failures, compactErr := compact.CheckAutoCompact(
				ctx, compactParams, tokenCount, params.ConsecutiveCompactFailures,
			)
			params.ConsecutiveCompactFailures = failures
			if compactErr == nil && result != nil && result.WasCompacted {
				params.Messages = []api.Message{
					api.UserMessage("[Previous conversation compacted]\n\n" + result.Summary),
				}
			}
		}

		// Check stop reason
		if lastMessage.StopReason == "end_turn" {
			return nil
		}

		// Extract tool uses and execute them
		toolResults, err := executeToolUses(ctx, lastMessage, params)
		if err != nil {
			return err
		}

		// Add tool results as user message and loop
		if len(toolResults) > 0 {
			params.Messages = append(params.Messages, api.ToolResultsMessage(toolResults))
		}
	}
}

// buildRequest creates the API request from loop params.
// System prompt sections are converted to separate TextBlockParam entries,
// filtering out the DynamicBoundaryMarker and empty sections.
func buildRequest(params *LoopParams) anthropic.MessageNewParams {
	// Convert messages to API params
	msgParams := make([]anthropic.MessageParam, 0, len(params.Messages))
	for _, m := range params.Messages {
		msgParams = append(msgParams, m.ToParam())
	}

	req := anthropic.MessageNewParams{
		Model:     params.Client.Model,
		MaxTokens: params.Client.MaxTokens,
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

	// Forward tool results to TUI
	for _, r := range results {
		if params.Program != nil {
			params.Program.Send(tui.StreamEventMsg{
				Event: api.StreamEvent{Type: api.EventText, Text: r.Content},
			})
		}
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

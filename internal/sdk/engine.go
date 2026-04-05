package sdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/compact"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// QueryEngineConfig configures a QueryEngine instance.
type QueryEngineConfig struct {
	Client       *api.Client
	Registry     *tools.Registry
	SystemPrompt string
	MaxTurns     int    // 0 = unlimited
	WorkingDir   string
	ProjectRoot  string
	StreamConfig api.StreamConfig // Betas, thinking, headers, effort, cache control
}

// QueryEngine manages agentic conversations programmatically.
// It wraps the streaming API and tool execution loop, emitting events
// on a channel instead of rendering to a TUI. This is the Go equivalent
// of the TypeScript QueryEngine.ts.
type QueryEngine struct {
	config      QueryEngineConfig
	mu          sync.Mutex
	messages    []api.Message
	costTracker *cost.Tracker
	sessionID   string
}

// NewQueryEngine creates a new QueryEngine with the given configuration.
func NewQueryEngine(cfg QueryEngineConfig) *QueryEngine {
	// Generate a random session ID
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	sessionID := hex.EncodeToString(b)

	model := api.DefaultModel
	if cfg.Client != nil {
		model = cfg.Client.Model
	}

	return &QueryEngine{
		config:      cfg,
		messages:    make([]api.Message, 0),
		costTracker: cost.NewTracker(model),
		sessionID:   sessionID,
	}
}

// Ask sends a user message and returns a channel of SDKEvents for the
// conversation turn. The channel is closed when the turn completes or
// an error occurs. Cancelling the context will abort the conversation
// and close the channel.
//
// Events emitted:
//   - EventTextDelta: streaming text from the assistant
//   - EventThinkingDelta: extended thinking content
//   - EventToolUseStart: tool invocation begins
//   - EventToolUseInput: incremental tool input JSON
//   - EventToolUseEnd: tool input complete
//   - EventToolResult: tool execution result
//   - EventCostUpdate: cumulative session cost after each API call
//   - EventTurnComplete: conversation turn finished
//   - EventError: an error occurred
func (e *QueryEngine) Ask(ctx context.Context, userMessage string) <-chan SDKEvent {
	ch := make(chan SDKEvent, 64)

	// Append user message to history
	e.mu.Lock()
	e.messages = append(e.messages, api.UserMessage(userMessage))
	e.mu.Unlock()

	go func() {
		defer close(ch)
		e.runLoop(ctx, ch)
	}()

	return ch
}

// Messages returns a copy of the conversation history.
func (e *QueryEngine) Messages() []api.Message {
	e.mu.Lock()
	defer e.mu.Unlock()
	msgs := make([]api.Message, len(e.messages))
	copy(msgs, e.messages)
	return msgs
}

// SessionCost returns the cumulative cost for this engine's session.
func (e *QueryEngine) SessionCost() float64 {
	return e.costTracker.Cost()
}

// SessionID returns the unique session identifier.
func (e *QueryEngine) SessionID() string {
	return e.sessionID
}

// sendEvent sends an event on the channel, respecting context cancellation.
func sendEvent(ctx context.Context, ch chan<- SDKEvent, evt SDKEvent) bool {
	select {
	case ch <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

// runLoop executes the agentic conversation loop, emitting SDK events.
// This is the SDK equivalent of query.RunLoop, decoupled from the TUI.
func (e *QueryEngine) runLoop(ctx context.Context, ch chan<- SDKEvent) {
	turns := 0
	maxTurns := e.config.MaxTurns

	for {
		if maxTurns > 0 && turns >= maxTurns {
			return
		}
		turns++

		// Apply micro-compaction before API call
		e.mu.Lock()
		e.messages = compact.MicroCompact(e.messages, e.config.Client.Model)
		messages := make([]api.Message, len(e.messages))
		copy(messages, e.messages)
		e.mu.Unlock()

		// Build API request
		reqParams := e.buildRequest(messages)

		// Stream API response
		var lastMessage *anthropic.Message
		var lastUsage *api.Usage
		streamCh := e.config.Client.StreamMessageWithConfig(ctx, reqParams, e.config.StreamConfig)

		for event := range streamCh {
			switch event.Type {
			case api.EventText:
				if !sendEvent(ctx, ch, TextDeltaEvent(event.Text)) {
					return
				}

			case api.EventThinking:
				if !sendEvent(ctx, ch, SDKEvent{
					Type: EventThinkingDelta,
					Text: event.Text,
				}) {
					return
				}

			case api.EventInputJSON:
				if !sendEvent(ctx, ch, SDKEvent{
					Type: EventToolUseInput,
					Text: event.Text,
				}) {
					return
				}

			case api.EventToolUseStart:
				if event.ToolUse != nil {
					if !sendEvent(ctx, ch, SDKEvent{
						Type:     EventToolUseStart,
						ToolName: event.ToolUse.Name,
						ToolID:   event.ToolUse.ID,
					}) {
						return
					}
				}

			case api.EventToolUseEnd:
				if event.ToolUse != nil {
					if !sendEvent(ctx, ch, SDKEvent{
						Type:      EventToolUseEnd,
						ToolName:  event.ToolUse.Name,
						ToolID:    event.ToolUse.ID,
						ToolInput: event.ToolUse.Input,
					}) {
						return
					}
				}

			case api.EventMessageComplete:
				lastMessage = event.Message
				lastUsage = event.Usage

			case api.EventError:
				if event.Error != nil {
					if !sendEvent(ctx, ch, ErrorEvent(event.Error)) {
						return
					}
					return
				}
			}
		}

		// Check context after stream ends
		if ctx.Err() != nil {
			return
		}

		if lastMessage == nil {
			sendEvent(ctx, ch, ErrorEvent(fmt.Errorf("stream ended without message completion")))
			return
		}

		// Track cost
		if lastUsage != nil {
			e.costTracker.Add(cost.Usage{
				InputTokens:              lastUsage.InputTokens,
				OutputTokens:             lastUsage.OutputTokens,
				CacheCreationInputTokens: lastUsage.CacheCreationInputTokens,
				CacheReadInputTokens:     lastUsage.CacheReadInputTokens,
			})
			sendEvent(ctx, ch, SDKEvent{
				Type: EventCostUpdate,
				Cost: e.costTracker.Cost(),
			})
		}

		// Add assistant message to history
		assistantMsg := api.MessageFromResponse(lastMessage)
		e.mu.Lock()
		e.messages = append(e.messages, assistantMsg)
		e.mu.Unlock()

		// Check stop reason -- if end_turn, we're done
		if lastMessage.StopReason == "end_turn" {
			sendEvent(ctx, ch, TurnCompleteEvent(&assistantMsg, e.costTracker.Cost()))
			return
		}

		// Execute tool uses
		toolResults, err := e.executeToolUses(ctx, lastMessage, ch)
		if err != nil {
			sendEvent(ctx, ch, ErrorEvent(err))
			return
		}

		// Add tool results and loop
		if len(toolResults) > 0 {
			toolResultMsg := api.ToolResultsMessage(toolResults)
			e.mu.Lock()
			e.messages = append(e.messages, toolResultMsg)
			e.mu.Unlock()
		}
	}
}

// buildRequest creates the API request from engine state.
func (e *QueryEngine) buildRequest(messages []api.Message) anthropic.MessageNewParams {
	msgParams := make([]anthropic.MessageParam, 0, len(messages))
	for _, m := range messages {
		msgParams = append(msgParams, m.ToParam())
	}

	req := anthropic.MessageNewParams{
		Model:     e.config.Client.Model,
		MaxTokens: e.config.Client.MaxTokens,
		Messages:  msgParams,
	}

	if e.config.SystemPrompt != "" {
		req.System = []anthropic.TextBlockParam{
			{Text: e.config.SystemPrompt},
		}
	}

	// Add tool definitions
	if e.config.Registry != nil {
		toolDefs := e.config.Registry.ToolDefinitions()
		if len(toolDefs) > 0 {
			apiTools := make([]anthropic.ToolUnionParam, 0, len(toolDefs))
			for _, td := range toolDefs {
				var schema anthropic.ToolInputSchemaParam
				if err := json.Unmarshal(td.InputSchema, &schema); err != nil {
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
	}

	return req
}

// executeToolUses processes tool_use blocks from the assistant response.
// SDK mode auto-approves all tool permissions (no TUI prompts).
func (e *QueryEngine) executeToolUses(ctx context.Context, msg *anthropic.Message, ch chan<- SDKEvent) ([]api.ToolResultEntry, error) {
	if e.config.Registry == nil {
		return nil, nil
	}

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

	// Partition into batches
	batches := tools.PartitionToolCalls(entries, e.config.Registry)

	// Build tool use context
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  e.config.WorkingDir,
		ProjectRoot: e.config.ProjectRoot,
		SessionID:   e.sessionID,
		AbortCtx:    ctx,
		PermCtx: &permissions.PermissionContext{
			Mode: permissions.ModeAuto,
		},
	}

	// SDK mode: auto-approve all tool permissions
	permissionFn := func(name string, input json.RawMessage, tool tools.Tool) (tools.PermissionResult, error) {
		return tools.PermissionAllow, nil
	}

	// Execute batches
	results, err := tools.ExecuteBatches(ctx, batches, e.config.Registry, toolCtx, permissionFn)
	if err != nil {
		return nil, err
	}

	// Emit tool result events
	for _, r := range results {
		sendEvent(ctx, ch, SDKEvent{
			Type:       EventToolResult,
			ToolID:     r.ToolUseID,
			ToolResult: r.Content,
			IsError:    r.IsError,
		})
	}

	return results, nil
}

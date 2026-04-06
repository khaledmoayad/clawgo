package api

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	EventText            StreamEventType = "text"
	EventThinking        StreamEventType = "thinking"
	EventInputJSON       StreamEventType = "input_json"
	EventToolUseStart    StreamEventType = "tool_use_start"
	EventToolUseEnd      StreamEventType = "tool_use_end"
	EventMessageDelta    StreamEventType = "message_delta"
	EventMessageComplete StreamEventType = "message_complete"
	EventError           StreamEventType = "error"
)

// StreamEvent represents a single event from the streaming API.
type StreamEvent struct {
	Type       StreamEventType    // Event type
	Text       string             // For text/thinking/input_json deltas
	ToolUse    *ToolUseBlock      // For tool_use_start/tool_use_end
	StopReason string             // For message_delta
	Usage      *Usage             // For message_complete
	Message    *anthropic.Message // Full accumulated message on completion
	Error      error              // For error events
}

// ToolUseBlock holds tool invocation data from streaming events.
type ToolUseBlock struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// StreamConfig holds optional configuration for stream requests.
// This allows callers to inject betas, thinking params, custom headers,
// effort level, and cache control without modifying the base MessageNewParams.
type StreamConfig struct {
	// Provider determines which beta set to inject.
	Provider ProviderType
	// Thinking configures extended thinking parameters.
	Thinking *ThinkingConfig
	// Headers configures custom request headers.
	Headers *RequestHeaders
	// Effort configures the effort/quality level for the model.
	Effort EffortLevel
	// CacheControl enables ephemeral cache control on system blocks.
	CacheControl bool
	// IsOAuth indicates the client is using a Claude.ai OAuth token
	// (Authorization: Bearer sk-ant-oat01-...). When true, the
	// "oauth-2025-04-20" beta header is added so the API accepts Bearer auth.
	IsOAuth bool
}

// StreamMessage sends a streaming API request and returns events on a channel.
// The channel is closed when the stream completes or errors.
// Uses client.SDK.Messages.NewStreaming() internally.
func (c *Client) StreamMessage(ctx context.Context, params anthropic.MessageNewParams) <-chan StreamEvent {
	return c.StreamMessageWithConfig(ctx, params, StreamConfig{})
}

// StreamMessageWithConfig sends a streaming API request with additional configuration
// for betas, thinking, headers, effort, and cache control.
func (c *Client) StreamMessageWithConfig(ctx context.Context, params anthropic.MessageNewParams, cfg StreamConfig) <-chan StreamEvent {
	ch := make(chan StreamEvent, 64)

	// Collect request options for SDK call
	var opts []option.RequestOption

	// Inject beta headers
	provider := cfg.Provider
	if provider == "" {
		provider = ProviderFirstParty
	}
	betas := GetMessagesBetas(provider)
	// Claude.ai OAuth subscribers require the oauth-2025-04-20 beta header so
	// the API accepts Authorization: Bearer instead of X-Api-Key.
	if cfg.IsOAuth {
		betas = append(betas, BetaOAuth)
	}
	for _, beta := range betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", beta))
	}

	// Inject thinking parameters
	if cfg.Thinking != nil {
		thinkingParam := BuildThinkingParam(*cfg.Thinking)
		if thinkingParam != nil {
			params.Thinking = *thinkingParam
		}
	}

	// Inject effort parameter
	if cfg.Effort != "" {
		params.OutputConfig.Effort = cfg.Effort.ToOutputConfigEffort()
	}

	// Apply cache control to system blocks
	if cfg.CacheControl && len(params.System) > 0 {
		AddCacheBreakpoints(params.System)
	}

	// Inject custom headers
	if cfg.Headers != nil {
		headerOpts := InjectCustomHeadersAsOptions(*cfg.Headers)
		opts = append(opts, headerOpts...)
	}

	go func() {
		defer close(ch)

		stream := c.SDK.Messages.NewStreaming(ctx, params, opts...)
		msg := anthropic.Message{}

		// Track current content block index for tool_use_end
		var currentBlockIndex int

		for stream.Next() {
			event := stream.Current()
			if err := msg.Accumulate(event); err != nil {
				ch <- StreamEvent{Type: EventError, Error: err}
				return
			}

			switch ev := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				currentBlockIndex = int(ev.Index)
				_ = currentBlockIndex // Track for potential use
				switch delta := ev.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					ch <- StreamEvent{Type: EventText, Text: delta.Text}
				case anthropic.InputJSONDelta:
					ch <- StreamEvent{Type: EventInputJSON, Text: delta.PartialJSON}
				case anthropic.ThinkingDelta:
					ch <- StreamEvent{Type: EventThinking, Text: delta.Thinking}
				}

			case anthropic.ContentBlockStartEvent:
				currentBlockIndex = int(ev.Index)
				// Check if the new content block is a tool_use
				if cb := ev.ContentBlock; cb.Type == "tool_use" {
					ch <- StreamEvent{
						Type: EventToolUseStart,
						ToolUse: &ToolUseBlock{
							ID:   cb.ID,
							Name: cb.Name,
						},
					}
				}

			case anthropic.ContentBlockStopEvent:
				// When a content block stops, if it was a tool_use, send the
				// complete input from the accumulated message
				idx := int(ev.Index)
				if idx < len(msg.Content) {
					block := msg.Content[idx]
					if block.Type == "tool_use" {
						ch <- StreamEvent{
							Type: EventToolUseEnd,
							ToolUse: &ToolUseBlock{
								ID:    block.ID,
								Name:  block.Name,
								Input: block.Input,
							},
						}
					}
				}

			case anthropic.MessageDeltaEvent:
				ch <- StreamEvent{
					Type:       EventMessageDelta,
					StopReason: string(ev.Delta.StopReason),
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{Type: EventError, Error: err}
			return
		}

		// Build extended usage from accumulated message
		usage := &Usage{
			InputTokens:              int(msg.Usage.InputTokens),
			OutputTokens:             int(msg.Usage.OutputTokens),
			CacheCreationInputTokens: int(msg.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int(msg.Usage.CacheReadInputTokens),
		}

		ch <- StreamEvent{
			Type:    EventMessageComplete,
			Usage:   usage,
			Message: &msg,
		}
	}()

	return ch
}

// AddCacheBreakpoints applies ephemeral cache control to the last system
// text block, enabling prompt caching. This mirrors Claude Code's behavior
// of adding cache_control: {type: "ephemeral"} to system content blocks.
func AddCacheBreakpoints(system []anthropic.TextBlockParam) {
	if len(system) == 0 {
		return
	}
	// Apply cache control to the last system block
	last := len(system) - 1
	system[last].CacheControl = anthropic.CacheControlEphemeralParam{}
}

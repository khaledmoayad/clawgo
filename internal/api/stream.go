package api

import (
	"context"
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
)

// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	EventText           StreamEventType = "text"
	EventThinking       StreamEventType = "thinking"
	EventInputJSON      StreamEventType = "input_json"
	EventToolUseStart   StreamEventType = "tool_use_start"
	EventToolUseEnd     StreamEventType = "tool_use_end"
	EventMessageDelta   StreamEventType = "message_delta"
	EventMessageComplete StreamEventType = "message_complete"
	EventError          StreamEventType = "error"
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

// StreamMessage sends a streaming API request and returns events on a channel.
// The channel is closed when the stream completes or errors.
// Uses client.SDK.Messages.NewStreaming() internally.
func (c *Client) StreamMessage(ctx context.Context, params anthropic.MessageNewParams) <-chan StreamEvent {
	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)

		stream := c.SDK.Messages.NewStreaming(ctx, params)
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

		// Send the completed message with usage info
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

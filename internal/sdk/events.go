// Package sdk provides the Agent SDK / QueryEngine for programmatic usage.
// It exposes a Go library API that allows consumers to run agentic conversations
// without the TUI, matching the TypeScript QueryEngine.ts patterns.
// The Go equivalent uses channels for streaming events instead of async generators.
package sdk

import (
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// SDKEventType identifies the kind of SDK event emitted during a conversation turn.
type SDKEventType string

const (
	// EventTextDelta is emitted for each text chunk streamed from the API.
	EventTextDelta SDKEventType = "text_delta"

	// EventThinkingDelta is emitted for each thinking chunk from extended thinking.
	EventThinkingDelta SDKEventType = "thinking_delta"

	// EventToolUseStart is emitted when a tool invocation begins.
	EventToolUseStart SDKEventType = "tool_use_start"

	// EventToolUseInput is emitted for incremental tool input JSON chunks.
	EventToolUseInput SDKEventType = "tool_use_input"

	// EventToolUseEnd is emitted when tool input is fully accumulated.
	EventToolUseEnd SDKEventType = "tool_use_end"

	// EventToolResult is emitted when a tool execution completes.
	EventToolResult SDKEventType = "tool_result"

	// EventTurnComplete is emitted when a conversation turn ends.
	EventTurnComplete SDKEventType = "turn_complete"

	// EventCostUpdate is emitted after each API response with cumulative cost.
	EventCostUpdate SDKEventType = "cost_update"

	// EventError is emitted when an error occurs during the conversation.
	EventError SDKEventType = "error"
)

// SDKEvent represents a single event from the QueryEngine during a conversation turn.
// Different fields are populated depending on the event Type.
type SDKEvent struct {
	Type       SDKEventType    // Event type identifier
	Text       string          // For text_delta, thinking_delta, tool_use_input
	ToolName   string          // For tool_use_start/end
	ToolID     string          // For tool_use_start/end/tool_result
	ToolInput  json.RawMessage // For tool_use_end (complete input JSON)
	ToolResult string          // For tool_result
	IsError    bool            // For tool_result, error
	Cost       float64         // For cost_update, turn_complete
	Message    *api.Message    // For turn_complete (full accumulated message)
	Error      error           // For error events
}

// TextDeltaEvent creates an SDKEvent for a text delta.
func TextDeltaEvent(text string) SDKEvent {
	return SDKEvent{
		Type: EventTextDelta,
		Text: text,
	}
}

// ErrorEvent creates an SDKEvent for an error.
func ErrorEvent(err error) SDKEvent {
	return SDKEvent{
		Type:    EventError,
		IsError: true,
		Error:   err,
	}
}

// TurnCompleteEvent creates an SDKEvent for a completed conversation turn.
func TurnCompleteEvent(msg *api.Message, cost float64) SDKEvent {
	return SDKEvent{
		Type:    EventTurnComplete,
		Message: msg,
		Cost:    cost,
	}
}

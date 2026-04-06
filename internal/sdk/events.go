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

	// EventResult is emitted with the final result of a conversation.
	EventResult SDKEventType = "result"

	// EventCompacting is emitted when compaction starts/ends.
	EventCompacting SDKEventType = "compacting"

	// EventUserMessage is emitted when replaying a user message.
	EventUserMessage SDKEventType = "user_message"

	// EventSystemMessage is emitted for system-level messages.
	EventSystemMessage SDKEventType = "system"
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
	IsError    bool            // For tool_result, error, result
	Cost       float64         // For cost_update, turn_complete, result
	Message    *api.Message    // For turn_complete (full accumulated message)
	Error      error           // For error events

	// SDK-02: Extended fields matching TS SDKMessage union
	Result     string // For result events: accumulated assistant text
	SessionID  string // For result events: session identifier
	NumTurns   int    // For result events: number of turns in conversation
	Status     string // For compacting events: "compacting" or ""
	StopReason string // For turn_complete and result events
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

// ResultEvent creates an SDKEvent for the final result of a conversation.
func ResultEvent(result, sessionID string, cost float64, numTurns int, isError bool, stopReason string) SDKEvent {
	return SDKEvent{
		Type:       EventResult,
		Result:     result,
		SessionID:  sessionID,
		Cost:       cost,
		NumTurns:   numTurns,
		IsError:    isError,
		StopReason: stopReason,
	}
}

// CompactingEvent creates an SDKEvent for compaction status changes.
func CompactingEvent(status string) SDKEvent {
	return SDKEvent{
		Type:   EventCompacting,
		Status: status,
	}
}

// UserMessageEvent creates an SDKEvent for replayed user messages.
func UserMessageEvent(text string) SDKEvent {
	return SDKEvent{
		Type: EventUserMessage,
		Text: text,
	}
}

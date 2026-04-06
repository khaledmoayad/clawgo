package sdk

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

func TestSDKEventTypeConstants(t *testing.T) {
	// Verify all expected event type constants are defined
	types := []SDKEventType{
		EventTextDelta,
		EventThinkingDelta,
		EventToolUseStart,
		EventToolUseInput,
		EventToolUseEnd,
		EventToolResult,
		EventTurnComplete,
		EventCostUpdate,
		EventError,
	}
	for _, typ := range types {
		if typ == "" {
			t.Error("SDKEventType constant should not be empty")
		}
	}
}

func TestSDKEventTypeValues(t *testing.T) {
	// Verify the string values match the TypeScript Agent SDK event names
	tests := []struct {
		got  SDKEventType
		want string
	}{
		{EventTextDelta, "text_delta"},
		{EventThinkingDelta, "thinking_delta"},
		{EventToolUseStart, "tool_use_start"},
		{EventToolUseInput, "tool_use_input"},
		{EventToolUseEnd, "tool_use_end"},
		{EventToolResult, "tool_result"},
		{EventTurnComplete, "turn_complete"},
		{EventCostUpdate, "cost_update"},
		{EventError, "error"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("event type = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestSDKEventStruct(t *testing.T) {
	// Verify SDKEvent struct fields exist and are usable
	evt := SDKEvent{
		Type:       EventTextDelta,
		Text:       "hello",
		ToolName:   "bash",
		ToolID:     "tool_123",
		ToolInput:  json.RawMessage(`{"command":"ls"}`),
		ToolResult: "output",
		IsError:    false,
		Cost:       0.001,
		Message:    &api.Message{Role: "assistant"},
		Error:      nil,
	}

	if evt.Type != EventTextDelta {
		t.Errorf("Type = %v, want %v", evt.Type, EventTextDelta)
	}
	if evt.Text != "hello" {
		t.Errorf("Text = %v, want %v", evt.Text, "hello")
	}
	if evt.ToolName != "bash" {
		t.Errorf("ToolName = %v, want %v", evt.ToolName, "bash")
	}
	if evt.ToolID != "tool_123" {
		t.Errorf("ToolID = %v, want %v", evt.ToolID, "tool_123")
	}
	if string(evt.ToolInput) != `{"command":"ls"}` {
		t.Errorf("ToolInput = %s, want %s", evt.ToolInput, `{"command":"ls"}`)
	}
	if evt.ToolResult != "output" {
		t.Errorf("ToolResult = %v, want %v", evt.ToolResult, "output")
	}
	if evt.IsError {
		t.Error("IsError should be false")
	}
	if evt.Cost != 0.001 {
		t.Errorf("Cost = %v, want %v", evt.Cost, 0.001)
	}
	if evt.Message == nil || evt.Message.Role != "assistant" {
		t.Error("Message should be set with role assistant")
	}
}

func TestTextDeltaEvent(t *testing.T) {
	evt := TextDeltaEvent("hello world")
	if evt.Type != EventTextDelta {
		t.Errorf("Type = %v, want %v", evt.Type, EventTextDelta)
	}
	if evt.Text != "hello world" {
		t.Errorf("Text = %v, want %v", evt.Text, "hello world")
	}
}

func TestErrorEvent(t *testing.T) {
	err := errors.New("something failed")
	evt := ErrorEvent(err)
	if evt.Type != EventError {
		t.Errorf("Type = %v, want %v", evt.Type, EventError)
	}
	if evt.Error != err {
		t.Errorf("Error = %v, want %v", evt.Error, err)
	}
	if !evt.IsError {
		t.Error("IsError should be true for error events")
	}
}

func TestTurnCompleteEvent(t *testing.T) {
	msg := &api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: "done"},
		},
	}
	evt := TurnCompleteEvent(msg, 0.05)
	if evt.Type != EventTurnComplete {
		t.Errorf("Type = %v, want %v", evt.Type, EventTurnComplete)
	}
	if evt.Message != msg {
		t.Error("Message should be set to provided message")
	}
	if evt.Cost != 0.05 {
		t.Errorf("Cost = %v, want %v", evt.Cost, 0.05)
	}
}

func TestSDKEventResultEvent(t *testing.T) {
	evt := ResultEvent("Hello world", "session-123", 0.05, 3, false, "end_turn")
	if evt.Type != EventResult {
		t.Errorf("Type = %v, want %v", evt.Type, EventResult)
	}
	if evt.Result != "Hello world" {
		t.Errorf("Result = %q, want %q", evt.Result, "Hello world")
	}
	if evt.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", evt.SessionID, "session-123")
	}
	if evt.Cost != 0.05 {
		t.Errorf("Cost = %v, want %v", evt.Cost, 0.05)
	}
	if evt.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want %d", evt.NumTurns, 3)
	}
	if evt.IsError {
		t.Error("IsError should be false")
	}
	if evt.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", evt.StopReason, "end_turn")
	}
}

func TestSDKEventResultEventWithError(t *testing.T) {
	evt := ResultEvent("", "session-456", 1.50, 1, true, "budget_exceeded")
	if evt.Type != EventResult {
		t.Errorf("Type = %v, want %v", evt.Type, EventResult)
	}
	if !evt.IsError {
		t.Error("IsError should be true for error result")
	}
	if evt.StopReason != "budget_exceeded" {
		t.Errorf("StopReason = %q, want %q", evt.StopReason, "budget_exceeded")
	}
	if evt.NumTurns != 1 {
		t.Errorf("NumTurns = %d, want %d", evt.NumTurns, 1)
	}
}

func TestSDKEventCompactingEvent(t *testing.T) {
	// Test start of compaction
	evt := CompactingEvent("compacting")
	if evt.Type != EventCompacting {
		t.Errorf("Type = %v, want %v", evt.Type, EventCompacting)
	}
	if evt.Status != "compacting" {
		t.Errorf("Status = %q, want %q", evt.Status, "compacting")
	}

	// Test end of compaction
	evt2 := CompactingEvent("")
	if evt2.Type != EventCompacting {
		t.Errorf("Type = %v, want %v", evt2.Type, EventCompacting)
	}
	if evt2.Status != "" {
		t.Errorf("Status = %q, want %q", evt2.Status, "")
	}
}

func TestSDKEventUserMessageEvent(t *testing.T) {
	evt := UserMessageEvent("What is Go?")
	if evt.Type != EventUserMessage {
		t.Errorf("Type = %v, want %v", evt.Type, EventUserMessage)
	}
	if evt.Text != "What is Go?" {
		t.Errorf("Text = %q, want %q", evt.Text, "What is Go?")
	}
}

func TestSDKEventTypeUniqueness(t *testing.T) {
	// All event type constants must have unique string values
	allTypes := []SDKEventType{
		EventTextDelta,
		EventThinkingDelta,
		EventToolUseStart,
		EventToolUseInput,
		EventToolUseEnd,
		EventToolResult,
		EventTurnComplete,
		EventCostUpdate,
		EventError,
		EventResult,
		EventCompacting,
		EventUserMessage,
		EventSystemMessage,
	}

	seen := make(map[SDKEventType]bool, len(allTypes))
	for _, typ := range allTypes {
		if typ == "" {
			t.Errorf("event type constant should not be empty")
		}
		if seen[typ] {
			t.Errorf("duplicate event type constant: %q", typ)
		}
		seen[typ] = true
	}

	if len(seen) != len(allTypes) {
		t.Errorf("expected %d unique types, got %d", len(allTypes), len(seen))
	}
}

func TestSDKEventNewTypeValues(t *testing.T) {
	// Verify new event type string values match TS SDK names
	tests := []struct {
		got  SDKEventType
		want string
	}{
		{EventResult, "result"},
		{EventCompacting, "compacting"},
		{EventUserMessage, "user_message"},
		{EventSystemMessage, "system"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("event type = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestSDKEventExtendedFields(t *testing.T) {
	// Verify the extended fields on SDKEvent are usable
	evt := SDKEvent{
		Type:       EventResult,
		Result:     "full result text",
		SessionID:  "sess-abc",
		NumTurns:   5,
		Status:     "",
		StopReason: "end_turn",
		Cost:       0.123,
		IsError:    false,
	}
	if evt.Result != "full result text" {
		t.Errorf("Result = %q, want %q", evt.Result, "full result text")
	}
	if evt.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", evt.SessionID, "sess-abc")
	}
	if evt.NumTurns != 5 {
		t.Errorf("NumTurns = %d, want %d", evt.NumTurns, 5)
	}
	if evt.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", evt.StopReason, "end_turn")
	}
}

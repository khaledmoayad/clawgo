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

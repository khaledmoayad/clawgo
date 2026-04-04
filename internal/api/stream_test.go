package api

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

// --- Error categorization tests ---

func TestCategorizeError_429(t *testing.T) {
	err := &anthropic.Error{StatusCode: 429}
	cat := CategorizeError(err)
	if cat != ErrRateLimit {
		t.Errorf("expected %q, got %q", ErrRateLimit, cat)
	}
}

func TestCategorizeError_529(t *testing.T) {
	err := &anthropic.Error{StatusCode: 529}
	cat := CategorizeError(err)
	if cat != ErrOverloaded {
		t.Errorf("expected %q, got %q", ErrOverloaded, cat)
	}
}

func TestCategorizeError_500(t *testing.T) {
	err := &anthropic.Error{StatusCode: 500}
	cat := CategorizeError(err)
	if cat != ErrServerError {
		t.Errorf("expected %q, got %q", ErrServerError, cat)
	}
}

func TestCategorizeError_401(t *testing.T) {
	err := &anthropic.Error{StatusCode: 401}
	cat := CategorizeError(err)
	if cat != ErrAuth {
		t.Errorf("expected %q, got %q", ErrAuth, cat)
	}
}

func TestCategorizeError_403(t *testing.T) {
	err := &anthropic.Error{StatusCode: 403}
	cat := CategorizeError(err)
	if cat != ErrAuth {
		t.Errorf("expected %q, got %q", ErrAuth, cat)
	}
}

func TestCategorizeError_400(t *testing.T) {
	err := &anthropic.Error{StatusCode: 400}
	cat := CategorizeError(err)
	if cat != ErrClientError {
		t.Errorf("expected %q, got %q", ErrClientError, cat)
	}
}

func TestCategorizeError_Nil(t *testing.T) {
	cat := CategorizeError(nil)
	if cat != ErrUnknown {
		t.Errorf("expected %q for nil error, got %q", ErrUnknown, cat)
	}
}

func TestIsRetryable_RateLimit(t *testing.T) {
	err := &anthropic.Error{StatusCode: 429}
	if !IsRetryable(err) {
		t.Error("rate_limit should be retryable")
	}
}

func TestIsRetryable_Overloaded(t *testing.T) {
	err := &anthropic.Error{StatusCode: 529}
	if !IsRetryable(err) {
		t.Error("overloaded should be retryable")
	}
}

func TestIsRetryable_ServerError(t *testing.T) {
	err := &anthropic.Error{StatusCode: 500}
	if !IsRetryable(err) {
		t.Error("server_error should be retryable")
	}
}

func TestIsRetryable_Auth(t *testing.T) {
	err := &anthropic.Error{StatusCode: 401}
	if IsRetryable(err) {
		t.Error("auth should NOT be retryable")
	}
}

func TestIsRetryable_ClientError(t *testing.T) {
	err := &anthropic.Error{StatusCode: 400}
	if IsRetryable(err) {
		t.Error("client_error should NOT be retryable")
	}
}

func TestIsRetryable_NetworkError(t *testing.T) {
	err := &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "api.anthropic.com"}}
	if !IsRetryable(err) {
		t.Error("network error should be retryable")
	}
}

// --- StreamEvent type tests ---

func TestStreamEvent_Types(t *testing.T) {
	// Verify all event types are distinct and correctly defined
	types := []StreamEventType{
		EventText, EventThinking, EventInputJSON,
		EventToolUseStart, EventToolUseEnd,
		EventMessageDelta, EventMessageComplete, EventError,
	}
	seen := make(map[StreamEventType]bool)
	for _, ty := range types {
		if seen[ty] {
			t.Errorf("duplicate event type: %q", ty)
		}
		seen[ty] = true
	}
	if len(types) != 8 {
		t.Errorf("expected 8 event types, got %d", len(types))
	}
}

func TestStreamEvent_TextEvent(t *testing.T) {
	evt := StreamEvent{Type: EventText, Text: "Hello world"}
	if evt.Type != EventText {
		t.Errorf("expected type %q, got %q", EventText, evt.Type)
	}
	if evt.Text != "Hello world" {
		t.Errorf("expected text %q, got %q", "Hello world", evt.Text)
	}
}

func TestStreamEvent_ToolUseEvent(t *testing.T) {
	input := json.RawMessage(`{"command":"ls -la"}`)
	evt := StreamEvent{
		Type: EventToolUseStart,
		ToolUse: &ToolUseBlock{
			ID:    "tool_123",
			Name:  "bash",
			Input: input,
		},
	}
	if evt.ToolUse.ID != "tool_123" {
		t.Errorf("expected tool ID %q, got %q", "tool_123", evt.ToolUse.ID)
	}
	if evt.ToolUse.Name != "bash" {
		t.Errorf("expected tool name %q, got %q", "bash", evt.ToolUse.Name)
	}
}

func TestStreamEvent_ErrorEvent(t *testing.T) {
	evt := StreamEvent{
		Type:  EventError,
		Error: &anthropic.Error{StatusCode: 500},
	}
	if evt.Error == nil {
		t.Error("error event should have non-nil error")
	}
}

// --- Message type tests ---

func TestUserMessage(t *testing.T) {
	msg := UserMessage("hello")
	if msg.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	if msg.Content[0].Type != ContentText {
		t.Errorf("expected type %q, got %q", ContentText, msg.Content[0].Type)
	}
	if msg.Content[0].Text != "hello" {
		t.Errorf("expected text %q, got %q", "hello", msg.Content[0].Text)
	}
}

func TestToolResultMessage(t *testing.T) {
	msg := ToolResultMessage("tool_abc", "output text", false)
	if msg.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	cb := msg.Content[0]
	if cb.Type != ContentToolResult {
		t.Errorf("expected type %q, got %q", ContentToolResult, cb.Type)
	}
	if cb.ToolUseID != "tool_abc" {
		t.Errorf("expected tool_use_id %q, got %q", "tool_abc", cb.ToolUseID)
	}
	if cb.Content != "output text" {
		t.Errorf("expected content %q, got %q", "output text", cb.Content)
	}
	if cb.IsError {
		t.Error("expected is_error to be false")
	}
}

func TestToolResultMessage_Error(t *testing.T) {
	msg := ToolResultMessage("tool_xyz", "error occurred", true)
	if !msg.Content[0].IsError {
		t.Error("expected is_error to be true")
	}
}

func TestAssistantMessage(t *testing.T) {
	msg := AssistantMessage("I can help with that")
	if msg.Role != "assistant" {
		t.Errorf("expected role %q, got %q", "assistant", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	if msg.Content[0].Text != "I can help with that" {
		t.Errorf("unexpected text: %q", msg.Content[0].Text)
	}
}

func TestMessage_ToParam(t *testing.T) {
	msg := UserMessage("test prompt")
	param := msg.ToParam()

	if param.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", param.Role)
	}
	if len(param.Content) != 1 {
		t.Fatalf("expected 1 content block param, got %d", len(param.Content))
	}
}

func TestMessage_ToParam_ToolResult(t *testing.T) {
	msg := ToolResultMessage("tool_123", "result data", false)
	param := msg.ToParam()

	if param.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", param.Role)
	}
	if len(param.Content) != 1 {
		t.Fatalf("expected 1 content block param, got %d", len(param.Content))
	}
}

func TestMessage_JSON_Roundtrip(t *testing.T) {
	msg := UserMessage("round trip test")
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Role != msg.Role {
		t.Errorf("expected role %q, got %q", msg.Role, decoded.Role)
	}
	if len(decoded.Content) != 1 || decoded.Content[0].Text != "round trip test" {
		t.Error("content did not survive round trip")
	}
}

func TestUsage_JSON(t *testing.T) {
	u := Usage{
		InputTokens:              1234,
		OutputTokens:             567,
		CacheCreationInputTokens: 100,
		CacheReadInputTokens:     200,
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Usage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.InputTokens != 1234 {
		t.Errorf("expected input tokens 1234, got %d", decoded.InputTokens)
	}
	if decoded.OutputTokens != 567 {
		t.Errorf("expected output tokens 567, got %d", decoded.OutputTokens)
	}
}

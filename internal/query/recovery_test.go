package query

import (
	"errors"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

func TestRecoveryIsMaxTokensStop(t *testing.T) {
	tests := []struct {
		name       string
		stopReason string
		want       bool
	}{
		{"max_tokens returns true", "max_tokens", true},
		{"end_turn returns false", "end_turn", false},
		{"empty string returns false", "", false},
		{"tool_use returns false", "tool_use", false},
		{"stop_sequence returns false", "stop_sequence", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMaxTokensStop(tt.stopReason)
			if got != tt.want {
				t.Errorf("IsMaxTokensStop(%q) = %v, want %v", tt.stopReason, got, tt.want)
			}
		})
	}
}

func TestRecoveryHandleMaxTokensEscalation(t *testing.T) {
	// Count=0, no override, small max tokens -> escalate to 64000
	state := &MaxTokensRecoveryState{
		RecoveryCount:     0,
		MaxOutputOverride: 0,
	}

	action, newMax := HandleMaxTokensRecovery(state, CappedDefaultMaxTokens)
	if action != RecoveryEscalate {
		t.Errorf("expected escalate action, got %q", action)
	}
	if newMax != EscalatedMaxTokens {
		t.Errorf("expected escalated max %d, got %d", EscalatedMaxTokens, newMax)
	}
}

func TestRecoveryHandleMaxTokensRetry(t *testing.T) {
	// Already escalated (override set), count < limit -> retry
	state := &MaxTokensRecoveryState{
		RecoveryCount:     1,
		MaxOutputOverride: EscalatedMaxTokens,
	}

	action, newMax := HandleMaxTokensRecovery(state, EscalatedMaxTokens)
	if action != RecoveryRetry {
		t.Errorf("expected retry action, got %q", action)
	}
	if newMax != EscalatedMaxTokens {
		t.Errorf("expected same max %d, got %d", EscalatedMaxTokens, newMax)
	}
}

func TestRecoveryHandleMaxTokensRetryAtLimit(t *testing.T) {
	// Count at limit -> stop
	state := &MaxTokensRecoveryState{
		RecoveryCount:     MaxOutputTokensRecoveryLimit,
		MaxOutputOverride: EscalatedMaxTokens,
	}

	action, _ := HandleMaxTokensRecovery(state, EscalatedMaxTokens)
	if action != RecoveryStop {
		t.Errorf("expected stop action, got %q", action)
	}
}

func TestRecoveryHandleMaxTokensStop(t *testing.T) {
	// Already escalated, count == 3 (exhausted) -> stop
	state := &MaxTokensRecoveryState{
		RecoveryCount:     3,
		MaxOutputOverride: EscalatedMaxTokens,
	}

	action, newMax := HandleMaxTokensRecovery(state, EscalatedMaxTokens)
	if action != RecoveryStop {
		t.Errorf("expected stop action, got %q", action)
	}
	if newMax != EscalatedMaxTokens {
		t.Errorf("expected unchanged max %d, got %d", EscalatedMaxTokens, newMax)
	}
}

func TestRecoveryHandleMaxTokensNoEscalateWhenAlreadyHigh(t *testing.T) {
	// Already at or above escalated tokens, count=0, no override -> retry not escalate
	state := &MaxTokensRecoveryState{
		RecoveryCount:     0,
		MaxOutputOverride: 0,
	}

	action, _ := HandleMaxTokensRecovery(state, EscalatedMaxTokens)
	if action != RecoveryRetry {
		t.Errorf("expected retry (not escalate) when already at escalated max, got %q", action)
	}
}

func TestRecoveryIsMediaSizeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"image too large",
			errors.New("The image provided is too large for the model"),
			true,
		},
		{
			"document too large",
			errors.New("The document is too large to process"),
			true,
		},
		{
			"request_too_large error type",
			errors.New("error type: request_too_large"),
			true,
		},
		{
			"case insensitive image",
			errors.New("IMAGE is TOO LARGE"),
			true,
		},
		{
			"unrelated error",
			errors.New("rate limit exceeded"),
			false,
		},
		{
			"nil error",
			nil,
			false,
		},
		{
			"too large without image or document",
			errors.New("the request payload is too large"),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMediaSizeError(tt.err)
			if got != tt.want {
				t.Errorf("IsMediaSizeError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRecoveryHandleMediaSizeErrorStripsImages(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("hello"),
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "Check this image"},
				{
					Type: api.ContentImage,
					Source: &api.ImageSource{
						Type:      "base64",
						MediaType: "image/png",
						Data:      "base64data...",
					},
				},
				{
					Type: api.ContentDocument,
					DocumentSource: &api.DocumentSource{
						Type:      "base64",
						MediaType: "application/pdf",
						Data:      "pdfdata...",
					},
				},
			},
		},
	}

	result := HandleMediaSizeError(messages)

	// Should only have 2 messages
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// Last message should only have the text block
	lastMsg := result[1]
	if len(lastMsg.Content) != 1 {
		t.Fatalf("expected 1 content block in last message, got %d", len(lastMsg.Content))
	}
	if lastMsg.Content[0].Type != api.ContentText {
		t.Errorf("expected text block, got %s", lastMsg.Content[0].Type)
	}
	if lastMsg.Content[0].Text != "Check this image" {
		t.Errorf("expected text 'Check this image', got %q", lastMsg.Content[0].Text)
	}
}

func TestRecoveryHandleMediaSizeErrorNoMediaUnchanged(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("hello"),
		api.UserMessage("world"),
	}

	result := HandleMediaSizeError(messages)

	// Should be identical (same slice reference since no changes needed)
	if len(result) != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), len(result))
	}
	for i := range messages {
		if result[i].Content[0].Text != messages[i].Content[0].Text {
			t.Errorf("message %d content changed unexpectedly", i)
		}
	}
}

func TestRecoveryHandleMediaSizeErrorEmptyMessages(t *testing.T) {
	result := HandleMediaSizeError(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = HandleMediaSizeError([]api.Message{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(result))
	}
}

func TestRecoveryGetContinuationMessage(t *testing.T) {
	msg := GetContinuationMessage()

	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %q", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	if msg.Content[0].Type != api.ContentText {
		t.Errorf("expected text content block, got %s", msg.Content[0].Type)
	}
	if msg.Content[0].Text == "" {
		t.Error("expected non-empty continuation text")
	}
	// Verify it contains the key instruction
	if !containsSubstring(msg.Content[0].Text, "Resume directly") {
		t.Error("continuation message should contain 'Resume directly'")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

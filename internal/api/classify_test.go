package api

import (
	"errors"
	"fmt"
	"testing"
)

func TestParsePromptTooLongTokenCounts(t *testing.T) {
	tests := []struct {
		input      string
		wantActual int
		wantLimit  int
	}{
		{
			input:      "prompt is too long: 137500 tokens > 135000 maximum",
			wantActual: 137500,
			wantLimit:  135000,
		},
		{
			input:      "Prompt is too long: 200000 tokens > 128000 maximum",
			wantActual: 200000,
			wantLimit:  128000,
		},
		{
			input:      "prompt is too long blah 50000 token > 40000",
			wantActual: 50000,
			wantLimit:  40000,
		},
		{
			input:      "some random error",
			wantActual: 0,
			wantLimit:  0,
		},
		{
			input:      "",
			wantActual: 0,
			wantLimit:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual, limit := ParsePromptTooLongTokenCounts(tt.input)
			if actual != tt.wantActual || limit != tt.wantLimit {
				t.Errorf("ParsePromptTooLongTokenCounts(%q) = (%d, %d), want (%d, %d)",
					tt.input, actual, limit, tt.wantActual, tt.wantLimit)
			}
		})
	}
}

func TestGetPromptTooLongTokenGap(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"prompt is too long: 137500 tokens > 135000 maximum", 2500},
		{"prompt is too long: 100000 tokens > 100000 maximum", 0},
		{"some random error", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := GetPromptTooLongTokenGap(tt.input)
			if got != tt.want {
				t.Errorf("GetPromptTooLongTokenGap(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsMediaSizeError(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"image exceeds 5 MB maximum: 5316852 bytes > 5242880 bytes", true},
		{"image dimensions exceed the allowed many-image limit", true},
		{"maximum of 100 PDF pages allowed", true},
		{"prompt is too long", false},
		{"rate limit exceeded", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsMediaSizeError(tt.input)
			if got != tt.want {
				t.Errorf("IsMediaSizeError(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsPromptTooLongError(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"prompt is too long: 137500 tokens > 135000 maximum", true},
		{"Prompt is too long: 200000 tokens > 128000 maximum", true},
		{"rate limit exceeded", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsPromptTooLongError(tt.input)
			if got != tt.want {
				t.Errorf("IsPromptTooLongError(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestClassifyAPIError_Timeout(t *testing.T) {
	err := fmt.Errorf("connection timeout after 30s")
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeAPITimeout {
		t.Errorf("expected ErrTypeAPITimeout, got %s", info.Type)
	}
	if !info.IsRecoverable {
		t.Error("timeout should be recoverable")
	}
}

func TestClassifyAPIError_RateLimit(t *testing.T) {
	err := &HTTPError{StatusCode: 429, Body: "Too many requests"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeRateLimit {
		t.Errorf("expected ErrTypeRateLimit, got %s", info.Type)
	}
	if info.Category != ErrRateLimit {
		t.Errorf("expected ErrRateLimit category, got %s", info.Category)
	}
	if !info.IsRecoverable {
		t.Error("rate limit should be recoverable")
	}
}

func TestClassifyAPIError_PromptTooLong(t *testing.T) {
	err := fmt.Errorf("prompt is too long: 137500 tokens > 135000 maximum")
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypePromptTooLong {
		t.Errorf("expected ErrTypePromptTooLong, got %s", info.Type)
	}
	if !info.IsRecoverable {
		t.Error("prompt too long should be recoverable via compaction")
	}
}

func TestClassifyAPIError_ServerOverload(t *testing.T) {
	err := &HTTPError{StatusCode: 529, Body: "Server overloaded"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeServerOverload {
		t.Errorf("expected ErrTypeServerOverload, got %s", info.Type)
	}
	if !info.IsRecoverable {
		t.Error("server overload should be recoverable")
	}
}

func TestClassifyAPIError_InvalidModel(t *testing.T) {
	err := &HTTPError{StatusCode: 400, Body: "Invalid model name: claude-nonexistent"}
	info := ClassifyAPIError(err, "claude-nonexistent", false)
	if info.Type != ErrTypeInvalidModel {
		t.Errorf("expected ErrTypeInvalidModel, got %s", info.Type)
	}
	if info.IsRecoverable {
		t.Error("invalid model should not be recoverable")
	}
}

func TestClassifyAPIError_AuthError(t *testing.T) {
	err := &HTTPError{StatusCode: 401, Body: "Unauthorized"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeAuthError {
		t.Errorf("expected ErrTypeAuthError, got %s", info.Type)
	}
	if info.Category != ErrAuth {
		t.Errorf("expected ErrAuth category, got %s", info.Category)
	}
}

func TestClassifyAPIError_TokenRevoked(t *testing.T) {
	err := &HTTPError{StatusCode: 403, Body: "OAuth token has been revoked"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeTokenRevoked {
		t.Errorf("expected ErrTypeTokenRevoked, got %s", info.Type)
	}
}

func TestClassifyAPIError_CreditBalance(t *testing.T) {
	err := fmt.Errorf("Your credit balance is too low to make this request")
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeCreditBalanceLow {
		t.Errorf("expected ErrTypeCreditBalanceLow, got %s", info.Type)
	}
}

func TestClassifyAPIError_ImageTooLarge(t *testing.T) {
	err := &HTTPError{StatusCode: 400, Body: "image exceeds 5 MB maximum: 5316852 bytes > 5242880 bytes"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeImageTooLarge {
		t.Errorf("expected ErrTypeImageTooLarge, got %s", info.Type)
	}
}

func TestClassifyAPIError_ToolUseMismatch(t *testing.T) {
	err := &HTTPError{StatusCode: 400, Message: "`tool_use` ids were found without `tool_result` blocks immediately after"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeToolUseMismatch {
		t.Errorf("expected ErrTypeToolUseMismatch, got %s", info.Type)
	}
}

func TestClassifyAPIError_PDFTooLarge(t *testing.T) {
	err := fmt.Errorf("maximum of 100 PDF pages exceeded")
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", true)
	if info.Type != ErrTypePDFTooLarge {
		t.Errorf("expected ErrTypePDFTooLarge, got %s", info.Type)
	}
	// Non-interactive should not mention esc key
	if info.UserMessage == "" {
		t.Error("expected non-empty user message")
	}
}

func TestClassifyAPIError_NonInteractiveMessages(t *testing.T) {
	// Auth errors should differ between interactive and non-interactive
	err := &HTTPError{StatusCode: 401, Message: "Unauthorized"}

	interactive := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	nonInteractive := ClassifyAPIError(err, "claude-sonnet-4-20250514", true)

	if interactive.UserMessage == nonInteractive.UserMessage {
		t.Error("interactive and non-interactive messages should differ for auth errors")
	}
}

func TestClassifyAPIError_ServerError(t *testing.T) {
	err := &HTTPError{StatusCode: 500, Message: "Internal server error"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeServerError {
		t.Errorf("expected ErrTypeServerError, got %s", info.Type)
	}
	if !info.IsRecoverable {
		t.Error("server error should be recoverable")
	}
}

func TestClassifyAPIError_Nil(t *testing.T) {
	info := ClassifyAPIError(nil, "claude-sonnet-4-20250514", false)
	if info != nil {
		t.Error("nil error should return nil info")
	}
}

func TestClassifyAPIError_GenericError(t *testing.T) {
	err := errors.New("some unknown error")
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeUnknown {
		t.Errorf("expected ErrTypeUnknown, got %s", info.Type)
	}
}

func TestGetRefusalMessage(t *testing.T) {
	info := GetRefusalMessage("refusal", "claude-sonnet-4-20250514", false)
	if info == nil {
		t.Fatal("expected non-nil info for refusal stop reason")
	}
	if info.Type != ErrTypeRefusal {
		t.Errorf("expected ErrTypeRefusal, got %s", info.Type)
	}

	// Non-refusal should return nil
	info = GetRefusalMessage("end_turn", "claude-sonnet-4-20250514", false)
	if info != nil {
		t.Error("expected nil for non-refusal stop reason")
	}
}

func TestClassifyAPIError_RequestTooLarge(t *testing.T) {
	err := &HTTPError{StatusCode: 413, Message: "Request entity too large"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Category != ErrClientError {
		t.Errorf("expected ErrClientError, got %s", info.Category)
	}
}

func TestClassifyAPIError_ExtraUsageRequired(t *testing.T) {
	err := &HTTPError{StatusCode: 429, Message: "Extra usage is required for long context"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeRateLimit {
		t.Errorf("expected ErrTypeRateLimit, got %s", info.Type)
	}
	if !containsStr(info.UserMessage, "Extra usage") {
		t.Errorf("expected Extra usage in message, got %s", info.UserMessage)
	}
}

func TestClassifyAPIError_DuplicateToolUseID(t *testing.T) {
	err := &HTTPError{StatusCode: 400, Message: "`tool_use` ids must be unique across all messages"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeDuplicateToolUseID {
		t.Errorf("expected ErrTypeDuplicateToolUseID, got %s", info.Type)
	}
}

func TestClassifyAPIError_OrgNotAllowed(t *testing.T) {
	err := &HTTPError{StatusCode: 403, Message: "OAuth authentication is currently not allowed for this organization"}
	info := ClassifyAPIError(err, "claude-sonnet-4-20250514", false)
	if info.Type != ErrTypeOAuthOrgNotAllowed {
		t.Errorf("expected ErrTypeOAuthOrgNotAllowed, got %s", info.Type)
	}
}

func TestClassifyAPIError_ModelNotFound(t *testing.T) {
	err := &HTTPError{StatusCode: 404, Message: "Not found"}
	info := ClassifyAPIError(err, "claude-nonexistent", false)
	if info.Type != ErrTypeInvalidModel {
		t.Errorf("expected ErrTypeInvalidModel, got %s", info.Type)
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

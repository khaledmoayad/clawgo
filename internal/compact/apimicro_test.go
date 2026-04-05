package compact

import (
	"strings"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// helper to create a long string exceeding microCompactMinLength
func longContent() string {
	return strings.Repeat("x", microCompactMinLength+100)
}

func TestAPIMicroCompact_ClearsPreCacheBoundary(t *testing.T) {
	state := NewCachedMicroCompactState()

	messages := []api.Message{
		// Message 0: assistant with tool_use from compactable tool
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tu-1", Name: "Bash"},
			},
		},
		// Message 1: user with tool_result (long content)
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: longContent()},
			},
		},
		// Message 2: assistant with another tool_use (post-cache)
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tu-2", Name: "Read"},
			},
		},
		// Message 3: user with tool_result (post-cache)
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-2", Content: longContent()},
			},
		},
	}

	// Cache breakpoint at index 2: only messages 0,1 are in cached region
	result := APIMicroCompact(messages, state, 2)

	// Message 1's tool result should be cleared
	if result[1].Content[0].Content != "[Content cleared to save context]" {
		t.Errorf("expected cleared content for pre-cache message, got %q", result[1].Content[0].Content)
	}

	// Message 3's tool result should be preserved (post-cache)
	if result[3].Content[0].Content == "[Content cleared to save context]" {
		t.Error("post-cache tool result should not be cleared")
	}

	// Verify the cleared ID was tracked
	if !state.ClearedIDs["tu-1"] {
		t.Error("expected tu-1 in ClearedIDs")
	}
}

func TestAPIMicroCompact_SkipsAlreadyClearedIDs(t *testing.T) {
	state := NewCachedMicroCompactState()
	state.ClearedIDs["tu-1"] = true // Pre-mark as cleared

	messages := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tu-1", Name: "Bash"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: longContent()},
			},
		},
	}

	result := APIMicroCompact(messages, state, 2)

	// Should NOT be cleared because it's already in ClearedIDs
	if result[1].Content[0].Content == "[Content cleared to save context]" {
		t.Error("should not re-clear already-cleared tool result")
	}
}

func TestAPIMicroCompact_PreservesPostCacheMessages(t *testing.T) {
	state := NewCachedMicroCompactState()

	messages := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tu-1", Name: "Grep"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: longContent()},
			},
		},
	}

	// Breakpoint at 0: nothing is in cached region
	result := APIMicroCompact(messages, state, 0)

	// Nothing should be cleared
	if result[1].Content[0].Content == "[Content cleared to save context]" {
		t.Error("should not clear when breakpoint is 0")
	}
}

func TestAPIMicroCompact_SkipsShortContent(t *testing.T) {
	state := NewCachedMicroCompactState()

	messages := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tu-1", Name: "Bash"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: "short"},
			},
		},
	}

	result := APIMicroCompact(messages, state, 2)

	// Short content should not be cleared
	if result[1].Content[0].Content != "short" {
		t.Errorf("short content should be preserved, got %q", result[1].Content[0].Content)
	}
}

func TestAPIMicroCompact_SkipsNonCompactableTools(t *testing.T) {
	state := NewCachedMicroCompactState()

	messages := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				// "CustomTool" is not in CompactableTools
				{Type: api.ContentToolUse, ID: "tu-1", Name: "CustomTool"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: longContent()},
			},
		},
	}

	result := APIMicroCompact(messages, state, 2)

	// Non-compactable tool results should be preserved
	if result[1].Content[0].Content == "[Content cleared to save context]" {
		t.Error("non-compactable tool result should not be cleared")
	}
}

func TestAPIMicroCompact_EmptyMessages(t *testing.T) {
	state := NewCachedMicroCompactState()
	result := APIMicroCompact(nil, state, 5)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	result = APIMicroCompact([]api.Message{}, state, 5)
	if len(result) != 0 {
		t.Error("expected empty for empty input")
	}
}

func TestUpdateCacheBreakpoint(t *testing.T) {
	state := NewCachedMicroCompactState()
	if state.LastCacheBreakpoint != 0 {
		t.Errorf("expected initial breakpoint=0, got %d", state.LastCacheBreakpoint)
	}

	UpdateCacheBreakpoint(state, 15)
	if state.LastCacheBreakpoint != 15 {
		t.Errorf("expected breakpoint=15, got %d", state.LastCacheBreakpoint)
	}
}

func TestAPIMicroCompact_DoesNotMutateOriginal(t *testing.T) {
	state := NewCachedMicroCompactState()
	original := longContent()

	messages := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tu-1", Name: "Read"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: original},
			},
		},
	}

	result := APIMicroCompact(messages, state, 2)

	// Result should be cleared
	if result[1].Content[0].Content != "[Content cleared to save context]" {
		t.Error("expected result to be cleared")
	}

	// Original should be unchanged
	if messages[1].Content[0].Content != original {
		t.Error("original message should not be mutated")
	}
}

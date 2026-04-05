package query

import (
	"encoding/json"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

func TestTombstoneYieldMissingToolResultBlocks(t *testing.T) {
	assistantMsgs := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "Let me help you."},
				{Type: api.ContentToolUse, ID: "tool_1", Name: "BashTool", Input: json.RawMessage(`{"command":"ls"}`)},
				{Type: api.ContentToolUse, ID: "tool_2", Name: "ReadTool", Input: json.RawMessage(`{"path":"foo.txt"}`)},
			},
		},
	}

	results := YieldMissingToolResultBlocks(assistantMsgs, "Interrupted: streaming error")

	if len(results) != 2 {
		t.Fatalf("expected 2 tool result entries, got %d", len(results))
	}

	// Check first result
	if results[0].ToolUseID != "tool_1" {
		t.Errorf("expected tool_use_id 'tool_1', got %q", results[0].ToolUseID)
	}
	if results[0].Content != "Interrupted: streaming error" {
		t.Errorf("unexpected content: %q", results[0].Content)
	}
	if !results[0].IsError {
		t.Error("expected is_error=true")
	}

	// Check second result
	if results[1].ToolUseID != "tool_2" {
		t.Errorf("expected tool_use_id 'tool_2', got %q", results[1].ToolUseID)
	}
	if !results[1].IsError {
		t.Error("expected is_error=true for second result")
	}
}

func TestTombstoneYieldMissingToolResultBlocksNoToolUse(t *testing.T) {
	// Assistant message with no tool_use blocks
	assistantMsgs := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "Just a text response"},
			},
		},
	}

	results := YieldMissingToolResultBlocks(assistantMsgs, "error")
	if len(results) != 0 {
		t.Errorf("expected 0 results for no tool_use blocks, got %d", len(results))
	}
}

func TestTombstoneYieldMissingSkipsUserMessages(t *testing.T) {
	// Mix of user and assistant messages - should only process assistant
	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tool_x", Content: "result"},
			},
		},
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tool_1", Name: "BashTool"},
			},
		},
	}

	results := YieldMissingToolResultBlocks(messages, "error")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only from assistant), got %d", len(results))
	}
	if results[0].ToolUseID != "tool_1" {
		t.Errorf("expected tool_use_id 'tool_1', got %q", results[0].ToolUseID)
	}
}

func TestTombstoneOrphanedMessagesCreatesResults(t *testing.T) {
	messages := []api.Message{
		// Pre-fallback: matched pair
		api.UserMessage("hello"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "old_tool", Name: "BashTool"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "old_tool", Content: "matched result"},
			},
		},
		// Post-fallback (fromIndex=3): orphaned tool_use
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "I'll run two tools"},
				{Type: api.ContentToolUse, ID: "orphan_1", Name: "GrepTool"},
				{Type: api.ContentToolUse, ID: "orphan_2", Name: "ReadTool"},
			},
		},
	}

	tombstoned, tombstones := TombstoneOrphanedMessages(messages, 3)

	// Should have the original 4 messages + 1 synthetic user message for the orphaned assistant
	if len(tombstoned) != 5 {
		t.Fatalf("expected 5 messages (4 original + 1 synthetic), got %d", len(tombstoned))
	}

	// The synthetic message should be right after the orphaned assistant message
	syntheticMsg := tombstoned[4]
	if syntheticMsg.Role != "user" {
		t.Errorf("expected synthetic message role 'user', got %q", syntheticMsg.Role)
	}
	if len(syntheticMsg.Content) != 2 {
		t.Fatalf("expected 2 synthetic tool results, got %d", len(syntheticMsg.Content))
	}

	// Check synthetic results
	for _, block := range syntheticMsg.Content {
		if block.Type != api.ContentToolResult {
			t.Errorf("expected tool_result type, got %s", block.Type)
		}
		if !block.IsError {
			t.Error("expected is_error=true for synthetic result")
		}
		if block.Content != tombstoneErrorContent {
			t.Errorf("expected tombstone error content, got %q", block.Content)
		}
		if block.ToolUseID != "orphan_1" && block.ToolUseID != "orphan_2" {
			t.Errorf("unexpected tool_use_id: %q", block.ToolUseID)
		}
	}

	// Check tombstone records
	if len(tombstones) != 1 {
		t.Fatalf("expected 1 tombstone record, got %d", len(tombstones))
	}
	if tombstones[0].Reason != "fallback" {
		t.Errorf("expected reason 'fallback', got %q", tombstones[0].Reason)
	}
}

func TestTombstoneOrphanedMessagesPreservesMatched(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("hello"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentToolUse, ID: "tool_1", Name: "BashTool"},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tool_1", Content: "result"},
			},
		},
	}

	tombstoned, tombstones := TombstoneOrphanedMessages(messages, 0)

	// All tool_use blocks already have matching results, so no synthetic messages
	if len(tombstoned) != 3 {
		t.Fatalf("expected 3 messages (unchanged), got %d", len(tombstoned))
	}
	if len(tombstones) != 0 {
		t.Errorf("expected 0 tombstones for fully matched messages, got %d", len(tombstones))
	}
}

func TestTombstoneOrphanedMessagesInvalidIndex(t *testing.T) {
	messages := []api.Message{api.UserMessage("hello")}

	// fromIndex out of range
	result, tombstones := TombstoneOrphanedMessages(messages, 5)
	if len(result) != 1 {
		t.Errorf("expected original messages returned for invalid index, got %d", len(result))
	}
	if len(tombstones) != 0 {
		t.Errorf("expected 0 tombstones for invalid index, got %d", len(tombstones))
	}

	// Negative index
	result, tombstones = TombstoneOrphanedMessages(messages, -1)
	if len(result) != 1 {
		t.Errorf("expected original messages returned for negative index, got %d", len(result))
	}
	if len(tombstones) != 0 {
		t.Errorf("expected 0 tombstones for negative index, got %d", len(tombstones))
	}
}

func TestTombstoneStripSignatureBlocks(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("hello"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentThinking, Thinking: "Let me think about this..."},
				{Type: api.ContentText, Text: "Here is my answer"},
				{Type: api.ContentRedactedThinking},
			},
		},
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "Another response"},
			},
		},
	}

	result := StripSignatureBlocks(messages)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// User message should be unchanged
	if result[0].Content[0].Text != "hello" {
		t.Errorf("user message should be unchanged")
	}

	// First assistant message should only have text block (thinking + redacted stripped)
	if len(result[1].Content) != 1 {
		t.Fatalf("expected 1 content block after stripping, got %d", len(result[1].Content))
	}
	if result[1].Content[0].Type != api.ContentText {
		t.Errorf("expected text block, got %s", result[1].Content[0].Type)
	}
	if result[1].Content[0].Text != "Here is my answer" {
		t.Errorf("expected 'Here is my answer', got %q", result[1].Content[0].Text)
	}

	// Second assistant message should be unchanged (no thinking blocks)
	if len(result[2].Content) != 1 {
		t.Fatalf("expected 1 content block in second assistant msg, got %d", len(result[2].Content))
	}
}

func TestTombstoneStripSignatureBlocksNoChange(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("hello"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "response"},
			},
		},
	}

	result := StripSignatureBlocks(messages)

	// Should return the same slice reference (no changes)
	if &result[0] != &messages[0] {
		t.Error("expected same slice when no changes needed")
	}
}

func TestTombstoneStripSignatureBlocksEmpty(t *testing.T) {
	result := StripSignatureBlocks(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

package query

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// --- Helper constructors ---

func toolUseAssistant(id, name, inputJSON string) api.Message {
	return api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentToolUse, ID: id, Name: name, Input: json.RawMessage(inputJSON)},
		},
	}
}

func toolResultUser(toolUseID, content string, isError bool) api.Message {
	return api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: api.ContentToolResult, ToolUseID: toolUseID, Content: content, IsError: isError},
		},
	}
}

func assistantWithID(text, messageID string) api.Message {
	return api.Message{
		Role:      "assistant",
		MessageID: messageID,
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
		},
	}
}

func toolUseAssistantWithID(id, name, inputJSON, messageID string) api.Message {
	return api.Message{
		Role:      "assistant",
		MessageID: messageID,
		Content: []api.ContentBlock{
			{Type: api.ContentToolUse, ID: id, Name: name, Input: json.RawMessage(inputJSON)},
		},
	}
}

func userWithToolResultAndText(toolUseID, resultContent, text string) api.Message {
	return api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
			{Type: api.ContentToolResult, ToolUseID: toolUseID, Content: resultContent},
		},
	}
}

func userWithImage(text string, base64Data string) api.Message {
	return api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
			{Type: api.ContentImage, Source: &api.ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      base64Data,
			}},
		},
	}
}

func userWithDocument(text string) api.Message {
	return api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
			{Type: api.ContentDocument, DocumentSource: &api.DocumentSource{
				Type:      "base64",
				MediaType: "application/pdf",
				Data:      "AAAA",
			}},
		},
	}
}

// --- NormalizeMessagesForAPI full pipeline tests ---

func TestNormalizeEmptyInput(t *testing.T) {
	result := NormalizeMessagesForAPI(nil, nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	result = NormalizeMessagesForAPI([]api.Message{}, nil)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(result))
	}
}

func TestNormalizeSingleUserMessage(t *testing.T) {
	msgs := []api.Message{userMsg("hello")}
	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected user role, got %s", result[0].Role)
	}
	if result[0].Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", result[0].Content[0].Text)
	}
}

func TestNormalizeConsecutiveUserMessagesMerged(t *testing.T) {
	msgs := []api.Message{
		userMsg("first"),
		userMsg("second"),
		userMsg("third"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected user role")
	}
	if len(result[0].Content) != 3 {
		t.Errorf("expected 3 content blocks in merged message, got %d", len(result[0].Content))
	}
	if result[0].Content[0].Text != "first" {
		t.Errorf("expected 'first', got %q", result[0].Content[0].Text)
	}
	if result[0].Content[1].Text != "second" {
		t.Errorf("expected 'second', got %q", result[0].Content[1].Text)
	}
	if result[0].Content[2].Text != "third" {
		t.Errorf("expected 'third', got %q", result[0].Content[2].Text)
	}
}

func TestNormalizeConsecutiveAssistantsSameIDMerged(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantWithID("part 1", "msg-123"),
		assistantWithID("part 2", "msg-123"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages (user + merged assistant), got %d", len(result))
	}
	if result[1].Role != "assistant" {
		t.Error("expected assistant role")
	}
	if len(result[1].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged assistant, got %d", len(result[1].Content))
	}
	if result[1].Content[0].Text != "part 1" {
		t.Errorf("expected 'part 1', got %q", result[1].Content[0].Text)
	}
	if result[1].Content[1].Text != "part 2" {
		t.Errorf("expected 'part 2', got %q", result[1].Content[1].Text)
	}
}

func TestNormalizeConsecutiveAssistantsDifferentIDNotMerged(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantWithID("from A", "msg-A"),
		assistantWithID("from B", "msg-B"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages (different IDs not merged), got %d", len(result))
	}
}

func TestNormalizeToolResultsHoistedBeforeText(t *testing.T) {
	// A user message with text followed by tool_result should be reordered
	msgs := []api.Message{
		userWithToolResultAndText("tool-1", "result data", "some text"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if len(result[0].Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result[0].Content))
	}
	// tool_result should come first
	if result[0].Content[0].Type != api.ContentToolResult {
		t.Errorf("expected tool_result first, got %s", result[0].Content[0].Type)
	}
	// text should come second
	if result[0].Content[1].Type != api.ContentText {
		t.Errorf("expected text second, got %s", result[0].Content[1].Type)
	}
}

func TestNormalizeToolResultsHoistedInMergedMessages(t *testing.T) {
	// When two user messages are merged, tool results from the second
	// should still be hoisted to the front
	msgs := []api.Message{
		userMsg("first message"),
		toolResultUser("tool-1", "result", false),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(result))
	}
	// tool_result should be hoisted to front
	if result[0].Content[0].Type != api.ContentToolResult {
		t.Errorf("expected tool_result first after hoisting, got %s", result[0].Content[0].Type)
	}
}

func TestNormalizeProgressAndSystemMessagesFiltered(t *testing.T) {
	// Messages with non-user/non-assistant roles should be filtered out
	msgs := []api.Message{
		userMsg("hello"),
		{Role: "system", Content: []api.ContentBlock{{Type: api.ContentText, Text: "system msg"}}},
		assistantMsg("response"),
		{Role: "progress", Content: []api.ContentBlock{{Type: api.ContentText, Text: "progress"}}},
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages (system+progress filtered), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected first message to be user")
	}
	if result[1].Role != "assistant" {
		t.Error("expected second message to be assistant")
	}
}

func TestNormalizeEmptyContentMessagesFiltered(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		{Role: "user", Content: []api.ContentBlock{}},
		assistantMsg("response"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages (empty filtered), got %d", len(result))
	}
}

func TestNormalizeThinkingOnlyAssistantRemoved(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		thinkingOnlyAssistant("deep thoughts"),
		userMsg("follow up"),
		assistantMsg("real response"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// Thinking-only assistant removed, adjacent users merged
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected user")
	}
	// Should have merged "hello" and "follow up"
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged user, got %d", len(result[0].Content))
	}
	if result[1].Role != "assistant" {
		t.Error("expected assistant")
	}
}

func TestNormalizeTrailingThinkingStripped(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantWithTrailingThinking("response", "trailing thought"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	last := result[1]
	if len(last.Content) != 1 {
		t.Fatalf("expected 1 content block (thinking stripped), got %d", len(last.Content))
	}
	if last.Content[0].Type != api.ContentText {
		t.Errorf("expected text block, got %s", last.Content[0].Type)
	}
	if last.Content[0].Text != "response" {
		t.Errorf("expected 'response', got %q", last.Content[0].Text)
	}
}

func TestNormalizeWhitespaceAssistantsRemovedAndUsersMerged(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		whitespaceAssistant(),
		userMsg("world"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// Whitespace assistant removed, users merged
	if len(result) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected user")
	}
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(result[0].Content))
	}
}

func TestNormalizeEmptyAssistantAfterStrippingGetsPlaceholder(t *testing.T) {
	// Test EnsureNonEmptyAssistantContent directly since filterNonAPIMessages
	// removes empty-content messages before step 11 can run.
	// Use a message that has content but becomes empty-like after thinking strip.
	// A thinking-only assistant that is NOT last (so FilterOrphanedThinkingMessages
	// removes it) won't test this path. Instead, test the helper directly.
	msgs := []api.Message{
		userMsg("hello"),
		emptyContentAssistant(), // non-final
		userMsg("follow up"),
		assistantMsg("final"),
	}

	result := EnsureNonEmptyAssistantContent(msgs)

	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	if len(result[1].Content) != 1 {
		t.Fatalf("expected 1 placeholder block, got %d", len(result[1].Content))
	}
	if result[1].Content[0].Text != "[No content]" {
		t.Errorf("expected '[No content]', got %q", result[1].Content[0].Text)
	}

	// Also verify that the full pipeline filters empty-content messages in step 1
	result2 := NormalizeMessagesForAPI(msgs, nil)
	// Empty content assistant is filtered in step 1, users merge
	if len(result2) != 2 {
		t.Fatalf("full pipeline: expected 2 messages (empty filtered, users merged), got %d", len(result2))
	}
}

func TestNormalizeErrorToolResultContentSanitized(t *testing.T) {
	// In ClawGo, tool_result content is a string, so sanitization is a
	// pass-through. This test verifies the step doesn't corrupt data.
	msgs := []api.Message{
		userMsg("hello"),
		toolUseAssistant("tool-1", "bash", `{"command":"ls"}`),
		toolResultUser("tool-1", "error occurred", true),
		assistantMsg("I see the error"),
	}

	result := NormalizeMessagesForAPI(msgs, []string{"bash"})

	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// Tool result content should be preserved
	var found bool
	for _, cb := range result[2].Content {
		if cb.Type == api.ContentToolResult {
			found = true
			if cb.Content != "error occurred" {
				t.Errorf("expected 'error occurred', got %q", cb.Content)
			}
		}
	}
	if !found {
		t.Error("expected to find tool_result block")
	}
}

func TestNormalizeFullRealisticConversation(t *testing.T) {
	// Realistic conversation: user, assistant with tool_use, user with tool_result,
	// assistant with text+thinking
	msgs := []api.Message{
		userMsg("Read the file main.go"),
		{
			Role:      "assistant",
			MessageID: "msg-001",
			Content: []api.ContentBlock{
				{Type: api.ContentThinking, Thinking: "Let me read the file"},
				{Type: api.ContentText, Text: "I'll read main.go for you."},
				{Type: api.ContentToolUse, ID: "tu-1", Name: "Read", Input: json.RawMessage(`{"file_path":"/main.go"}`)},
			},
		},
		toolResultUser("tu-1", "package main\n\nfunc main() {}", false),
		{
			Role:      "assistant",
			MessageID: "msg-002",
			Content: []api.ContentBlock{
				{Type: api.ContentThinking, Thinking: "The file contains a simple main function"},
				{Type: api.ContentText, Text: "Here's the content of main.go..."},
			},
		},
	}

	result := NormalizeMessagesForAPI(msgs, []string{"Read"})

	// Should preserve the conversation structure
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// First: user message
	if result[0].Role != "user" {
		t.Error("expected user message first")
	}

	// Second: assistant with thinking + text + tool_use
	if result[1].Role != "assistant" {
		t.Error("expected assistant second")
	}
	hasThinking := false
	hasText := false
	hasToolUse := false
	for _, cb := range result[1].Content {
		switch cb.Type {
		case api.ContentThinking:
			hasThinking = true
		case api.ContentText:
			hasText = true
		case api.ContentToolUse:
			hasToolUse = true
		}
	}
	if !hasThinking {
		t.Error("expected thinking block in assistant")
	}
	if !hasText {
		t.Error("expected text block in assistant")
	}
	if !hasToolUse {
		t.Error("expected tool_use block in assistant")
	}

	// Third: user with tool_result
	if result[2].Role != "user" {
		t.Error("expected user message with tool result")
	}
	if result[2].Content[0].Type != api.ContentToolResult {
		t.Error("expected tool_result block")
	}

	// Fourth: assistant with thinking + text (trailing thinking stripped since it's last)
	if result[3].Role != "assistant" {
		t.Error("expected final assistant")
	}
	// Trailing thinking should be stripped from last assistant, BUT thinking
	// comes before text in this message, so it's NOT trailing.
	// The content should have thinking + text preserved.
	if len(result[3].Content) != 2 {
		t.Errorf("expected 2 blocks (thinking + text), got %d", len(result[3].Content))
	}
}

func TestNormalizeAssistantWithSameIDMergedAcrossToolResults(t *testing.T) {
	// In concurrent agent scenarios, assistant messages with same ID can be
	// interleaved with tool results. They should still be merged.
	msgs := []api.Message{
		userMsg("hello"),
		toolUseAssistantWithID("tu-1", "bash", `{"command":"ls"}`, "msg-123"),
		toolResultUser("tu-1", "file1.go\nfile2.go", false),
		assistantWithID("continuing response", "msg-123"),
	}

	result := NormalizeMessagesForAPI(msgs, []string{"bash"})

	// The two assistant messages with msg-123 should be merged
	assistantCount := 0
	for _, m := range result {
		if m.Role == "assistant" {
			assistantCount++
		}
	}
	if assistantCount != 1 {
		t.Errorf("expected 1 merged assistant, got %d", assistantCount)
	}

	// Find the merged assistant and verify it has both tool_use and text
	for _, m := range result {
		if m.Role == "assistant" {
			hasToolUse := false
			hasText := false
			for _, cb := range m.Content {
				if cb.Type == api.ContentToolUse {
					hasToolUse = true
				}
				if cb.Type == api.ContentText {
					hasText = true
				}
			}
			if !hasToolUse {
				t.Error("merged assistant should have tool_use block")
			}
			if !hasText {
				t.Error("merged assistant should have text block")
			}
		}
	}
}

func TestNormalizeStripMediaFromErrorPreceded(t *testing.T) {
	// When a tool result indicates a document is too large, the immediately
	// preceding user message's document blocks should be stripped.
	// The backward walk only finds user messages (stops at assistant).
	msgs := []api.Message{
		userWithDocument("check this pdf"),
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tu-1",
					Content:   "document too large to process",
					IsError:   true,
				},
			},
		},
		assistantMsg("sorry about that"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// After step 1, both user messages survive. Step 2 strips the document
	// from the first user (directly preceding the error). Step 3 merges
	// the two adjacent users. The merged user should not have a document block.
	firstUser := result[0]
	for _, cb := range firstUser.Content {
		if cb.Type == api.ContentDocument {
			t.Error("document block should have been stripped from error-preceded message")
		}
	}
}

func TestNormalizeValidateImagesDoesNotError(t *testing.T) {
	// Images over the size limit should log a warning but not error
	// Generate a large base64 string (over 5MB)
	largeData := strings.Repeat("A", 6*1024*1024) // 6MB
	msgs := []api.Message{
		userWithImage("check this image", largeData),
		assistantMsg("I see the image"),
	}

	// This should not panic or error
	result := NormalizeMessagesForAPI(msgs, nil)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
}

func TestNormalizeImageUnderLimitPasses(t *testing.T) {
	smallData := strings.Repeat("A", 1024) // 1KB
	msgs := []api.Message{
		userWithImage("small image", smallData),
	}

	result := NormalizeMessagesForAPI(msgs, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	// Image should be preserved
	hasImage := false
	for _, cb := range result[0].Content {
		if cb.Type == api.ContentImage {
			hasImage = true
		}
	}
	if !hasImage {
		t.Error("expected image block to be preserved")
	}
}

// --- Individual step tests ---

func TestNormalizeFilterNonAPIMessages(t *testing.T) {
	msgs := []api.Message{
		userMsg("valid user"),
		{Role: "system", Content: []api.ContentBlock{{Type: api.ContentText, Text: "sys"}}},
		assistantMsg("valid assistant"),
		{Role: "progress", Content: []api.ContentBlock{{Type: api.ContentText, Text: "prog"}}},
		{Role: "user", Content: []api.ContentBlock{}}, // empty content
	}

	result := filterNonAPIMessages(msgs)

	if len(result) != 2 {
		t.Fatalf("expected 2 valid messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected user")
	}
	if result[1].Role != "assistant" {
		t.Error("expected assistant")
	}
}

func TestNormalizeMergeConsecutiveUsers(t *testing.T) {
	msgs := []api.Message{
		userMsg("a"),
		userMsg("b"),
		assistantMsg("c"),
		userMsg("d"),
		userMsg("e"),
	}

	result := mergeConsecutiveUsers(msgs)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	// First merged user: a + b
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 blocks in first merged user, got %d", len(result[0].Content))
	}
	// Assistant
	if result[1].Role != "assistant" {
		t.Error("expected assistant")
	}
	// Second merged user: d + e
	if len(result[2].Content) != 2 {
		t.Errorf("expected 2 blocks in second merged user, got %d", len(result[2].Content))
	}
}

func TestNormalizeMergeConsecutiveUsersToolResultsHoisted(t *testing.T) {
	msgs := []api.Message{
		userMsg("some text"),
		toolResultUser("tu-1", "result data", false),
	}

	result := mergeConsecutiveUsers(msgs)

	if len(result) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(result))
	}
	// Tool result should be hoisted to front
	if result[0].Content[0].Type != api.ContentToolResult {
		t.Errorf("expected tool_result first, got %s", result[0].Content[0].Type)
	}
	if result[0].Content[1].Type != api.ContentText {
		t.Errorf("expected text second, got %s", result[0].Content[1].Type)
	}
}

func TestNormalizeMergeConsecutiveAssistants(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantWithID("part1", "id-1"),
		assistantWithID("part2", "id-1"),
		assistantWithID("different", "id-2"),
	}

	result := mergeConsecutiveAssistants(msgs)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages (user + merged + separate), got %d", len(result))
	}
	// Merged assistant
	if len(result[1].Content) != 2 {
		t.Errorf("expected 2 blocks in merged assistant, got %d", len(result[1].Content))
	}
	// Separate assistant
	if result[2].Content[0].Text != "different" {
		t.Errorf("expected 'different', got %q", result[2].Content[0].Text)
	}
}

func TestNormalizeMergeConsecutiveAssistantsNoID(t *testing.T) {
	// Assistants without message IDs should not be merged
	msgs := []api.Message{
		userMsg("hello"),
		assistantMsg("first"),
		assistantMsg("second"),
	}

	result := mergeConsecutiveAssistants(msgs)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages (no ID, no merge), got %d", len(result))
	}
}

func TestNormalizeHoistToolResults(t *testing.T) {
	content := []api.ContentBlock{
		{Type: api.ContentText, Text: "text 1"},
		{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: "result 1"},
		{Type: api.ContentText, Text: "text 2"},
		{Type: api.ContentToolResult, ToolUseID: "tu-2", Content: "result 2"},
	}

	result := hoistToolResults(content)

	if len(result) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(result))
	}
	// First two should be tool results
	if result[0].Type != api.ContentToolResult {
		t.Errorf("expected tool_result at 0, got %s", result[0].Type)
	}
	if result[1].Type != api.ContentToolResult {
		t.Errorf("expected tool_result at 1, got %s", result[1].Type)
	}
	// Last two should be text
	if result[2].Type != api.ContentText {
		t.Errorf("expected text at 2, got %s", result[2].Type)
	}
	if result[3].Type != api.ContentText {
		t.Errorf("expected text at 3, got %s", result[3].Type)
	}
}

func TestNormalizeHoistToolResultsNoToolResults(t *testing.T) {
	content := []api.ContentBlock{
		{Type: api.ContentText, Text: "text 1"},
		{Type: api.ContentText, Text: "text 2"},
	}

	result := hoistToolResults(content)

	// Should return original content unchanged (same reference)
	if &result[0] != &content[0] {
		t.Error("expected same reference when no tool results")
	}
}

func TestNormalizeHoistAllToolResults(t *testing.T) {
	msgs := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "text"},
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: "result"},
			},
		},
		assistantMsg("response"),
	}

	result := hoistAllToolResults(msgs)

	// User message should have tool_result first
	if result[0].Content[0].Type != api.ContentToolResult {
		t.Errorf("expected tool_result first, got %s", result[0].Content[0].Type)
	}
	// Assistant message should be unchanged
	if result[1].Content[0].Text != "response" {
		t.Error("assistant should be unchanged")
	}
}

func TestNormalizeSanitizeErrorToolResultContent(t *testing.T) {
	msgs := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "tu-1", Content: "some error", IsError: true},
			},
		},
	}

	result := sanitizeErrorToolResultContent(msgs)

	// In ClawGo, content is already a string so this is a pass-through
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Content[0].Content != "some error" {
		t.Errorf("expected 'some error', got %q", result[0].Content[0].Content)
	}
}

func TestNormalizeIsToolResultMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  api.Message
		want bool
	}{
		{
			name: "tool result user message",
			msg:  toolResultUser("tu-1", "result", false),
			want: true,
		},
		{
			name: "plain user message",
			msg:  userMsg("hello"),
			want: false,
		},
		{
			name: "assistant message",
			msg:  assistantMsg("hi"),
			want: false,
		},
		{
			name: "error tool result",
			msg:  toolResultUser("tu-1", "error", true),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isToolResultMessage(tt.msg)
			if got != tt.want {
				t.Errorf("isToolResultMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeAssistantToolInputsWithValidTools(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		toolUseAssistant("tu-1", "Read", `{"file_path":"/test"}`),
	}

	result := normalizeAssistantToolInputs(msgs, []string{"Read", "Write"})

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	// Tool use should be preserved
	if result[1].Content[0].Name != "Read" {
		t.Errorf("expected 'Read', got %q", result[1].Content[0].Name)
	}
}

func TestNormalizeAssistantToolInputsEmptyToolNames(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		toolUseAssistant("tu-1", "Read", `{"file_path":"/test"}`),
	}

	// Empty tool names should pass through unchanged
	result := normalizeAssistantToolInputs(msgs, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
}

func TestNormalizeAlternatingRolesPreserved(t *testing.T) {
	// Standard conversation should pass through cleanly
	msgs := []api.Message{
		userMsg("hello"),
		assistantMsg("hi"),
		userMsg("how are you"),
		assistantMsg("good"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	expectedRoles := []string{"user", "assistant", "user", "assistant"}
	for i, expected := range expectedRoles {
		if result[i].Role != expected {
			t.Errorf("message %d: expected role %s, got %s", i, expected, result[i].Role)
		}
	}
}

func TestNormalizeComplexPipelineOrder(t *testing.T) {
	// Test the critical ordering bug from TS comments:
	// [text("\n\n"), thinking("...")] should be fully removed
	msgs := []api.Message{
		userMsg("hello"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "\n\n"},
				{Type: api.ContentThinking, Thinking: "deep thoughts"},
			},
		},
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// After pipeline:
	// 1-7: no change
	// 8: not orphaned (has text block)
	// 9: trailing thinking stripped -> [text("\n\n")]
	// 10: whitespace-only removed
	// Result: just the user message
	if len(result) != 1 {
		t.Fatalf("expected 1 message (whitespace assistant removed), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected only user message remaining")
	}
}

func TestNormalizeRedactedThinkingOnlyRemoved(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		redactedThinkingAssistant(),
		userMsg("follow up"),
		assistantMsg("response"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// Redacted thinking-only assistant should be removed, users merged
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected user")
	}
	// Merged user should have both content blocks
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 blocks in merged user, got %d", len(result[0].Content))
	}
}

func TestNormalizeMixedThinkingAndTextPreserved(t *testing.T) {
	// Non-trailing thinking in a non-last assistant should be preserved
	msgs := []api.Message{
		userMsg("hello"),
		thinkingAssistant("some thought", "and text"),
		userMsg("follow up"),
		assistantMsg("final"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	// Second message should still have thinking + text
	if len(result[1].Content) != 2 {
		t.Errorf("expected 2 blocks (thinking + text), got %d", len(result[1].Content))
	}
}

func TestNormalizeMultipleOrphanedThinkingRemoved(t *testing.T) {
	msgs := []api.Message{
		userMsg("a"),
		thinkingOnlyAssistant("orphan 1"),
		userMsg("b"),
		thinkingOnlyAssistant("orphan 2"),
		userMsg("c"),
		assistantMsg("real"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// Both orphans removed, all three users merged
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected merged user")
	}
	if len(result[0].Content) != 3 {
		t.Errorf("expected 3 content blocks in merged user, got %d", len(result[0].Content))
	}
}

func TestNormalizeStripDocumentFromErrorPreceded(t *testing.T) {
	// The backward walk finds user messages only, so the error must be
	// in a user message directly following (no intervening assistant).
	msgs := []api.Message{
		userWithDocument("check this"),
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tu-1",
					Content:   "PDF document too large, request too large",
					IsError:   true,
				},
			},
		},
		assistantMsg("sorry"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// First user (merged with second after stripping) should not have document
	for _, cb := range result[0].Content {
		if cb.Type == api.ContentDocument {
			t.Error("document should have been stripped")
		}
	}
	// Text should remain
	hasText := false
	for _, cb := range result[0].Content {
		if cb.Type == api.ContentText && cb.Text == "check this" {
			hasText = true
		}
	}
	if !hasText {
		t.Error("text block should be preserved after document stripping")
	}
}

func TestNormalizeStripImageFromErrorPreceded(t *testing.T) {
	// Error user message must directly follow the image user message
	// (no intervening assistant) for the backward walk to find it.
	msgs := []api.Message{
		userWithImage("check this", "smalldata"),
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tu-1",
					Content:   "Image too large to process, image exceeds limit",
					IsError:   true,
				},
			},
		},
		assistantMsg("sorry"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	// First user (merged with error user) should not have image
	for _, cb := range result[0].Content {
		if cb.Type == api.ContentImage {
			t.Error("image should have been stripped")
		}
	}
}

func TestNormalizePreservesMessageOrder(t *testing.T) {
	// Verify that the pipeline preserves the order of valid messages
	msgs := []api.Message{
		userMsg("1"),
		assistantMsg("2"),
		userMsg("3"),
		assistantMsg("4"),
		userMsg("5"),
	}

	result := NormalizeMessagesForAPI(msgs, nil)

	if len(result) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(result))
	}

	for i, m := range result {
		expected := string(rune('1' + i))
		if m.Content[0].Text != expected {
			t.Errorf("message %d: expected text %q, got %q", i, expected, m.Content[0].Text)
		}
	}
}

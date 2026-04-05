package query

import (
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// helper to make messages concise in tests
func userMsg(text string) api.Message {
	return api.UserMessage(text)
}

func assistantMsg(text string) api.Message {
	return api.AssistantMessage(text)
}

func thinkingAssistant(thinking, text string) api.Message {
	blocks := []api.ContentBlock{
		{Type: api.ContentThinking, Thinking: thinking},
	}
	if text != "" {
		blocks = append(blocks, api.ContentBlock{Type: api.ContentText, Text: text})
	}
	return api.Message{Role: "assistant", Content: blocks}
}

func thinkingOnlyAssistant(thinking string) api.Message {
	return api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentThinking, Thinking: thinking},
		},
	}
}

func redactedThinkingAssistant() api.Message {
	return api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentRedactedThinking},
		},
	}
}

func assistantWithTrailingThinking(text, thinking string) api.Message {
	return api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
			{Type: api.ContentThinking, Thinking: thinking},
		},
	}
}

func whitespaceAssistant() api.Message {
	return api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: "  \n\n  "},
		},
	}
}

func emptyContentAssistant() api.Message {
	return api.Message{
		Role:    "assistant",
		Content: []api.ContentBlock{},
	}
}

// --- HasThinkingContent tests ---

func TestThinkingHasThinkingContent(t *testing.T) {
	tests := []struct {
		name string
		msg  api.Message
		want bool
	}{
		{
			name: "text only",
			msg:  assistantMsg("hello"),
			want: false,
		},
		{
			name: "thinking block",
			msg:  thinkingAssistant("let me think", "answer"),
			want: true,
		},
		{
			name: "redacted thinking",
			msg:  redactedThinkingAssistant(),
			want: true,
		},
		{
			name: "user message",
			msg:  userMsg("hello"),
			want: false,
		},
		{
			name: "empty content",
			msg:  emptyContentAssistant(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasThinkingContent(tt.msg)
			if got != tt.want {
				t.Errorf("HasThinkingContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- FilterTrailingThinking tests ---

func TestThinkingFilterTrailingThinkingRemovesTrailing(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantWithTrailingThinking("response", "trailing thought"),
	}

	result := FilterTrailingThinking(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	last := result[1]
	if len(last.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(last.Content))
	}
	if last.Content[0].Type != api.ContentText {
		t.Errorf("expected text block, got %s", last.Content[0].Type)
	}
	if last.Content[0].Text != "response" {
		t.Errorf("expected 'response', got %q", last.Content[0].Text)
	}
}

func TestThinkingFilterTrailingThinkingAllThinking(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		thinkingOnlyAssistant("all thinking"),
	}

	result := FilterTrailingThinking(msgs)
	last := result[1]
	if len(last.Content) != 1 {
		t.Fatalf("expected 1 placeholder block, got %d", len(last.Content))
	}
	if last.Content[0].Text != "[No message content]" {
		t.Errorf("expected placeholder, got %q", last.Content[0].Text)
	}
}

func TestThinkingFilterTrailingThinkingUserLast(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantMsg("hi"),
		userMsg("bye"),
	}

	result := FilterTrailingThinking(msgs)
	// Should be unchanged since last message is user
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[2].Content[0].Text != "bye" {
		t.Error("last message should be unchanged")
	}
}

func TestThinkingFilterTrailingThinkingNoTrailing(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		thinkingAssistant("thought", "answer"),
	}

	result := FilterTrailingThinking(msgs)
	// Thinking is NOT trailing (text comes after), so unchanged
	if len(result[1].Content) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(result[1].Content))
	}
}

func TestThinkingFilterTrailingThinkingEmpty(t *testing.T) {
	result := FilterTrailingThinking(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestThinkingFilterTrailingDoesNotModifyEarlierAssistants(t *testing.T) {
	msgs := []api.Message{
		userMsg("first"),
		assistantWithTrailingThinking("earlier", "thinking here"),
		userMsg("second"),
		assistantMsg("final"),
	}

	result := FilterTrailingThinking(msgs)
	// Earlier assistant with trailing thinking should NOT be modified
	if len(result[1].Content) != 2 {
		t.Errorf("earlier assistant should be unchanged, got %d blocks", len(result[1].Content))
	}
}

// --- FilterOrphanedThinkingMessages tests ---

func TestThinkingFilterOrphanedRemovesThinkingOnly(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		thinkingOnlyAssistant("orphan thought"),
		userMsg("follow up"),
		assistantMsg("real response"),
	}

	result := FilterOrphanedThinkingMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages (orphan removed), got %d", len(result))
	}
	// Should be: user, user, assistant
	if result[0].Role != "user" {
		t.Error("first should be user")
	}
	if result[1].Role != "user" {
		t.Error("second should be user")
	}
	if result[2].Role != "assistant" {
		t.Error("third should be assistant")
	}
}

func TestThinkingFilterOrphanedKeepsMixed(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		thinkingAssistant("thought", "also text"),
	}

	result := FilterOrphanedThinkingMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (mixed kept), got %d", len(result))
	}
}

func TestThinkingFilterOrphanedRedactedThinking(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		redactedThinkingAssistant(),
		userMsg("follow up"),
	}

	result := FilterOrphanedThinkingMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (redacted orphan removed), got %d", len(result))
	}
}

func TestThinkingFilterOrphanedEmptyContent(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		emptyContentAssistant(),
	}

	result := FilterOrphanedThinkingMessages(msgs)
	// Empty content assistant is kept (not thinking-only)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
}

// --- FilterWhitespaceOnlyAssistants tests ---

func TestThinkingFilterWhitespaceRemovesWhitespace(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		whitespaceAssistant(),
		userMsg("follow up"),
	}

	result := FilterWhitespaceOnlyAssistants(msgs)
	// Whitespace assistant removed, two user messages merged
	if len(result) != 1 {
		t.Fatalf("expected 1 merged user message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected user message")
	}
	// Should have content blocks from both user messages
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged message, got %d", len(result[0].Content))
	}
}

func TestThinkingFilterWhitespaceKeepsNonWhitespace(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantMsg("real content"),
	}

	result := FilterWhitespaceOnlyAssistants(msgs)
	if len(result) != 2 {
		t.Errorf("expected 2 messages unchanged, got %d", len(result))
	}
}

func TestThinkingFilterWhitespaceKeepsToolUse(t *testing.T) {
	// Message with whitespace text AND tool_use should be kept
	msg := api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: "  \n\n  "},
			{Type: api.ContentToolUse, ID: "tool-1", Name: "read", Input: []byte(`{}`)},
		},
	}
	msgs := []api.Message{userMsg("hello"), msg}

	result := FilterWhitespaceOnlyAssistants(msgs)
	if len(result) != 2 {
		t.Errorf("expected 2 messages (tool_use kept), got %d", len(result))
	}
}

func TestThinkingFilterWhitespaceEmpty(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		emptyContentAssistant(),
	}

	result := FilterWhitespaceOnlyAssistants(msgs)
	// Empty content is not whitespace-only -- it's kept
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

// --- EnsureNonEmptyAssistantContent tests ---

func TestThinkingEnsureNonEmptyAddsPlaceholder(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		emptyContentAssistant(), // non-final, empty
		userMsg("follow up"),
		assistantMsg("final"),
	}

	result := EnsureNonEmptyAssistantContent(msgs)
	if len(result[1].Content) != 1 {
		t.Fatalf("expected 1 placeholder block, got %d", len(result[1].Content))
	}
	if result[1].Content[0].Text != "[No content]" {
		t.Errorf("expected placeholder text, got %q", result[1].Content[0].Text)
	}
}

func TestThinkingEnsureNonEmptySkipsFinal(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		emptyContentAssistant(), // final, should stay empty
	}

	result := EnsureNonEmptyAssistantContent(msgs)
	if len(result[1].Content) != 0 {
		t.Errorf("final assistant should stay empty, got %d blocks", len(result[1].Content))
	}
}

func TestThinkingEnsureNonEmptyNoChange(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		assistantMsg("real content"),
	}

	result := EnsureNonEmptyAssistantContent(msgs)
	if len(result) != 2 {
		t.Errorf("expected 2 messages unchanged, got %d", len(result))
	}
}

func TestThinkingEnsureNonEmptyEmptySlice(t *testing.T) {
	result := EnsureNonEmptyAssistantContent(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

// --- EnforceThinkingRules (full pipeline) tests ---

func TestThinkingEnforceThinkingRulesFullPipeline(t *testing.T) {
	// Complex case: orphaned thinking, trailing thinking, whitespace assistant
	msgs := []api.Message{
		userMsg("first"),
		thinkingOnlyAssistant("orphan"),                 // Step 1: removed by FilterOrphanedThinkingMessages
		userMsg("second"),
		assistantWithTrailingThinking("ok", "trailing"), // Step 2: trailing thinking stripped (but not last msg)
		userMsg("third"),
		whitespaceAssistant(),                           // Step 3: whitespace removed, adjacent users merged
		userMsg("fourth"),
	}

	result := EnforceThinkingRules(msgs)

	// Trace through pipeline:
	// 1. FilterOrphanedThinkingMessages: removes thinking-only orphan
	//    -> [user("first"), user("second"), assistant("ok"+thinking), user("third"), wsAssistant, user("fourth")]
	// 2. FilterTrailingThinking: last message is user("fourth"), no change
	// 3. FilterWhitespaceOnlyAssistants: removes wsAssistant, then merge pass
	//    catches ALL adjacent users including first+second (made adjacent by step 1)
	//    -> [merged_user("first"+"second"), assistant("ok"+thinking), merged_user("third"+"fourth")]
	// 4. EnsureNonEmptyAssistantContent: no empty non-final assistants, no change
	//
	// Result: 3 messages

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Merged first + second users
	if result[0].Role != "user" {
		t.Error("expected merged user message first")
	}
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged user (first+second), got %d", len(result[0].Content))
	}
	if result[0].Content[0].Text != "first" {
		t.Errorf("expected 'first', got %q", result[0].Content[0].Text)
	}
	if result[0].Content[1].Text != "second" {
		t.Errorf("expected 'second', got %q", result[0].Content[1].Text)
	}

	// Assistant with thinking (thinking preserved since text comes before thinking)
	if result[1].Role != "assistant" {
		t.Error("expected assistant message")
	}
	if len(result[1].Content) != 2 {
		t.Errorf("expected 2 blocks in assistant (text+thinking), got %d", len(result[1].Content))
	}

	// Merged third + fourth users
	if result[2].Role != "user" {
		t.Error("expected merged user message")
	}
	if len(result[2].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged user (third+fourth), got %d", len(result[2].Content))
	}
}

func TestThinkingEnforceThinkingRulesNoThinking(t *testing.T) {
	// Normal conversation with no thinking -- should pass through unchanged
	msgs := []api.Message{
		userMsg("hello"),
		assistantMsg("hi there"),
		userMsg("how are you"),
		assistantMsg("I'm well"),
	}

	result := EnforceThinkingRules(msgs)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages unchanged, got %d", len(result))
	}
	for i, msg := range result {
		if msg.Role != msgs[i].Role {
			t.Errorf("message %d role mismatch: %s vs %s", i, msg.Role, msgs[i].Role)
		}
	}
}

func TestThinkingEnforceThinkingRulesEmptyInput(t *testing.T) {
	result := EnforceThinkingRules(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestThinkingEnforceThinkingRulesOrderMatters(t *testing.T) {
	// The specific bug from TS comments: a message like [text("\n\n"), thinking("...")]
	// If you filter whitespace first: it survives (has non-text block)
	// Then thinking strip removes thinking, leaving [text("\n\n")] which API rejects
	//
	// Correct order: strip thinking first, then filter whitespace
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

	result := EnforceThinkingRules(msgs)
	// After FilterOrphanedThinkingMessages: unchanged (has text block too)
	// After FilterTrailingThinking: thinking stripped -> [text("\n\n")]
	// After FilterWhitespaceOnlyAssistants: whitespace-only removed
	// Result: just the user message
	if len(result) != 1 {
		t.Fatalf("expected 1 message (whitespace assistant removed), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Error("expected only user message remaining")
	}
}

func TestThinkingEnforceThinkingRulesNonFinalEmptyAssistant(t *testing.T) {
	msgs := []api.Message{
		userMsg("hello"),
		emptyContentAssistant(),
		userMsg("follow up"),
		assistantMsg("final"),
	}

	result := EnforceThinkingRules(msgs)
	// Non-final empty assistant should get placeholder
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	if len(result[1].Content) != 1 {
		t.Fatalf("expected 1 placeholder block, got %d", len(result[1].Content))
	}
	if result[1].Content[0].Text != "[No content]" {
		t.Errorf("expected '[No content]' placeholder, got %q", result[1].Content[0].Text)
	}
}

package tools

import (
	"strings"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// testSizer implements ToolResultSizer for testing with custom per-tool limits.
type testSizer struct {
	limits map[string]int
}

func (s *testSizer) MaxResultSizeChars(toolName string) int {
	if limit, ok := s.limits[toolName]; ok {
		return limit
	}
	return DefaultMaxResultSizeChars
}

func TestBudgetShortResultsPassThrough(t *testing.T) {
	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-1",
					Content:   "short result",
					Name:      "Bash",
				},
			},
		},
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "ok"},
			},
		},
	}

	result := ApplyToolResultBudget(messages, nil)

	// Should return the original slice (no modifications)
	if &result[0] != &messages[0] {
		t.Error("expected original slice to be returned for short results")
	}

	// Verify content is unchanged
	if result[0].Content[0].Content != "short result" {
		t.Errorf("expected 'short result', got %q", result[0].Content[0].Content)
	}
}

func TestBudgetResultExceedingDefaultMaxIsTruncated(t *testing.T) {
	// Create a result that exceeds DefaultMaxResultSizeChars (50k)
	longContent := strings.Repeat("x", DefaultMaxResultSizeChars+1000)

	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-1",
					Content:   longContent,
					Name:      "Bash",
				},
			},
		},
	}

	result := ApplyToolResultBudget(messages, nil)

	content := result[0].Content[0].Content

	// Should be truncated with marker
	if !strings.Contains(content, "Output truncated") {
		t.Error("expected truncation marker in result")
	}

	// The first part should be the original content up to the limit
	if !strings.HasPrefix(content, strings.Repeat("x", DefaultMaxResultSizeChars)) {
		t.Error("expected truncated content to start with original content")
	}

	// Should mention the total character count
	expectedTotal := DefaultMaxResultSizeChars + 1000
	if !strings.Contains(content, "51000") {
		t.Errorf("expected total char count %d in marker, got content ending: %s",
			expectedTotal, content[len(content)-100:])
	}

	// The truncated content should be shorter than the original
	if len(content) >= len(longContent) {
		t.Error("truncated content should be shorter than original")
	}
}

func TestBudgetPerMessageBudgetEnforcement(t *testing.T) {
	// Create multiple tool results that together exceed MaxToolResultsPerMessageChars (200k)
	// Each result is 80k chars, 3 of them = 240k > 200k budget
	singleSize := 80_000
	content1 := strings.Repeat("a", singleSize)
	content2 := strings.Repeat("b", singleSize)
	content3 := strings.Repeat("c", singleSize)

	// Use a custom sizer with a high per-tool limit so per-tool truncation doesn't trigger
	sizer := &testSizer{
		limits: map[string]int{
			"ReadTool":  100_000,
			"ReadTool2": 100_000,
			"ReadTool3": 100_000,
		},
	}

	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-1",
					Content:   content1,
					Name:      "ReadTool",
				},
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-2",
					Content:   content2,
					Name:      "ReadTool2",
				},
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-3",
					Content:   content3,
					Name:      "ReadTool3",
				},
			},
		},
	}

	result := ApplyToolResultBudget(messages, sizer)

	// At least one result should have been cleared to stay within budget
	clearedCount := 0
	for _, cb := range result[0].Content {
		if cb.Type == api.ContentToolResult && strings.Contains(cb.Content, "message budget") {
			clearedCount++
		}
	}

	if clearedCount == 0 {
		t.Error("expected at least one result to be cleared for message budget")
	}

	// Total size should now be within budget
	totalSize := 0
	for _, cb := range result[0].Content {
		if cb.Type == api.ContentToolResult {
			totalSize += len(cb.Content)
		}
	}
	if totalSize > MaxToolResultsPerMessageChars {
		t.Errorf("total size %d exceeds budget %d", totalSize, MaxToolResultsPerMessageChars)
	}
}

func TestBudgetCustomMaxResultSizeCharsRespected(t *testing.T) {
	// A tool with a custom limit of 100 chars should be truncated at 100
	customLimit := 100
	sizer := &testSizer{
		limits: map[string]int{
			"SmallTool": customLimit,
		},
	}

	longContent := strings.Repeat("z", 500)

	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-1",
					Content:   longContent,
					Name:      "SmallTool",
				},
			},
		},
	}

	result := ApplyToolResultBudget(messages, sizer)

	content := result[0].Content[0].Content

	// Should contain truncation marker
	if !strings.Contains(content, "Output truncated") {
		t.Error("expected truncation marker for custom limit")
	}

	// Should show first 100 chars of the original content
	if !strings.HasPrefix(content, strings.Repeat("z", customLimit)) {
		t.Error("expected content to start with first 100 chars of original")
	}

	// Should mention showing first 100 of 500
	if !strings.Contains(content, "100") || !strings.Contains(content, "500") {
		t.Errorf("expected marker to mention 100 of 500, got: %s", content[customLimit:])
	}
}

func TestBudgetUnlimitedToolNotTruncated(t *testing.T) {
	// A tool with limit 0 (unlimited) should not be truncated per-tool
	sizer := &testSizer{
		limits: map[string]int{
			"UnlimitedTool": 0,
		},
	}

	longContent := strings.Repeat("u", DefaultMaxResultSizeChars*2)

	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-1",
					Content:   longContent,
					Name:      "UnlimitedTool",
				},
			},
		},
	}

	result := ApplyToolResultBudget(messages, sizer)

	content := result[0].Content[0].Content

	// Should NOT have per-tool truncation marker (limit=0 means unlimited)
	if strings.Contains(content, "Output truncated") {
		t.Error("unlimited tool should not have per-tool truncation")
	}

	// Content should be the full original (per-message budget may or may not trigger
	// depending on total size vs MaxToolResultsPerMessageChars)
	if len(longContent) <= MaxToolResultsPerMessageChars {
		// Under per-message budget, so no truncation at all
		if content != longContent {
			t.Error("expected full content for unlimited tool under message budget")
		}
	}
}

func TestBudgetAssistantMessagesUnchanged(t *testing.T) {
	messages := []api.Message{
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: strings.Repeat("x", DefaultMaxResultSizeChars*2)},
			},
		},
	}

	result := ApplyToolResultBudget(messages, nil)

	// Should return original slice -- assistant messages are never budgeted
	if &result[0] != &messages[0] {
		t.Error("expected original slice for assistant messages")
	}
}

func TestBudgetNonToolResultBlocksPreserved(t *testing.T) {
	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "some text"},
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tool-1",
					Content:   "short result",
					Name:      "Bash",
				},
			},
		},
	}

	result := ApplyToolResultBudget(messages, nil)

	// Non-tool-result blocks should be preserved
	if result[0].Content[0].Type != api.ContentText {
		t.Error("expected text block to be preserved")
	}
	if result[0].Content[0].Text != "some text" {
		t.Error("expected text content to be unchanged")
	}
}

func TestBudgetLargestBlocksClearedFirst(t *testing.T) {
	// When per-message budget is exceeded, largest blocks are cleared first.
	// Create 3 blocks: 50k, 100k, 60k = 210k > 200k budget
	sizer := &testSizer{
		limits: map[string]int{
			"Tool1": 120_000,
			"Tool2": 120_000,
			"Tool3": 120_000,
		},
	}

	content1 := strings.Repeat("a", 50_000)  // smallest
	content2 := strings.Repeat("b", 100_000) // largest
	content3 := strings.Repeat("c", 60_000)  // medium

	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "t1", Content: content1, Name: "Tool1"},
				{Type: api.ContentToolResult, ToolUseID: "t2", Content: content2, Name: "Tool2"},
				{Type: api.ContentToolResult, ToolUseID: "t3", Content: content3, Name: "Tool3"},
			},
		},
	}

	result := ApplyToolResultBudget(messages, sizer)

	// The largest block (100k, tool-2) should be cleared first
	blocks := result[0].Content
	if !strings.Contains(blocks[1].Content, "message budget") {
		t.Error("expected largest block (index 1) to be cleared for message budget")
	}

	// The smallest block should be preserved
	if strings.Contains(blocks[0].Content, "message budget") {
		t.Error("expected smallest block (index 0) to NOT be cleared")
	}
}

func TestBudgetEmptyMessagesHandled(t *testing.T) {
	// Empty message slice should return without error
	result := ApplyToolResultBudget(nil, nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	result = ApplyToolResultBudget([]api.Message{}, nil)
	if len(result) != 0 {
		t.Error("expected empty slice for empty input")
	}
}

func TestBudgetToolUseIDPreserved(t *testing.T) {
	// When truncating, the ToolUseID should be preserved
	longContent := strings.Repeat("x", DefaultMaxResultSizeChars+100)

	messages := []api.Message{
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "unique-tool-id-123",
					Content:   longContent,
					Name:      "Bash",
				},
			},
		},
	}

	result := ApplyToolResultBudget(messages, nil)

	if result[0].Content[0].ToolUseID != "unique-tool-id-123" {
		t.Errorf("expected ToolUseID to be preserved, got %q", result[0].Content[0].ToolUseID)
	}
}

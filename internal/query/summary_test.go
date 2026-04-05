package query

import (
	"encoding/json"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractToolPairs_Empty(t *testing.T) {
	result := extractToolPairs(nil)
	assert.Nil(t, result)

	result = extractToolPairs([]api.Message{})
	assert.Nil(t, result)
}

func TestExtractToolPairs_NoToolUse(t *testing.T) {
	msgs := []api.Message{
		api.UserMessage("hello"),
		api.AssistantMessage("hi there"),
	}
	result := extractToolPairs(msgs)
	assert.Nil(t, result)
}

func TestExtractToolPairs_SingleTool(t *testing.T) {
	msgs := []api.Message{
		api.UserMessage("read the file"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "I'll read the file."},
				{
					Type:  api.ContentToolUse,
					ID:    "tu_1",
					Name:  "Read",
					Input: json.RawMessage(`{"file_path":"/test.go"}`),
				},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tu_1",
					Content:   "package main\nfunc main() {}",
				},
			},
		},
	}

	result := extractToolPairs(msgs)
	require.Len(t, result, 1)
	assert.Equal(t, "Read", result[0].Name)
	assert.Contains(t, result[0].Input, "file_path")
	assert.Contains(t, result[0].Output, "package main")
}

func TestExtractToolPairs_MultipleTool(t *testing.T) {
	msgs := []api.Message{
		api.UserMessage("find and read files"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{
					Type:  api.ContentToolUse,
					ID:    "tu_1",
					Name:  "Glob",
					Input: json.RawMessage(`{"pattern":"*.go"}`),
				},
				{
					Type:  api.ContentToolUse,
					ID:    "tu_2",
					Name:  "Read",
					Input: json.RawMessage(`{"file_path":"main.go"}`),
				},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tu_1",
					Content:   "main.go\ntest.go",
				},
				{
					Type:      api.ContentToolResult,
					ToolUseID: "tu_2",
					Content:   "func main() {}",
				},
			},
		},
	}

	result := extractToolPairs(msgs)
	require.Len(t, result, 2)
	assert.Equal(t, "Glob", result[0].Name)
	assert.Equal(t, "Read", result[1].Name)
	assert.Contains(t, result[0].Output, "main.go")
	assert.Contains(t, result[1].Output, "func main")
}

func TestExtractToolPairs_OnlyLastAssistant(t *testing.T) {
	// Ensure we only extract from the last assistant message with tools
	msgs := []api.Message{
		api.UserMessage("first"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{
					Type:  api.ContentToolUse,
					ID:    "old_tu",
					Name:  "Bash",
					Input: json.RawMessage(`{"command":"ls"}`),
				},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "old_tu", Content: "old result"},
			},
		},
		api.AssistantMessage("thinking..."),
		api.UserMessage("second"),
		{
			Role: "assistant",
			Content: []api.ContentBlock{
				{
					Type:  api.ContentToolUse,
					ID:    "new_tu",
					Name:  "Read",
					Input: json.RawMessage(`{"file_path":"new.go"}`),
				},
			},
		},
		{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, ToolUseID: "new_tu", Content: "new result"},
			},
		},
	}

	result := extractToolPairs(msgs)
	require.Len(t, result, 1)
	assert.Equal(t, "Read", result[0].Name)
	assert.Contains(t, result[0].Output, "new result")
}

func TestTruncateJSON(t *testing.T) {
	short := json.RawMessage(`{"key":"val"}`)
	assert.Equal(t, `{"key":"val"}`, truncateJSON(short, 300))

	long := json.RawMessage(`{"key":"` + string(make([]byte, 400)) + `"}`)
	result := truncateJSON(long, 300)
	assert.Len(t, result, 300)
	assert.True(t, len(result) <= 300)
	assert.Contains(t, result, "...")
}

func TestTruncateJSON_Nil(t *testing.T) {
	assert.Equal(t, "{}", truncateJSON(nil, 300))
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "hello", truncateString("hello", 300))

	long := string(make([]byte, 400))
	result := truncateString(long, 300)
	assert.Len(t, result, 300)
	assert.Contains(t, result, "...")
}

func TestCreateToolUseSummaryMessage(t *testing.T) {
	msg := CreateToolUseSummaryMessage("Read config.json")
	assert.Equal(t, "user", msg.Role)
	assert.Len(t, msg.Content, 1)
	assert.Contains(t, msg.Content[0].Text, "Tool use summary:")
	assert.Contains(t, msg.Content[0].Text, "Read config.json")
}

func TestToolUseSummarySystemPrompt_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, ToolUseSummarySystemPrompt)
	assert.Contains(t, ToolUseSummarySystemPrompt, "summary label")
	assert.Contains(t, ToolUseSummarySystemPrompt, "past tense")
}

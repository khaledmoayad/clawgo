package compact

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/khaledmoayad/clawgo/internal/api"
)

func TestSnipIsSnipEnabled(t *testing.T) {
	assert.True(t, IsSnipEnabled(), "snip should be enabled by default")
}

func TestSnipIsSnipRuntimeEnabled(t *testing.T) {
	assert.True(t, IsSnipRuntimeEnabled())
}

func TestSnipConversation_ShortConversationUnchanged(t *testing.T) {
	// Fewer than SnipRecencyWindow messages should be returned unchanged
	messages := make([]api.Message, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			messages[i] = api.UserMessage("query")
		} else {
			messages[i] = api.AssistantMessage("response")
		}
	}

	result := SnipConversation(messages, "claude-sonnet-4-20250514")
	assert.Equal(t, messages, result, "short conversations should be unchanged")
}

func TestSnipConversation_OldToolResultsSnipped(t *testing.T) {
	// Build a conversation with 25 messages where the first few have
	// large tool results that should be snipped
	longContent := strings.Repeat("x", SnipMinResultLength+100)
	toolUseID := "tool-use-123"

	messages := make([]api.Message, 25)

	// Message 0: assistant with tool_use
	messages[0] = api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{
				Type:  api.ContentToolUse,
				ID:    toolUseID,
				Name:  "Bash",
				Input: json.RawMessage(`{"command":"ls -la"}`),
			},
		},
	}

	// Message 1: user with tool_result (large content)
	messages[1] = api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{
				Type:      api.ContentToolResult,
				ToolUseID: toolUseID,
				Content:   longContent,
			},
		},
	}

	// Fill remaining messages
	for i := 2; i < 25; i++ {
		if i%2 == 0 {
			messages[i] = api.UserMessage("query")
		} else {
			messages[i] = api.AssistantMessage("response")
		}
	}

	result := SnipConversation(messages, "claude-sonnet-4-20250514")
	require.Len(t, result, 25)

	// The tool result (message 1) should be snipped
	assert.Equal(t, SnipReplacementMessage, result[1].Content[0].Content,
		"old large tool result should be snipped")

	// The tool_use input (message 0) should also be snipped
	assert.Contains(t, string(result[0].Content[0].Input), SnipInputReplacementMessage,
		"corresponding tool_use input should be snipped")
}

func TestSnipConversation_RecentMessagesProtected(t *testing.T) {
	longContent := strings.Repeat("x", SnipMinResultLength+100)
	toolUseID := "tool-recent"

	messages := make([]api.Message, 25)

	// Fill first messages as plain text
	for i := 0; i < 23; i++ {
		if i%2 == 0 {
			messages[i] = api.UserMessage("query")
		} else {
			messages[i] = api.AssistantMessage("response")
		}
	}

	// Message 23 (within recency window): assistant with tool_use
	messages[23] = api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{
				Type:  api.ContentToolUse,
				ID:    toolUseID,
				Name:  "Bash",
				Input: json.RawMessage(`{"command":"ls"}`),
			},
		},
	}

	// Message 24 (within recency window): user with large tool_result
	messages[24] = api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{
				Type:      api.ContentToolResult,
				ToolUseID: toolUseID,
				Content:   longContent,
			},
		},
	}

	result := SnipConversation(messages, "claude-sonnet-4-20250514")
	require.Len(t, result, 25)

	// Recent messages should NOT be snipped (within recency window)
	assert.Equal(t, longContent, result[24].Content[0].Content,
		"recent tool results should be protected")
}

func TestSnipConversation_ShortResultsPreserved(t *testing.T) {
	shortContent := "short result"
	toolUseID := "tool-short"

	messages := make([]api.Message, 25)

	// Message 0: assistant with tool_use
	messages[0] = api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{
				Type:  api.ContentToolUse,
				ID:    toolUseID,
				Name:  "Read",
				Input: json.RawMessage(`{"path":"/tmp/file"}`),
			},
		},
	}

	// Message 1: user with short tool_result
	messages[1] = api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{
				Type:      api.ContentToolResult,
				ToolUseID: toolUseID,
				Content:   shortContent,
			},
		},
	}

	// Fill remaining
	for i := 2; i < 25; i++ {
		if i%2 == 0 {
			messages[i] = api.UserMessage("query")
		} else {
			messages[i] = api.AssistantMessage("response")
		}
	}

	result := SnipConversation(messages, "claude-sonnet-4-20250514")
	require.Len(t, result, 25)

	// Short results should NOT be snipped
	assert.Equal(t, shortContent, result[1].Content[0].Content,
		"short tool results below threshold should be preserved")
}

func TestSnipConversation_ToolUseInputSnipped(t *testing.T) {
	longContent := strings.Repeat("y", SnipMinResultLength+1)
	toolUseID := "tool-input-snip"
	originalInput := json.RawMessage(`{"command":"echo hello world"}`)

	messages := make([]api.Message, 25)

	// Message 0: assistant with tool_use
	messages[0] = api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{
				Type:  api.ContentToolUse,
				ID:    toolUseID,
				Name:  "Bash",
				Input: originalInput,
			},
		},
	}

	// Message 1: user with large tool_result
	messages[1] = api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{
				Type:      api.ContentToolResult,
				ToolUseID: toolUseID,
				Content:   longContent,
			},
		},
	}

	for i := 2; i < 25; i++ {
		if i%2 == 0 {
			messages[i] = api.UserMessage("q")
		} else {
			messages[i] = api.AssistantMessage("r")
		}
	}

	result := SnipConversation(messages, "claude-sonnet-4-20250514")

	// Verify the tool_use input was snipped
	assert.NotEqual(t, string(originalInput), string(result[0].Content[0].Input),
		"tool_use input should be replaced when result is snipped")
	assert.Contains(t, string(result[0].Content[0].Input), SnipInputReplacementMessage)

	// Verify the tool_result was snipped
	assert.Equal(t, SnipReplacementMessage, result[1].Content[0].Content)
}

func TestSnipConversation_NonCompactableToolsPreserved(t *testing.T) {
	longContent := strings.Repeat("z", SnipMinResultLength+100)
	toolUseID := "tool-custom"

	messages := make([]api.Message, 25)

	// Message 0: assistant with non-compactable tool
	messages[0] = api.Message{
		Role: "assistant",
		Content: []api.ContentBlock{
			{
				Type:  api.ContentToolUse,
				ID:    toolUseID,
				Name:  "CustomTool",
				Input: json.RawMessage(`{}`),
			},
		},
	}

	// Message 1: user with large tool_result from non-compactable tool
	messages[1] = api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{
				Type:      api.ContentToolResult,
				ToolUseID: toolUseID,
				Content:   longContent,
			},
		},
	}

	for i := 2; i < 25; i++ {
		if i%2 == 0 {
			messages[i] = api.UserMessage("q")
		} else {
			messages[i] = api.AssistantMessage("r")
		}
	}

	result := SnipConversation(messages, "claude-sonnet-4-20250514")

	// Non-compactable tool results should NOT be snipped
	assert.Equal(t, longContent, result[1].Content[0].Content,
		"non-compactable tool results should be preserved")
}

func TestSnipAppendMessageTag(t *testing.T) {
	t.Run("appends tag to text message", func(t *testing.T) {
		msg := api.UserMessage("Hello world")
		tagged := AppendMessageTag(msg, "msg-123")

		require.Len(t, tagged.Content, 1)
		assert.Equal(t, "Hello world [id:msg-123]", tagged.Content[0].Text)
		assert.Equal(t, "user", tagged.Role)
	})

	t.Run("appends to last text block", func(t *testing.T) {
		msg := api.Message{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: "First"},
				{Type: api.ContentText, Text: "Second"},
			},
		}
		tagged := AppendMessageTag(msg, "abc")

		assert.Equal(t, "First", tagged.Content[0].Text)
		assert.Equal(t, "Second [id:abc]", tagged.Content[1].Text)
	})

	t.Run("no text blocks returns unchanged", func(t *testing.T) {
		msg := api.Message{
			Role: "user",
			Content: []api.ContentBlock{
				{Type: api.ContentToolResult, Content: "result"},
			},
		}
		tagged := AppendMessageTag(msg, "xyz")

		assert.Equal(t, msg.Content[0].Content, tagged.Content[0].Content)
	})

	t.Run("trims trailing spaces before tag", func(t *testing.T) {
		msg := api.UserMessage("Hello   ")
		tagged := AppendMessageTag(msg, "456")

		assert.Equal(t, "Hello [id:456]", tagged.Content[0].Text)
	})
}

func TestSnipConstants(t *testing.T) {
	assert.Equal(t, 20, SnipRecencyWindow, "recency window should be 20")
	assert.Equal(t, 1000, SnipMinResultLength, "min result length should be 1000")
	assert.Contains(t, SnipReplacementMessage, "Tool result snipped")
}

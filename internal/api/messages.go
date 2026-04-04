// Package api provides the Anthropic API client layer for ClawGo.
// It wraps the official anthropic-sdk-go with streaming, retry, and error
// categorization tailored to match the TypeScript Claude Code behavior.
package api

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
)

// ContentBlockType identifies the kind of content within a message.
type ContentBlockType string

const (
	ContentText             ContentBlockType = "text"
	ContentToolUse          ContentBlockType = "tool_use"
	ContentToolResult       ContentBlockType = "tool_result"
	ContentThinking         ContentBlockType = "thinking"
	ContentImage            ContentBlockType = "image"
	ContentDocument         ContentBlockType = "document"
	ContentRedactedThinking ContentBlockType = "redacted_thinking"
)

// ContentBlock represents a single block of content within a message.
// It mirrors the TypeScript ContentBlock union type, using optional fields
// for the various block types.
type ContentBlock struct {
	Type      ContentBlockType `json:"type"`
	Text      string           `json:"text,omitempty"`
	ID        string           `json:"id,omitempty"`          // tool_use ID
	Name      string           `json:"name,omitempty"`        // tool name
	Input     json.RawMessage  `json:"input,omitempty"`       // tool input JSON
	ToolUseID string           `json:"tool_use_id,omitempty"` // for tool_result
	Content   string           `json:"content,omitempty"`     // tool result content
	IsError   bool             `json:"is_error,omitempty"`
	Thinking  string           `json:"thinking,omitempty"`    // thinking text

	// Image support (type="image")
	Source *ImageSource `json:"source,omitempty"`

	// Document support (type="document")
	DocumentSource *DocumentSource `json:"document_source,omitempty"`
}

// ImageSource represents a base64-encoded image to send to the API.
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g., "image/png", "image/jpeg"
	Data      string `json:"data"`       // base64-encoded image data
}

// DocumentSource represents a base64-encoded document (PDF) to send to the API.
type DocumentSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "application/pdf"
	Data      string `json:"data"`       // base64-encoded document data
}

// Message represents a conversation message (user or assistant).
type Message struct {
	Role    string         `json:"role"` // "user" or "assistant"
	Content []ContentBlock `json:"content"`
}

// Usage tracks token counts from an API response.
// Includes all 7+ fields that Claude Code tracks for cost calculation
// and analytics, including cache, server tool use, and ephemeral tokens.
type Usage struct {
	InputTokens              int                  `json:"input_tokens"`
	OutputTokens             int                  `json:"output_tokens"`
	CacheCreationInputTokens int                  `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int                  `json:"cache_read_input_tokens,omitempty"`
	ServerToolUse            *ServerToolUseUsage   `json:"server_tool_use,omitempty"`
	Ephemeral1hInputTokens   int                  `json:"ephemeral_1h_input_tokens,omitempty"`
}

// ServerToolUseUsage tracks usage from server-side tool invocations
// (e.g., web search, web fetch) that the API performs on behalf of the user.
type ServerToolUseUsage struct {
	WebSearchRequests int `json:"web_search_requests,omitempty"`
	WebFetchRequests  int `json:"web_fetch_requests,omitempty"`
}

// ToParam converts a Message to the SDK's MessageParam for sending back to the API.
func (m *Message) ToParam() anthropic.MessageParam {
	var blocks []anthropic.ContentBlockParamUnion
	for _, cb := range m.Content {
		switch cb.Type {
		case ContentText:
			blocks = append(blocks, anthropic.NewTextBlock(cb.Text))
		case ContentThinking:
			blocks = append(blocks, anthropic.NewThinkingBlock("", cb.Thinking))
		case ContentToolUse:
			blocks = append(blocks, anthropic.NewToolUseBlock(cb.ID, cb.Input, cb.Name))
		case ContentToolResult:
			result := anthropic.NewToolResultBlock(cb.ToolUseID, cb.Content, cb.IsError)
			blocks = append(blocks, result)
		case ContentImage:
			if cb.Source != nil {
				blocks = append(blocks, anthropic.NewImageBlockBase64(cb.Source.MediaType, cb.Source.Data))
			}
		case ContentDocument:
			if cb.DocumentSource != nil {
				blocks = append(blocks, anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{
					Data: cb.DocumentSource.Data,
				}))
			}
		case ContentRedactedThinking:
			// Redacted thinking blocks are read-only from the API;
			// they are included as-is in conversation history for continuity.
			// The SDK doesn't have a specific constructor, so we skip them.
		}
	}
	return anthropic.MessageParam{
		Role:    anthropic.MessageParamRole(m.Role),
		Content: blocks,
	}
}

// UserMessage creates a user message with a single text content block.
func UserMessage(text string) Message {
	return Message{
		Role: "user",
		Content: []ContentBlock{
			{Type: ContentText, Text: text},
		},
	}
}

// AssistantMessage creates an assistant message with a single text content block.
func AssistantMessage(text string) Message {
	return Message{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: ContentText, Text: text},
		},
	}
}

// ToolResultMessage creates a user message containing a tool result.
func ToolResultMessage(toolUseID, content string, isError bool) Message {
	return Message{
		Role: "user",
		Content: []ContentBlock{
			{
				Type:      ContentToolResult,
				ToolUseID: toolUseID,
				Content:   content,
				IsError:   isError,
			},
		},
	}
}

// ToolResultEntry represents a single tool result to be sent back to the API.
type ToolResultEntry struct {
	ToolUseID string
	Content   string
	IsError   bool
}

// ToolResultsMessage creates a user message containing multiple tool results.
// This is used when the assistant message contains multiple tool_use blocks
// and all results need to be sent back in a single user message.
func ToolResultsMessage(results []ToolResultEntry) Message {
	blocks := make([]ContentBlock, 0, len(results))
	for _, r := range results {
		blocks = append(blocks, ContentBlock{
			Type:      ContentToolResult,
			ToolUseID: r.ToolUseID,
			Content:   r.Content,
			IsError:   r.IsError,
		})
	}
	return Message{
		Role:    "user",
		Content: blocks,
	}
}

// MessageFromResponse converts an Anthropic SDK Message to our Message type.
// This extracts the content blocks from the API response into our internal
// representation for conversation history tracking.
func MessageFromResponse(msg *anthropic.Message) Message {
	blocks := make([]ContentBlock, 0, len(msg.Content))
	for _, cb := range msg.Content {
		switch cb.Type {
		case "text":
			blocks = append(blocks, ContentBlock{
				Type: ContentText,
				Text: cb.Text,
			})
		case "tool_use":
			blocks = append(blocks, ContentBlock{
				Type:  ContentToolUse,
				ID:    cb.ID,
				Name:  cb.Name,
				Input: cb.Input,
			})
		case "thinking":
			blocks = append(blocks, ContentBlock{
				Type:     ContentThinking,
				Thinking: cb.Thinking,
			})
		case "redacted_thinking":
			blocks = append(blocks, ContentBlock{
				Type: ContentRedactedThinking,
			})
		}
	}
	return Message{
		Role:    string(msg.Role),
		Content: blocks,
	}
}

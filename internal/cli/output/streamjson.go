package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// StreamJSONWriter writes newline-delimited JSON messages to an io.Writer.
// Thread-safe for concurrent tool execution output.
type StreamJSONWriter struct {
	w  io.Writer
	mu sync.Mutex
}

// NewStreamJSONWriter creates a writer that outputs NDJSON to w.
func NewStreamJSONWriter(w io.Writer) *StreamJSONWriter {
	return &StreamJSONWriter{w: w}
}

// WriteMessage serializes msg as JSON and writes it followed by a newline.
// Each message is a complete JSON object on its own line.
func (s *StreamJSONWriter) WriteMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal stream message: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = fmt.Fprintf(s.w, "%s\n", data)
	return err
}

// WriteAssistant writes an assistant message to the stream.
func (s *StreamJSONWriter) WriteAssistant(content []ContentBlock, sessionID string, model string) error {
	return s.WriteMessage(AssistantMessage{
		Type:      TypeAssistant,
		SessionID: sessionID,
		Message: ContentMessage{
			Role:    "assistant",
			Content: content,
			Model:   model,
		},
	})
}

// WriteToolUse writes a tool invocation message to the stream.
func (s *StreamJSONWriter) WriteToolUse(id, name string, input any, sessionID string) error {
	return s.WriteMessage(ToolUseMessage{
		Type:      TypeToolUse,
		ID:        id,
		Name:      name,
		Input:     input,
		SessionID: sessionID,
	})
}

// WriteToolResult writes a tool result message to the stream.
func (s *StreamJSONWriter) WriteToolResult(toolUseID, content string, isError bool, sessionID string) error {
	return s.WriteMessage(ToolResultMsg{
		Type:      TypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
		SessionID: sessionID,
	})
}

// WriteResult writes the final result message to the stream.
func (s *StreamJSONWriter) WriteResult(result *ResultMessage) error {
	return s.WriteMessage(result)
}

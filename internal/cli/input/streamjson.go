// Package input handles reading user input in various formats for
// non-interactive mode. It supports plain text (default) and stream-json
// (NDJSON from SDK hosts).
package input

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// UserMessage represents an incoming user message in stream-json input format.
type UserMessage struct {
	Type    string `json:"type"` // "user"
	Message struct {
		Role    string `json:"role"`    // "user"
		Content string `json:"content"`
	} `json:"message"`
}

// StreamJSONReader reads newline-delimited JSON messages from an io.Reader.
// Used when --input-format=stream-json to receive structured messages from
// SDK hosts instead of plain text from stdin.
type StreamJSONReader struct {
	scanner *bufio.Scanner
}

// NewStreamJSONReader creates a reader that parses NDJSON from r.
func NewStreamJSONReader(r io.Reader) *StreamJSONReader {
	scanner := bufio.NewScanner(r)
	// 1MB buffer for large messages (matching Claude Code's behavior)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	return &StreamJSONReader{scanner: scanner}
}

// ReadMessage reads the next JSON message from the input stream.
// Returns io.EOF when no more messages are available.
func (s *StreamJSONReader) ReadMessage() (*UserMessage, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read stream input: %w", err)
		}
		return nil, io.EOF
	}
	line := s.scanner.Bytes()
	var msg UserMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("parse stream input message: %w", err)
	}
	return &msg, nil
}

// ReadPrompt reads from r based on the input format.
// For "text" format, reads all of stdin as a single string.
// For "stream-json" format, reads the first user message.
func ReadPrompt(r io.Reader, format string) (string, error) {
	switch format {
	case "stream-json":
		reader := NewStreamJSONReader(r)
		msg, err := reader.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("read stream-json input: %w", err)
		}
		if msg.Type != "user" {
			return "", fmt.Errorf("expected user message, got %q", msg.Type)
		}
		return msg.Message.Content, nil
	default: // "text"
		data, err := io.ReadAll(r)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	}
}

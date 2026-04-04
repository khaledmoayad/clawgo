package lspclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMessage(t *testing.T) {
	// Create a client with a pipe for stdin so we can capture output
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	c := &Client{
		stdin:   pw,
		pending: make(map[int64]chan json.RawMessage),
		done:    make(chan struct{}),
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/method",
	}

	// Write in a goroutine since pipe is synchronous
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.writeMessage(msg)
	}()

	// Read the framed output
	reader := bufio.NewReader(pr)
	headerLine, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, headerLine, "Content-Length:")

	// Read the blank line separator
	blank, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "\r\n", blank)

	// Parse Content-Length
	var contentLen int
	_, err = fmt.Sscanf(headerLine, "Content-Length: %d\r\n", &contentLen)
	require.NoError(t, err)
	assert.Greater(t, contentLen, 0)

	// Read the body
	body := make([]byte, contentLen)
	_, err = io.ReadFull(reader, body)
	require.NoError(t, err)

	var parsed JSONRPCMessage
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, "2.0", parsed.JSONRPC)
	assert.Equal(t, "test/method", parsed.Method)

	require.NoError(t, <-errCh)
}

func TestReadMessage(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"test/notification"}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	c := &Client{
		stdout:  bufio.NewReader(bytes.NewReader([]byte(frame))),
		pending: make(map[int64]chan json.RawMessage),
		done:    make(chan struct{}),
	}

	raw, err := c.readMessage()
	require.NoError(t, err)

	var msg JSONRPCMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, "2.0", msg.JSONRPC)
	assert.Equal(t, "test/notification", msg.Method)
}

func TestReadMessage_MultipleHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1}`
	frame := fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(body), body)

	c := &Client{
		stdout:  bufio.NewReader(bytes.NewReader([]byte(frame))),
		pending: make(map[int64]chan json.RawMessage),
		done:    make(chan struct{}),
	}

	raw, err := c.readMessage()
	require.NoError(t, err)
	assert.NotNil(t, raw)
}

func TestReadMessage_MissingContentLength(t *testing.T) {
	frame := "Some-Header: value\r\n\r\n{}"

	c := &Client{
		stdout:  bufio.NewReader(bytes.NewReader([]byte(frame))),
		pending: make(map[int64]chan json.RawMessage),
		done:    make(chan struct{}),
	}

	_, err := c.readMessage()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Content-Length")
}

func TestProtocolTypes(t *testing.T) {
	params := InitializeParams{
		ProcessID: 1234,
		RootURI:   "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocClientCap{
				PublishDiagnostics: &PublishDiagCap{
					RelatedInformation: true,
				},
			},
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, float64(1234), m["processId"])
	assert.Equal(t, "file:///workspace", m["rootUri"])

	caps, ok := m["capabilities"].(map[string]interface{})
	require.True(t, ok)
	td, ok := caps["textDocument"].(map[string]interface{})
	require.True(t, ok)
	pd, ok := td["publishDiagnostics"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, pd["relatedInformation"])
}

func TestJSONRPCMessage_Request(t *testing.T) {
	id := int64(1)
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "textDocument/hover",
		Params:  json.RawMessage(`{"position":{"line":0,"character":0}}`),
	}

	assert.True(t, msg.IsRequest())
	assert.False(t, msg.IsResponse())
	assert.False(t, msg.IsNotification())

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "2.0", parsed["jsonrpc"])
	assert.Equal(t, float64(1), parsed["id"])
	assert.Equal(t, "textDocument/hover", parsed["method"])
}

func TestJSONRPCMessage_Notification(t *testing.T) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  json.RawMessage(`{}`),
	}

	assert.True(t, msg.IsNotification())
	assert.False(t, msg.IsRequest())
	assert.False(t, msg.IsResponse())

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "2.0", parsed["jsonrpc"])
	assert.Nil(t, parsed["id"])
	assert.Equal(t, "initialized", parsed["method"])
}

func TestJSONRPCMessage_Response(t *testing.T) {
	id := int64(5)
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  json.RawMessage(`{"capabilities":{}}`),
	}

	assert.True(t, msg.IsResponse())
	assert.False(t, msg.IsRequest())
	assert.False(t, msg.IsNotification())
}

func TestJSONRPCError(t *testing.T) {
	e := &JSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
	}
	assert.Equal(t, "Invalid Request", e.Error())
}

func TestDiagnostic_JSON(t *testing.T) {
	diag := Diagnostic{
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
		Severity: SeverityError,
		Message:  "undefined variable",
		Source:   "gopls",
	}

	data, err := json.Marshal(diag)
	require.NoError(t, err)

	var parsed Diagnostic
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, 10, parsed.Range.Start.Line)
	assert.Equal(t, 5, parsed.Range.Start.Character)
	assert.Equal(t, SeverityError, parsed.Severity)
	assert.Equal(t, "undefined variable", parsed.Message)
	assert.Equal(t, "gopls", parsed.Source)
}

func TestTextDocumentItem_JSON(t *testing.T) {
	item := TextDocumentItem{
		URI:        "file:///workspace/main.go",
		LanguageID: "go",
		Version:    1,
		Text:       "package main\n",
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var parsed TextDocumentItem
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "file:///workspace/main.go", parsed.URI)
	assert.Equal(t, "go", parsed.LanguageID)
}

func TestManagerCount(t *testing.T) {
	m := NewManager()
	assert.Equal(t, 0, m.Count())
}

func TestManagerCloseAll_Empty(t *testing.T) {
	m := NewManager()
	// Should not panic on empty
	m.CloseAll()
	assert.Equal(t, 0, m.Count())
}

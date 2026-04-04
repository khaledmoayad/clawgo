package lspclient

import "encoding/json"

// JSON-RPC 2.0 message types for LSP communication.

// JSONRPCMessage represents a JSON-RPC 2.0 message (request, response, or notification).
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *JSONRPCError) Error() string {
	return e.Message
}

// IsRequest returns true if the message is a request (has ID and Method).
func (m *JSONRPCMessage) IsRequest() bool {
	return m.ID != nil && m.Method != ""
}

// IsResponse returns true if the message is a response (has ID, no Method).
func (m *JSONRPCMessage) IsResponse() bool {
	return m.ID != nil && m.Method == ""
}

// IsNotification returns true if the message is a notification (no ID, has Method).
func (m *JSONRPCMessage) IsNotification() bool {
	return m.ID == nil && m.Method != ""
}

// LSP protocol types for the initialize handshake and diagnostics.

// InitializeParams is sent from client to server in the initialize request.
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities defines the capabilities the client supports.
type ClientCapabilities struct {
	TextDocument *TextDocClientCap `json:"textDocument,omitempty"`
}

// TextDocClientCap defines text document capabilities.
type TextDocClientCap struct {
	PublishDiagnostics *PublishDiagCap `json:"publishDiagnostics,omitempty"`
}

// PublishDiagCap defines publish diagnostics capabilities.
type PublishDiagCap struct {
	RelatedInformation bool `json:"relatedInformation"`
}

// InitializeResult is the response from the server to an initialize request.
type InitializeResult struct {
	Capabilities json.RawMessage `json:"capabilities"`
}

// Diagnostic represents a diagnostic (error, warning, etc.) from the server.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

// Severity constants for diagnostics.
const (
	SeverityError       = 1
	SeverityWarning     = 2
	SeverityInformation = 3
	SeverityHint        = 4
)

// Range represents a text range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a text document (zero-indexed).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// TextDocumentItem represents a text document transferred between client and server.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// PublishDiagnosticsParams is sent from server to client for diagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Package output defines SDK-compatible message types for non-interactive
// output formats (json, stream-json). The types match Claude Code's SDK
// message schemas from entrypoints/sdk/coreSchemas.ts so that downstream
// consumers (editors, CI, SDK hosts) can parse ClawGo output identically.
package output

// MessageType identifies the kind of SDK message.
type MessageType string

const (
	TypeAssistant  MessageType = "assistant"
	TypeUser       MessageType = "user"
	TypeResult     MessageType = "result"
	TypeSystem     MessageType = "system"
	TypeToolUse    MessageType = "tool_use"
	TypeToolResult MessageType = "tool_result"
)

// ResultSubtype identifies the outcome of a non-interactive session.
type ResultSubtype string

const (
	SubtypeSuccess              ResultSubtype = "success"
	SubtypeError                ResultSubtype = "error_during_execution"
	SubtypeErrorMaxTurns        ResultSubtype = "error_max_turns"
	SubtypeErrorMaxBudget       ResultSubtype = "error_max_budget_usd"
)

// AssistantMessage is an assistant turn in stream-json output.
// Matches SDKAssistantMessageSchema.
type AssistantMessage struct {
	Type      MessageType    `json:"type"`
	Message   ContentMessage `json:"message"`
	SessionID string         `json:"session_id"`
}

// ContentMessage is a simplified API message with content blocks.
type ContentMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model,omitempty"`
	Usage   *UsageInfo     `json:"usage,omitempty"`
}

// ContentBlock represents a single content block within a message.
type ContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

// UsageInfo mirrors API usage response fields.
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ResultMessage is the final message in json and stream-json output.
// Matches SDKResultSuccessSchema / SDKResultErrorSchema.
type ResultMessage struct {
	Type         MessageType   `json:"type"`               // always "result"
	Subtype      ResultSubtype `json:"subtype"`
	Result       string        `json:"result,omitempty"`    // final text (success)
	Errors       []string      `json:"errors,omitempty"`    // accumulated errors (error subtypes)
	SessionID    string        `json:"session_id"`
	DurationMS   int64         `json:"duration_ms"`
	DurationAPIMS int64        `json:"duration_api_ms"`
	IsError      bool          `json:"is_error"`
	NumTurns     int           `json:"num_turns"`
	TotalCostUSD float64       `json:"total_cost_usd"`
	Usage        *UsageInfo    `json:"usage"`
	StopReason   *string       `json:"stop_reason"`
}

// ToolUseMessage represents a tool invocation in stream-json output.
type ToolUseMessage struct {
	Type      MessageType `json:"type"` // "tool_use"
	Name      string      `json:"name"`
	ID        string      `json:"id"`
	Input     any         `json:"input,omitempty"`
	SessionID string      `json:"session_id"`
}

// ToolResultMsg represents a tool result in stream-json output.
type ToolResultMsg struct {
	Type      MessageType `json:"type"` // "tool_result"
	ToolUseID string      `json:"tool_use_id"`
	Content   string      `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
	SessionID string      `json:"session_id"`
}

// SystemMessage is a status/system message in stream-json output.
type SystemMessage struct {
	Type      MessageType `json:"type"`    // "system"
	Subtype   string      `json:"subtype"` // e.g., "init", "status"
	Message   string      `json:"message,omitempty"`
	SessionID string      `json:"session_id"`
}

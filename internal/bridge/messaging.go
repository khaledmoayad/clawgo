// Package bridge - messaging.go implements the bridge message protocol.
//
// Pure functions for classifying, filtering, and routing messages between the
// bridge transport and the local REPL. Matches the TypeScript bridgeMessaging.ts.
package bridge

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// SDKMessage represents a message in the bridge protocol.
// It has a Type field and optional UUID for deduplication.
type SDKMessage struct {
	Type    string          `json:"type"`
	UUID    string          `json:"uuid,omitempty"`
	Subtype string          `json:"subtype,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	// Raw holds the original JSON for forwarding without re-serialization.
	Raw json.RawMessage `json:"-"`
}

// SDKControlResponse represents a control_response from the bridge.
type SDKControlResponse struct {
	Type      string           `json:"type"` // "control_response"
	Response  *ControlResponse `json:"response"`
	SessionID string           `json:"session_id,omitempty"`
}

// ControlResponse is the inner response payload of a control message.
type ControlResponse struct {
	Subtype   string          `json:"subtype"` // "success" or "error"
	RequestID string          `json:"request_id"`
	Response  json.RawMessage `json:"response,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// SDKControlRequest represents a control_request from the server.
type SDKControlRequest struct {
	Type      string          `json:"type"` // "control_request"
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
}

// ControlRequestPayload is the decoded inner request of a control_request.
type ControlRequestPayload struct {
	Subtype           string  `json:"subtype"`
	Model             string  `json:"model,omitempty"`
	MaxThinkingTokens *int    `json:"max_thinking_tokens,omitempty"`
	PermissionMode    string  `json:"permission_mode,omitempty"`
	Text              string  `json:"text,omitempty"`
}

// SDKResultSuccess is a minimal result message for session archival.
type SDKResultSuccess struct {
	Type              string         `json:"type"`    // "result"
	Subtype           string         `json:"subtype"` // "success"
	DurationMs        int            `json:"duration_ms"`
	DurationAPIMs     int            `json:"duration_api_ms"`
	IsError           bool           `json:"is_error"`
	NumTurns          int            `json:"num_turns"`
	Result            string         `json:"result"`
	StopReason        *string        `json:"stop_reason"`
	TotalCostUSD      float64        `json:"total_cost_usd"`
	Usage             map[string]int `json:"usage"`
	ModelUsage        map[string]any `json:"modelUsage"`
	PermissionDenials []string       `json:"permission_denials"`
	SessionID         string         `json:"session_id"`
	UUID              string         `json:"uuid"`
}

// IsSDKMessage checks if raw JSON represents an SDKMessage (non-null object with string "type").
func IsSDKMessage(data json.RawMessage) bool {
	if len(data) == 0 {
		return false
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Type != ""
}

// IsSDKControlResponse checks if raw JSON is a control_response message.
func IsSDKControlResponse(data json.RawMessage) bool {
	var probe struct {
		Type     string `json:"type"`
		Response *struct {
			Subtype string `json:"subtype"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Type == "control_response" && probe.Response != nil
}

// IsSDKControlRequest checks if raw JSON is a control_request message.
func IsSDKControlRequest(data json.RawMessage) bool {
	var probe struct {
		Type      string `json:"type"`
		RequestID string `json:"request_id"`
		Request   *json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Type == "control_request" && probe.RequestID != "" && probe.Request != nil
}

// ParseSDKControlRequest parses raw JSON into an SDKControlRequest.
func ParseSDKControlRequest(data json.RawMessage) (*SDKControlRequest, error) {
	var req SDKControlRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ParseSDKControlResponse parses raw JSON into an SDKControlResponse.
func ParseSDKControlResponse(data json.RawMessage) (*SDKControlResponse, error) {
	var resp SDKControlResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ParseSDKMessage parses raw JSON into an SDKMessage, preserving the Raw field.
func ParseSDKMessage(data json.RawMessage) (*SDKMessage, error) {
	var msg SDKMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	msg.Raw = data
	return &msg, nil
}

// IsEligibleBridgeMessage returns true if a message should be forwarded to the
// bridge transport. Eligible messages are:
//   - user messages (non-virtual)
//   - assistant messages (non-virtual)
//   - system messages with subtype "local_command"
func IsEligibleBridgeMessage(m *api.Message) bool {
	if m == nil {
		return false
	}
	switch m.Role {
	case "user":
		return true
	case "assistant":
		return true
	}
	return false
}

// ExtractTitleText extracts title-worthy text from a user message.
// Returns empty string if the message is not suitable for titling.
func ExtractTitleText(m *api.Message) string {
	if m == nil || m.Role != "user" {
		return ""
	}
	// Extract text content from the message
	for _, block := range m.Content {
		if block.Type == "text" && block.Text != "" {
			text := strings.TrimSpace(block.Text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

// IngressCallbacks holds the callback functions used by HandleIngressMessage.
type IngressCallbacks struct {
	OnInboundMessage    func(msg *SDKMessage)
	OnPermissionResponse func(resp *SDKControlResponse)
	OnControlRequest    func(req *SDKControlRequest)
}

// HandleIngressMessage parses and routes an ingress WebSocket message.
// It handles deduplication using the provided UUID sets and dispatches to
// the appropriate callback based on message type.
func HandleIngressMessage(
	data json.RawMessage,
	recentPostedUUIDs *BoundedUUIDSet,
	recentInboundUUIDs *BoundedUUIDSet,
	callbacks IngressCallbacks,
) {
	if len(data) == 0 {
		return
	}

	// Check for control_response first
	if IsSDKControlResponse(data) {
		if callbacks.OnPermissionResponse != nil {
			resp, err := ParseSDKControlResponse(data)
			if err != nil {
				log.Printf("bridge:messaging: failed to parse control_response: %v", err)
				return
			}
			callbacks.OnPermissionResponse(resp)
		}
		return
	}

	// Check for control_request
	if IsSDKControlRequest(data) {
		if callbacks.OnControlRequest != nil {
			req, err := ParseSDKControlRequest(data)
			if err != nil {
				log.Printf("bridge:messaging: failed to parse control_request: %v", err)
				return
			}
			callbacks.OnControlRequest(req)
		}
		return
	}

	// Validate it's an SDK message
	if !IsSDKMessage(data) {
		return
	}

	msg, err := ParseSDKMessage(data)
	if err != nil {
		log.Printf("bridge:messaging: failed to parse SDK message: %v", err)
		return
	}

	// Echo dedup: skip if we sent this message (reflected back by the server)
	if msg.UUID != "" && recentPostedUUIDs.Has(msg.UUID) {
		return
	}

	// Re-delivery dedup: skip if we already received this message
	if msg.UUID != "" {
		if recentInboundUUIDs.Has(msg.UUID) {
			return
		}
		recentInboundUUIDs.Add(msg.UUID)
	}

	// Only forward user messages to the inbound handler
	if msg.Type == "user" {
		if callbacks.OnInboundMessage != nil {
			callbacks.OnInboundMessage(msg)
		}
	}
}

// MakeResultMessage creates a minimal result message for session archival.
func MakeResultMessage(sessionID, uuid string) *SDKResultSuccess {
	return &SDKResultSuccess{
		Type:              "result",
		Subtype:           "success",
		DurationMs:        0,
		DurationAPIMs:     0,
		IsError:           false,
		NumTurns:          0,
		Result:            "",
		StopReason:        nil,
		TotalCostUSD:      0,
		Usage:             map[string]int{"input_tokens": 0, "output_tokens": 0},
		ModelUsage:        map[string]any{},
		PermissionDenials: []string{},
		SessionID:         sessionID,
		UUID:              uuid,
	}
}

// MakeControlResponse creates a success control response.
func MakeControlResponse(requestID, sessionID string, response any) *SDKControlResponse {
	respData, _ := json.Marshal(response)
	return &SDKControlResponse{
		Type: "control_response",
		Response: &ControlResponse{
			Subtype:   "success",
			RequestID: requestID,
			Response:  respData,
		},
		SessionID: sessionID,
	}
}

// MakeControlErrorResponse creates an error control response.
func MakeControlErrorResponse(requestID, sessionID, errMsg string) *SDKControlResponse {
	return &SDKControlResponse{
		Type: "control_response",
		Response: &ControlResponse{
			Subtype:   "error",
			RequestID: requestID,
			Error:     errMsg,
		},
		SessionID: sessionID,
	}
}

// BoundedUUIDSet is a FIFO-bounded ring buffer for UUID deduplication.
// It provides O(1) lookup and O(capacity) memory usage.
type BoundedUUIDSet struct {
	mu       sync.Mutex
	capacity int
	items    []string
	lookup   map[string]struct{}
	pos      int // next write position in the ring buffer
	count    int // total items currently stored
}

// NewBoundedUUIDSet creates a new BoundedUUIDSet with the given capacity.
func NewBoundedUUIDSet(capacity int) *BoundedUUIDSet {
	if capacity <= 0 {
		capacity = 1000
	}
	return &BoundedUUIDSet{
		capacity: capacity,
		items:    make([]string, capacity),
		lookup:   make(map[string]struct{}, capacity),
	}
}

// Add inserts a UUID into the set. If the set is at capacity, the oldest
// entry is evicted.
func (s *BoundedUUIDSet) Add(uuid string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.lookup[uuid]; ok {
		return // already present
	}

	// Evict the oldest entry if at capacity
	if s.count >= s.capacity {
		old := s.items[s.pos]
		if old != "" {
			delete(s.lookup, old)
		}
	} else {
		s.count++
	}

	s.items[s.pos] = uuid
	s.lookup[uuid] = struct{}{}
	s.pos = (s.pos + 1) % s.capacity
}

// Has returns true if the UUID is in the set.
func (s *BoundedUUIDSet) Has(uuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.lookup[uuid]
	return ok
}

// Clear removes all entries from the set.
func (s *BoundedUUIDSet) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookup = make(map[string]struct{}, s.capacity)
	s.items = make([]string, s.capacity)
	s.pos = 0
	s.count = 0
}

// Size returns the number of UUIDs currently in the set.
func (s *BoundedUUIDSet) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

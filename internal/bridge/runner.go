// Package bridge - runner.go implements the bridge session runner.
//
// The session runner is the glue between a work item (received from polling)
// and the local query engine. It:
//  1. Decodes the work secret from the work response
//  2. Acknowledges the work item
//  3. Establishes a WebSocket transport to the session ingress
//  4. Handles incoming messages (user prompts from claude.ai)
//  5. Runs the agentic query loop via the SDK QueryEngine
//  6. Relays streaming responses back through the transport
//  7. Sends heartbeats to extend the work lease
//  8. Cleans up on completion or cancellation
//
// This replaces the placeholder handleWork in bridge.go.
package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/sdk"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const (
	// heartbeatInterval is how often we send heartbeats for active work items.
	heartbeatInterval = 30 * time.Second

	// uuidSetCapacity is the max size of the dedup UUID ring buffers.
	uuidSetCapacity = 5000

	// sessionActivityRingSize is the max activities to track per session.
	sessionActivityRingSize = 10
)

// SessionRunnerConfig configures a bridge session runner.
type SessionRunnerConfig struct {
	// EnvironmentID is the registered bridge environment ID.
	EnvironmentID string

	// APIClient is the bridge API client for heartbeats and acknowledgements.
	APIClient *APIClient

	// ToolRegistry provides tools available in the session.
	ToolRegistry *tools.Registry

	// SystemPrompt is the system prompt for the query engine.
	SystemPrompt string

	// WorkingDir is the working directory for tool execution.
	WorkingDir string

	// ProjectRoot is the project root directory.
	ProjectRoot string

	// CreateAPIClient creates a new API client for the session.
	// This allows the session to use the work secret's API base URL and token.
	CreateAPIClient func(baseURL, token string) *api.Client

	// OnSessionStart is called when a session starts processing.
	OnSessionStart func(sessionID, prompt string)

	// OnSessionComplete is called when a session completes.
	OnSessionComplete func(sessionID string, duration time.Duration, status SessionDoneStatus)

	// OnActivity is called when a session activity occurs (for status display).
	OnActivity func(sessionID string, activity SessionActivity)

	// OnDebug is an optional debug logging callback.
	OnDebug func(string)
}

// SessionRunner manages a single bridge session lifecycle.
type SessionRunner struct {
	config       SessionRunnerConfig
	workResponse *WorkResponse
	workSecret   *WorkSecret
	transport    Transport

	mu         sync.Mutex
	activities []SessionActivity

	recentPostedUUIDs  *BoundedUUIDSet
	recentInboundUUIDs *BoundedUUIDSet
}

// NewSessionRunner creates a session runner for the given work response.
func NewSessionRunner(cfg SessionRunnerConfig, workResp *WorkResponse) *SessionRunner {
	return &SessionRunner{
		config:             cfg,
		workResponse:       workResp,
		activities:         make([]SessionActivity, 0, sessionActivityRingSize),
		recentPostedUUIDs:  NewBoundedUUIDSet(uuidSetCapacity),
		recentInboundUUIDs: NewBoundedUUIDSet(uuidSetCapacity),
	}
}

func (r *SessionRunner) debug(msg string) {
	if r.config.OnDebug != nil {
		r.config.OnDebug(msg)
	}
}

// Run executes the session lifecycle: decode secret, ack, connect transport,
// process messages, and clean up. Blocks until the context is cancelled or
// the session completes.
func (r *SessionRunner) Run(ctx context.Context) error {
	sessionID := r.workResponse.Data.ID
	workID := r.workResponse.ID

	// 1. Decode the work secret
	secret, err := DecodeWorkSecret(r.workResponse.Secret)
	if err != nil {
		return fmt.Errorf("session %s: decode work secret: %w", sessionID, err)
	}
	r.workSecret = secret

	r.debug(fmt.Sprintf("[runner:%s] decoded work secret, api_base=%s", sessionID, secret.APIBaseURL))

	// 2. Acknowledge the work item
	if err := r.config.APIClient.AcknowledgeWork(ctx, r.config.EnvironmentID, workID, secret.SessionIngressToken); err != nil {
		return fmt.Errorf("session %s: acknowledge work: %w", sessionID, err)
	}

	r.debug(fmt.Sprintf("[runner:%s] acknowledged work %s", sessionID, workID))

	// 3. Build and connect the transport
	wsURL := BuildSDKURL(secret.APIBaseURL, sessionID)
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+secret.SessionIngressToken)
	headers.Set("anthropic-version", "2023-06-01")

	transport := NewWSTransport(TransportConfig{
		URL:       wsURL,
		Headers:   headers,
		SessionID: sessionID,
		GetToken: func() string {
			return secret.SessionIngressToken
		},
		OnDebug: r.config.OnDebug,
	})
	r.transport = transport

	// Set up message handler
	msgCh := make(chan *SDKMessage, 64)
	transport.SetOnData(func(data json.RawMessage) {
		HandleIngressMessage(data, r.recentPostedUUIDs, r.recentInboundUUIDs, IngressCallbacks{
			OnInboundMessage: func(msg *SDKMessage) {
				select {
				case msgCh <- msg:
				default:
					log.Printf("bridge:runner:%s: message channel full, dropping message", sessionID)
				}
			},
			OnControlRequest: func(req *SDKControlRequest) {
				r.handleControlRequest(ctx, req, sessionID)
			},
		})
	})

	transport.SetOnClose(func(code int) {
		r.debug(fmt.Sprintf("[runner:%s] transport closed with code %d", sessionID, code))
	})

	if err := transport.Connect(ctx); err != nil {
		return fmt.Errorf("session %s: transport connect: %w", sessionID, err)
	}
	defer transport.Close()

	// Notify session start
	if r.config.OnSessionStart != nil {
		r.config.OnSessionStart(sessionID, "")
	}

	startTime := time.Now()

	// 4. Start heartbeat loop
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go r.heartbeatLoop(heartbeatCtx, workID, secret.SessionIngressToken)

	// 5. Process messages in a loop
	status := r.processMessages(ctx, sessionID, msgCh)

	// 6. Clean up
	elapsed := time.Since(startTime)
	if r.config.OnSessionComplete != nil {
		r.config.OnSessionComplete(sessionID, elapsed, status)
	}

	r.debug(fmt.Sprintf("[runner:%s] session complete in %s, status=%s", sessionID, elapsed, status))

	return nil
}

// processMessages handles incoming user messages by running the query engine.
func (r *SessionRunner) processMessages(ctx context.Context, sessionID string, msgCh <-chan *SDKMessage) SessionDoneStatus {
	for {
		select {
		case <-ctx.Done():
			return SessionDoneStatusInterrupted

		case msg, ok := <-msgCh:
			if !ok {
				return SessionDoneStatusCompleted
			}

			if msg.Type != "user" {
				continue
			}

			// Extract the user's text from the message content
			userText := extractUserText(msg)
			if userText == "" {
				continue
			}

			r.recordActivity(SessionActivity{
				Type:      SessionActivityText,
				Summary:   truncate(userText, 100),
				Timestamp: time.Now().Unix(),
			})

			// Run the query engine for this user message
			if err := r.runQueryTurn(ctx, sessionID, userText); err != nil {
				r.debug(fmt.Sprintf("[runner:%s] query error: %v", sessionID, err))
				r.recordActivity(SessionActivity{
					Type:      SessionActivityError,
					Summary:   err.Error(),
					Timestamp: time.Now().Unix(),
				})
				// Send error through transport
				r.sendError(ctx, sessionID, err)
				return SessionDoneStatusFailed
			}
		}
	}
}

// runQueryTurn runs a single conversation turn through the query engine and
// relays streaming events back through the bridge transport.
func (r *SessionRunner) runQueryTurn(ctx context.Context, sessionID, userText string) error {
	if r.config.CreateAPIClient == nil {
		return fmt.Errorf("no API client factory configured")
	}

	// Create API client using the work secret's base URL and auth
	authToken := ""
	if r.workSecret != nil && len(r.workSecret.Auth) > 0 {
		authToken = r.workSecret.Auth[0].Token
	}

	apiClient := r.config.CreateAPIClient(r.workSecret.APIBaseURL, authToken)

	engine := sdk.NewQueryEngine(sdk.QueryEngineConfig{
		Client:       apiClient,
		Registry:     r.config.ToolRegistry,
		SystemPrompt: r.config.SystemPrompt,
		WorkingDir:   r.config.WorkingDir,
		ProjectRoot:  r.config.ProjectRoot,
	})

	// Run the query and relay events through the transport
	events := engine.Ask(ctx, userText)

	for evt := range events {
		switch evt.Type {
		case sdk.EventTextDelta:
			r.relayStreamEvent(ctx, sessionID, "content_block_delta", map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type": "text_delta",
					"text": evt.Text,
				},
			})

		case sdk.EventToolUseStart:
			r.recordActivity(SessionActivity{
				Type:      SessionActivityToolStart,
				Summary:   evt.ToolName,
				Timestamp: time.Now().Unix(),
			})
			r.relayStreamEvent(ctx, sessionID, "content_block_start", map[string]any{
				"type": "content_block_start",
				"content_block": map[string]any{
					"type": "tool_use",
					"id":   evt.ToolID,
					"name": evt.ToolName,
				},
			})

		case sdk.EventToolResult:
			r.recordActivity(SessionActivity{
				Type:      SessionActivityResult,
				Summary:   truncate(evt.ToolResult, 100),
				Timestamp: time.Now().Unix(),
			})

		case sdk.EventTurnComplete:
			r.relayStreamEvent(ctx, sessionID, "message_stop", map[string]any{
				"type": "message_stop",
			})

		case sdk.EventError:
			if evt.Error != nil {
				return evt.Error
			}
		}
	}

	return nil
}

// relayStreamEvent sends a stream event through the bridge transport.
func (r *SessionRunner) relayStreamEvent(ctx context.Context, sessionID, eventType string, data any) {
	envelope := map[string]any{
		"type":       "stream_event",
		"session_id": sessionID,
		"event":      data,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		r.debug(fmt.Sprintf("[runner:%s] failed to marshal stream event: %v", sessionID, err))
		return
	}

	// Track for echo dedup
	if err := r.transport.Write(payload); err != nil {
		r.debug(fmt.Sprintf("[runner:%s] failed to write stream event: %v", sessionID, err))
	}
}

// sendError sends an error event through the bridge transport.
func (r *SessionRunner) sendError(ctx context.Context, sessionID string, err error) {
	envelope := map[string]any{
		"type":       "error",
		"session_id": sessionID,
		"error":      err.Error(),
	}

	payload, errMarshal := json.Marshal(envelope)
	if errMarshal != nil {
		return
	}

	r.transport.Write(payload)
}

// handleControlRequest processes a server control request and responds.
func (r *SessionRunner) handleControlRequest(ctx context.Context, req *SDKControlRequest, sessionID string) {
	var payload ControlRequestPayload
	if err := json.Unmarshal(req.Request, &payload); err != nil {
		r.sendControlResponse(ctx, sessionID, MakeControlErrorResponse(
			req.RequestID, sessionID,
			fmt.Sprintf("failed to parse control request: %v", err),
		))
		return
	}

	switch payload.Subtype {
	case "initialize":
		r.sendControlResponse(ctx, sessionID, MakeControlResponse(
			req.RequestID, sessionID,
			map[string]any{
				"commands":                 []string{},
				"output_style":            "normal",
				"available_output_styles": []string{"normal"},
				"models":                  []string{},
				"account":                 map[string]any{},
			},
		))

	case "interrupt":
		// For now, log and respond success. Full implementation would signal
		// the query engine to abort the current turn.
		r.debug(fmt.Sprintf("[runner:%s] received interrupt control request", sessionID))
		r.sendControlResponse(ctx, sessionID, MakeControlResponse(
			req.RequestID, sessionID, map[string]any{},
		))

	default:
		r.sendControlResponse(ctx, sessionID, MakeControlErrorResponse(
			req.RequestID, sessionID,
			fmt.Sprintf("REPL bridge does not handle control_request subtype: %s", payload.Subtype),
		))
	}
}

// sendControlResponse sends a control response through the transport.
func (r *SessionRunner) sendControlResponse(ctx context.Context, sessionID string, resp *SDKControlResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		r.debug(fmt.Sprintf("[runner:%s] failed to marshal control response: %v", sessionID, err))
		return
	}
	r.transport.Write(data)
}

// heartbeatLoop sends periodic heartbeats to extend the work lease.
func (r *SessionRunner) heartbeatLoop(ctx context.Context, workID, sessionToken string) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaseExtended, state, err := r.config.APIClient.HeartbeatWork(
				ctx, r.config.EnvironmentID, workID, sessionToken,
			)
			if err != nil {
				r.debug(fmt.Sprintf("[runner:heartbeat] error for work %s: %v", workID, err))
				continue
			}
			r.debug(fmt.Sprintf("[runner:heartbeat] work %s: lease=%v state=%s", workID, leaseExtended, state))
		}
	}
}

// recordActivity adds a session activity event, keeping only the last N.
func (r *SessionRunner) recordActivity(activity SessionActivity) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.activities = append(r.activities, activity)
	if len(r.activities) > sessionActivityRingSize {
		r.activities = r.activities[len(r.activities)-sessionActivityRingSize:]
	}

	if r.config.OnActivity != nil {
		r.config.OnActivity(r.workResponse.Data.ID, activity)
	}
}

// Activities returns a copy of the current activity ring buffer.
func (r *SessionRunner) Activities() []SessionActivity {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]SessionActivity, len(r.activities))
	copy(result, r.activities)
	return result
}

// extractUserText extracts the text content from an SDK message.
func extractUserText(msg *SDKMessage) string {
	if msg == nil {
		return ""
	}

	// Try to extract from content field
	if len(msg.Content) > 0 {
		var content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(msg.Content, &content); err == nil {
			for _, block := range content {
				if block.Type == "text" && block.Text != "" {
					return block.Text
				}
			}
		}

		// Maybe content is a raw string
		var text string
		if err := json.Unmarshal(msg.Content, &text); err == nil && text != "" {
			return text
		}
	}

	// Try the raw message for a text field
	if len(msg.Raw) > 0 {
		var raw struct {
			Text    string `json:"text"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(msg.Raw, &raw); err == nil {
			if raw.Text != "" {
				return raw.Text
			}
			if raw.Message != "" {
				return raw.Message
			}
		}
	}

	return ""
}

// truncate shortens a string to the given max length, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}


// DecodeWorkSecret decodes a base64url-encoded work secret from the bridge API.
func DecodeWorkSecret(encoded string) (*WorkSecret, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode work secret: %w", err)
		}
	}
	var secret WorkSecret
	if err := json.Unmarshal(data, &secret); err != nil {
		return nil, fmt.Errorf("unmarshal work secret: %w", err)
	}
	return &secret, nil
}

// BuildSDKURL constructs the WebSocket URL for session ingress.
func BuildSDKURL(apiBaseURL, sessionID string) string {
	return fmt.Sprintf("%s/v1/code/sessions/%s", apiBaseURL, sessionID)
}

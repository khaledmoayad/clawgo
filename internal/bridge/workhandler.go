package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

const (
	// heartbeatInterval is the interval between heartbeat requests for active sessions.
	heartbeatInterval = 30 * time.Second
)

// WorkHandler processes incoming work items from the bridge API.
type WorkHandler struct {
	api     *APIClient
	envID   string
	config  BridgeConfig
	onDebug func(string)
}

// NewWorkHandler creates a new WorkHandler.
func NewWorkHandler(api *APIClient, envID string, config BridgeConfig) *WorkHandler {
	return &WorkHandler{
		api:     api,
		envID:   envID,
		config:  config,
		onDebug: config.OnDebug,
	}
}

func (h *WorkHandler) debug(msg string) {
	if h.onDebug != nil {
		h.onDebug(msg)
	}
}

// DecodeWorkSecret decodes a base64url-encoded work secret and validates its version.
func DecodeWorkSecret(secretB64 string) (*WorkSecret, error) {
	// base64url decode the secret string
	decoded, err := base64.RawURLEncoding.DecodeString(secretB64)
	if err != nil {
		// Try with padding
		decoded, err = base64.URLEncoding.DecodeString(secretB64)
		if err != nil {
			return nil, fmt.Errorf("base64url decode work secret: %w", err)
		}
	}

	var secret WorkSecret
	if err := json.Unmarshal(decoded, &secret); err != nil {
		return nil, fmt.Errorf("json unmarshal work secret: %w", err)
	}

	// Validate version
	if secret.Version != 1 {
		return nil, fmt.Errorf("unsupported work secret version: %d", secret.Version)
	}

	// Validate required fields
	if secret.SessionIngressToken == "" {
		return nil, fmt.Errorf("invalid work secret: missing or empty session_ingress_token")
	}
	if secret.APIBaseURL == "" {
		return nil, fmt.Errorf("invalid work secret: missing api_base_url")
	}

	return &secret, nil
}

// HandleWork processes a single WorkResponse. For healthcheck items, it returns
// immediately. For session items, it decodes the work secret, acknowledges the
// work, and spawns a session with heartbeat management.
func (h *WorkHandler) HandleWork(ctx context.Context, work *WorkResponse, pool *SessionPool) error {
	// Healthcheck items don't need a session
	if work.Data.Type == "healthcheck" {
		h.debug(fmt.Sprintf("[bridge:work] healthcheck received workId=%s", work.ID))
		log.Printf("bridge: healthcheck work item %s", work.ID)
		return nil
	}

	// Decode the work secret
	secret, err := DecodeWorkSecret(work.Secret)
	if err != nil {
		return fmt.Errorf("decode work secret for %s: %w", work.ID, err)
	}

	sessionID := work.Data.ID
	sessionToken := secret.SessionIngressToken

	h.debug(fmt.Sprintf("[bridge:work] processing workId=%s sessionId=%s", work.ID, sessionID))

	// Acknowledge the work to confirm receipt
	if err := h.api.AcknowledgeWork(ctx, h.envID, work.ID, sessionToken); err != nil {
		return fmt.Errorf("acknowledge work %s: %w", work.ID, err)
	}

	// Spawn a session in the pool keyed by work.ID
	_, err = pool.Spawn(ctx, work, func(sessionCtx context.Context, w *WorkResponse) {
		h.runSession(sessionCtx, w, sessionToken, sessionID)
	})
	if err != nil {
		return fmt.Errorf("spawn session for work %s: %w", work.ID, err)
	}

	log.Printf("bridge: spawned session for workId=%s sessionId=%s", work.ID, sessionID)
	return nil
}

// runSession runs the heartbeat loop for a session and handles cleanup.
func (h *WorkHandler) runSession(ctx context.Context, work *WorkResponse, sessionToken, sessionID string) {
	startTime := time.Now()
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	// Start the actual query work (placeholder -- real wiring is integration)
	workDone := make(chan SessionDoneStatus, 1)
	go func() {
		// TODO: Wire to query loop in future integration
		<-sessionCtx.Done()
		workDone <- SessionDoneStatusInterrupted
	}()

	for {
		select {
		case <-sessionCtx.Done():
			h.HandleSessionDone(work, sessionID, SessionDoneStatusInterrupted, startTime)
			return
		case status := <-workDone:
			h.HandleSessionDone(work, sessionID, status, startTime)
			return
		case <-heartbeatTicker.C:
			leaseExtended, state, err := h.api.HeartbeatWork(sessionCtx, h.envID, work.ID, sessionToken)
			if err != nil {
				log.Printf("bridge: heartbeat error for workId=%s: %v", work.ID, err)
				sessionCancel()
			} else if !leaseExtended {
				log.Printf("bridge: lease not extended for workId=%s, stopping session", work.ID)
				sessionCancel()
			} else if state == "stopping" {
				log.Printf("bridge: work %s state=stopping, cancelling session", work.ID)
				sessionCancel()
			}
		}
	}
}

// HandleSessionDone archives the session and logs completion.
func (h *WorkHandler) HandleSessionDone(work *WorkResponse, sessionID string, status SessionDoneStatus, startTime time.Time) {
	duration := time.Since(startTime)

	// Archive the session (best-effort, use background context since session may be cancelled)
	archiveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.api.ArchiveSession(archiveCtx, sessionID); err != nil {
		log.Printf("bridge: failed to archive session %s: %v", sessionID, err)
	}

	log.Printf("bridge: session done workId=%s sessionId=%s status=%s duration=%s", work.ID, sessionID, status, duration.Round(time.Millisecond))
}

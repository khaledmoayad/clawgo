// Package bridge implements bridge/remote control mode for ClawGo.
// In bridge mode, the CLI registers as an environment, polls for work
// from claude.ai, spawns child sessions, and relays events via WebSocket.
package bridge

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/khaledmoayad/clawgo/internal/app"
)

// Bridge orchestrates the bridge/remote control mode lifecycle:
// environment registration, work polling, session spawning, and cleanup.
type Bridge struct {
	config Config
	api    *APIClient
	pool   *SessionPool
	envID  string
}

// NewBridge creates a new Bridge with the given configuration.
func NewBridge(cfg Config) *Bridge {
	cfg = cfg.withDefaults()
	return &Bridge{
		config: cfg,
		api:    NewAPIClient(cfg.APIBaseURL, cfg.GetToken),
		pool:   NewSessionPool(cfg.MaxConcurrentSessions),
	}
}

// Start registers the environment, starts the poll loop, and blocks
// until the context is cancelled or an unrecoverable error occurs.
func (b *Bridge) Start(ctx context.Context) error {
	// Register this machine as a bridge environment
	env, err := b.api.RegisterEnvironment(ctx, b.config.EnvironmentName)
	if err != nil {
		return fmt.Errorf("bridge register: %w", err)
	}
	b.envID = env.ID

	// Register cleanup so graceful shutdown stops sessions and reports offline
	app.RegisterCleanup(b.Stop)

	log.Printf("bridge: registered environment %q (id=%s)", env.Name, env.ID)

	// Start polling for work
	ticker := time.NewTicker(b.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := b.pollAndDispatch(ctx); err != nil {
				// Log poll errors but continue -- transient failures are expected
				log.Printf("bridge: poll error: %v", err)
			}
		}
	}
}

// Stop cancels all running sessions and reports offline status (best effort).
func (b *Bridge) Stop() {
	b.pool.StopAll()
	if b.envID != "" {
		// Best-effort status report -- use background context since main may be cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.api.ReportStatus(ctx, b.envID, "offline")
	}
}

// pollAndDispatch fetches work items and spawns sessions for each.
func (b *Bridge) pollAndDispatch(ctx context.Context) error {
	items, err := b.api.PollWork(ctx, b.envID)
	if err != nil {
		return err
	}

	for _, work := range items {
		if !b.pool.CanSpawn() {
			log.Printf("bridge: pool at capacity, skipping work item %s", work.SessionID)
			break
		}
		if _, err := b.pool.Spawn(ctx, work, b.handleWork); err != nil {
			log.Printf("bridge: failed to spawn session for %s: %v", work.SessionID, err)
		}
	}
	return nil
}

// handleWork processes a single work item in a child session.
// This is a placeholder -- full wiring with the query loop is integration work.
func (b *Bridge) handleWork(ctx context.Context, work WorkItem) {
	log.Printf("bridge: handling work item session=%s prompt=%q org=%s", work.SessionID, work.Prompt, work.OrgUUID)

	// In a complete implementation, this would:
	// 1. Create a new query session with the work prompt
	// 2. Connect a WebSocket for event relay
	// 3. Stream responses back to claude.ai
	// For now, we just wait for context cancellation.
	<-ctx.Done()
}

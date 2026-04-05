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
	config      BridgeConfig
	api         *APIClient
	pool        *SessionPool
	workHandler *WorkHandler
	envID       string
	envSecret   string
}

// NewBridge creates a new Bridge with the given configuration.
func NewBridge(cfg BridgeConfig) *Bridge {
	cfg = cfg.withDefaults()
	api := NewAPIClient(cfg.APIBaseURL, cfg.GetToken)
	if cfg.OnDebug != nil {
		api.SetDebugFunc(cfg.OnDebug)
	}
	return &Bridge{
		config: cfg,
		api:    api,
		pool:   NewSessionPool(cfg.MaxConcurrentSessions),
	}
}

// Start registers the environment, starts the poll loop, and blocks
// until the context is cancelled or an unrecoverable error occurs.
func (b *Bridge) Start(ctx context.Context) error {
	// Register this machine as a bridge environment
	envID, envSecret, err := b.api.RegisterBridgeEnvironment(ctx, b.config)
	if err != nil {
		return fmt.Errorf("bridge register: %w", err)
	}
	b.envID = envID
	b.envSecret = envSecret

	// Create the work handler now that we have an environment ID
	b.workHandler = NewWorkHandler(b.api, b.envID, b.config)

	// Register cleanup so graceful shutdown deregisters and stops sessions
	app.RegisterCleanup(b.Stop)

	log.Printf("bridge: registered environment %q (id=%s)", b.config.EnvironmentName, b.envID)

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

// Stop cancels all running sessions and deregisters the environment (best effort).
func (b *Bridge) Stop() {
	b.pool.StopAll()
	if b.envID != "" {
		// Best-effort deregister -- use background context since main may be cancelled
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := b.api.DeregisterEnvironment(ctx, b.envID); err != nil {
			log.Printf("bridge: deregister error: %v", err)
		}
	}
}

// pollAndDispatch fetches a work item and dispatches it to the work handler.
func (b *Bridge) pollAndDispatch(ctx context.Context) error {
	work, err := b.api.PollForWork(ctx, b.envID, b.envSecret, nil)
	if err != nil {
		return err
	}

	// No work available
	if work == nil {
		return nil
	}

	if !b.pool.CanSpawn() {
		log.Printf("bridge: pool at capacity, skipping work %s", work.ID)
		return nil
	}

	if err := b.workHandler.HandleWork(ctx, work, b.pool); err != nil {
		log.Printf("bridge: failed to handle work %s: %v", work.ID, err)
	}
	return nil
}

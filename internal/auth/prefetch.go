// Package auth provides authentication and credential management for ClawGo.
// This includes credential prefetching for cloud providers (Bedrock, Vertex)
// to reduce first-request latency.
package auth

import (
	"context"
	"sync"
	"time"
)

// prefetchTimeout is the maximum time allowed for each credential prefetch operation.
const prefetchTimeout = 10 * time.Second

// CredentialPrefetcher is the interface for provider-specific credential prefetching.
// Implementations should obtain and cache credentials so that the first API
// request doesn't incur the full authentication latency.
//
// BedrockPrefetcher and VertexPrefetcher will be added in Phase 5 (API-06, API-07).
type CredentialPrefetcher interface {
	// Prefetch obtains and caches credentials. Called at startup.
	Prefetch(ctx context.Context) error

	// Name returns a human-readable name for this prefetcher (e.g., "bedrock", "vertex").
	Name() string
}

// PrefetchCredentials runs all prefetchers concurrently, each with a 10-second
// timeout. Returns a slice of errors (nil entries for successful prefetches).
// This is called at startup to reduce first-request latency for cloud providers.
func PrefetchCredentials(ctx context.Context, prefetchers ...CredentialPrefetcher) []error {
	if len(prefetchers) == 0 {
		return nil
	}

	errs := make([]error, len(prefetchers))
	var wg sync.WaitGroup
	wg.Add(len(prefetchers))

	for i, p := range prefetchers {
		go func(idx int, pf CredentialPrefetcher) {
			defer wg.Done()
			timeoutCtx, cancel := context.WithTimeout(ctx, prefetchTimeout)
			defer cancel()
			errs[idx] = pf.Prefetch(timeoutCtx)
		}(i, p)
	}

	wg.Wait()
	return errs
}

// NoopPrefetcher is a no-op implementation for testing and for when no cloud
// providers are configured.
type NoopPrefetcher struct{}

// Prefetch is a no-op that always succeeds.
func (n *NoopPrefetcher) Prefetch(_ context.Context) error {
	return nil
}

// Name returns "noop".
func (n *NoopPrefetcher) Name() string {
	return "noop"
}

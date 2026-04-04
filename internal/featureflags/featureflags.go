// Package featureflags provides a GrowthBook-compatible feature flag client
// with local caching and periodic background refresh. Feature values are
// always read from cache (never blocking on network), enabling stale-but-fast
// reads matching the TypeScript getFeatureValue_CACHED_MAY_BE_STALE pattern.
package featureflags

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config controls the feature flag client behavior.
type Config struct {
	// ClientKey is the GrowthBook SDK key used to fetch feature definitions.
	ClientKey string

	// APIHost is the GrowthBook API host (default "https://cdn.growthbook.io").
	APIHost string

	// RefreshInterval is how often to re-fetch features in the background
	// (default 5 minutes).
	RefreshInterval time.Duration

	// Attributes are user/session attributes for targeting (id, sessionId, platform, etc.).
	Attributes map[string]any

	// Enabled is the master switch. When false, all methods return defaults
	// and no HTTP requests are made. Feature flags are opt-in.
	Enabled bool
}

// client is the package-level singleton feature flag client.
var (
	mu       sync.RWMutex
	features sync.Map // map[string]any of feature key -> value
	cfg      Config
	cancel   context.CancelFunc
	enabled  bool
)

// Init initializes the feature flag client. It fetches features from the
// GrowthBook API endpoint and starts a background refresh goroutine.
// If the initial fetch fails, it logs a warning and uses empty features
// (graceful degradation). If Config.Enabled is false, Init is a no-op.
func Init(c Config) error {
	mu.Lock()

	cfg = c

	// Apply defaults
	if cfg.APIHost == "" {
		cfg.APIHost = "https://cdn.growthbook.io"
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 5 * time.Minute
	}

	// Check master switch
	if !cfg.Enabled {
		enabled = false
		mu.Unlock()
		return nil
	}

	// Check CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC env var
	if isNonessentialTrafficDisabled() {
		enabled = false
		mu.Unlock()
		return nil
	}

	enabled = true

	// Snapshot config values before releasing lock for fetchFeatures
	host := cfg.APIHost
	key := cfg.ClientKey
	interval := cfg.RefreshInterval

	mu.Unlock()

	// Initial fetch (non-blocking on error -- graceful degradation).
	// Called outside mu.Lock to avoid deadlock since fetchFeatures
	// acquires mu.RLock internally.
	if err := fetchFeatures(host, key); err != nil {
		log.Printf("featureflags: initial fetch failed (using empty features): %v", err)
	}

	// Start background refresh
	mu.Lock()
	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	mu.Unlock()

	go refreshLoop(ctx, interval)

	return nil
}

// Close shuts down the background refresh goroutine.
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if cancel != nil {
		cancel()
		cancel = nil
	}
	enabled = false

	// Clear features
	features.Range(func(key, _ any) bool {
		features.Delete(key)
		return true
	})
}

// IsEnabled returns true if the feature exists and has a truthy value.
// Always reads from cache -- never blocks on network.
func IsEnabled(featureKey string) bool {
	if !isActive() {
		return false
	}

	val, ok := features.Load(featureKey)
	if !ok {
		return false
	}

	return isTruthy(val)
}

// IsEnabledCached is an alias for IsEnabled, explicitly named to match
// the TypeScript checkStatsigFeatureGate_CACHED_MAY_BE_STALE pattern.
// The "cached may be stale" semantics mean we never block on network.
func IsEnabledCached(featureKey string) bool {
	return IsEnabled(featureKey)
}

// GetValue returns the feature value for the given key, or defaultValue
// if the feature is not found or the client is disabled.
func GetValue(featureKey string, defaultValue any) any {
	if !isActive() {
		return defaultValue
	}

	val, ok := features.Load(featureKey)
	if !ok {
		return defaultValue
	}

	return val
}

// UpdateAttributes updates the targeting attributes and triggers a
// background refresh to re-evaluate targeting rules.
func UpdateAttributes(attrs map[string]any) {
	mu.Lock()
	cfg.Attributes = attrs
	active := enabled
	host := cfg.APIHost
	key := cfg.ClientKey
	mu.Unlock()

	if active {
		go func() {
			if err := fetchFeatures(host, key); err != nil {
				log.Printf("featureflags: refresh after attribute update failed: %v", err)
			}
		}()
	}
}

// isActive returns whether the client is enabled and should serve values.
func isActive() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

// isNonessentialTrafficDisabled checks the CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC env var.
func isNonessentialTrafficDisabled() bool {
	val := strings.ToLower(os.Getenv("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"))
	return val == "1" || val == "true"
}

// fetchFeatures retrieves feature definitions from the GrowthBook API
// and updates the local cache. The host and key parameters are passed
// explicitly to avoid acquiring mu.RLock (callers snapshot cfg while
// holding the lock, then call this function after releasing it).
func fetchFeatures(host, key string) error {
	if key == "" {
		return fmt.Errorf("no client key configured")
	}

	url := fmt.Sprintf("%s/api/features/%s", strings.TrimRight(host, "/"), key)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// GrowthBook API response format:
	// { "features": { "featureKey": { "defaultValue": ... }, ... } }
	var apiResp struct {
		Features map[string]struct {
			DefaultValue any `json:"defaultValue"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parsing response JSON: %w", err)
	}

	// Update the cache
	for key, feat := range apiResp.Features {
		features.Store(key, feat.DefaultValue)
	}

	return nil
}

// refreshLoop periodically fetches features until the context is cancelled.
func refreshLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Snapshot config under lock
			mu.RLock()
			host := cfg.APIHost
			key := cfg.ClientKey
			mu.RUnlock()

			if err := fetchFeatures(host, key); err != nil {
				log.Printf("featureflags: background refresh failed: %v", err)
			}
		}
	}
}

// isTruthy checks if a value is "truthy" in the JavaScript sense:
// non-nil, non-zero, non-empty-string, non-false.
func isTruthy(val any) bool {
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		return v != "" && v != "0" && v != "false"
	default:
		// For complex types (maps, slices), consider them truthy if non-nil
		return true
	}
}

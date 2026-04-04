package featureflags

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGrowthBookResponse returns a JSON response matching the GrowthBook API format.
func mockGrowthBookResponse(features map[string]any) []byte {
	type featureDef struct {
		DefaultValue any `json:"defaultValue"`
	}

	defs := make(map[string]featureDef)
	for k, v := range features {
		defs[k] = featureDef{DefaultValue: v}
	}

	resp := struct {
		Features map[string]featureDef `json:"features"`
	}{
		Features: defs,
	}

	data, _ := json.Marshal(resp)
	return data
}

func TestInit_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGrowthBookResponse(map[string]any{
			"dark_mode":    true,
			"beta_feature": false,
			"max_retries":  float64(3),
		}))
	}))
	defer server.Close()
	defer Close()

	err := Init(Config{
		ClientKey:       "sdk-test-key",
		APIHost:         server.URL,
		RefreshInterval: 1 * time.Hour, // long interval, we test manually
		Enabled:         true,
	})
	require.NoError(t, err)

	assert.True(t, IsEnabled("dark_mode"))
	assert.False(t, IsEnabled("beta_feature"))
	assert.False(t, IsEnabled("nonexistent"))
}

func TestIsEnabled_TruthyValues(t *testing.T) {
	defer Close()

	// Directly populate the features map for unit testing
	err := Init(Config{Enabled: false}) // init disabled, we set features manually
	require.NoError(t, err)

	// Override enabled state for testing
	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("bool_true", true)
	features.Store("bool_false", false)
	features.Store("number_nonzero", float64(42))
	features.Store("number_zero", float64(0))
	features.Store("string_nonempty", "hello")
	features.Store("string_empty", "")
	features.Store("string_false", "false")
	features.Store("nil_val", nil)

	assert.True(t, IsEnabled("bool_true"))
	assert.False(t, IsEnabled("bool_false"))
	assert.True(t, IsEnabled("number_nonzero"))
	assert.False(t, IsEnabled("number_zero"))
	assert.True(t, IsEnabled("string_nonempty"))
	assert.False(t, IsEnabled("string_empty"))
	assert.False(t, IsEnabled("string_false"))
	assert.False(t, IsEnabled("nil_val"))
	assert.False(t, IsEnabled("missing"))
}

func TestGetValue_ReturnsDefault(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("exists", "real-value")

	assert.Equal(t, "real-value", GetValue("exists", "default"))
	assert.Equal(t, "default", GetValue("missing", "default"))
	assert.Equal(t, 42, GetValue("also-missing", 42))
}

func TestDisabledMode_NoHTTP(t *testing.T) {
	httpCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		w.Write(mockGrowthBookResponse(map[string]any{"test": true}))
	}))
	defer server.Close()
	defer Close()

	err := Init(Config{
		ClientKey: "sdk-test-key",
		APIHost:   server.URL,
		Enabled:   false, // disabled
	})
	require.NoError(t, err)

	assert.False(t, httpCalled, "should not make HTTP calls when disabled")
	assert.False(t, IsEnabled("test"))
	assert.Equal(t, "fallback", GetValue("test", "fallback"))
}

func TestBackgroundRefresh(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount <= 1 {
			w.Write(mockGrowthBookResponse(map[string]any{"counter": float64(1)}))
		} else {
			w.Write(mockGrowthBookResponse(map[string]any{"counter": float64(2)}))
		}
	}))
	defer server.Close()
	defer Close()

	err := Init(Config{
		ClientKey:       "sdk-test-key",
		APIHost:         server.URL,
		RefreshInterval: 100 * time.Millisecond, // fast refresh for test
		Enabled:         true,
	})
	require.NoError(t, err)

	// Initial value
	val := GetValue("counter", float64(0))
	assert.Equal(t, float64(1), val)

	// Wait for background refresh
	time.Sleep(300 * time.Millisecond)

	// Should have updated
	val = GetValue("counter", float64(0))
	assert.Equal(t, float64(2), val)
	assert.GreaterOrEqual(t, callCount, 2)
}

func TestDisableNonessentialTraffic(t *testing.T) {
	httpCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		w.Write(mockGrowthBookResponse(map[string]any{"test": true}))
	}))
	defer server.Close()
	defer Close()

	t.Setenv("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "1")

	err := Init(Config{
		ClientKey: "sdk-test-key",
		APIHost:   server.URL,
		Enabled:   true,
	})
	require.NoError(t, err)

	assert.False(t, httpCalled, "should not make HTTP calls when nonessential traffic disabled")
	assert.False(t, IsEnabled("test"))
}

func TestIsEnabledCached_MatchesIsEnabled(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("feature_a", true)

	assert.Equal(t, IsEnabled("feature_a"), IsEnabledCached("feature_a"))
	assert.Equal(t, IsEnabled("missing"), IsEnabledCached("missing"))
}

func TestClose_ClearsState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGrowthBookResponse(map[string]any{"test_feature": true}))
	}))
	defer server.Close()

	err := Init(Config{
		ClientKey:       "sdk-test-key",
		APIHost:         server.URL,
		RefreshInterval: 1 * time.Hour,
		Enabled:         true,
	})
	require.NoError(t, err)
	assert.True(t, IsEnabled("test_feature"))

	Close()

	// After close, should return defaults
	assert.False(t, IsEnabled("test_feature"))
	assert.Equal(t, "default", GetValue("test_feature", "default"))
}

func TestUpdateAttributes(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGrowthBookResponse(map[string]any{"feat": true}))
	}))
	defer server.Close()
	defer Close()

	err := Init(Config{
		ClientKey:       "sdk-test-key",
		APIHost:         server.URL,
		RefreshInterval: 1 * time.Hour,
		Enabled:         true,
		Attributes:      map[string]any{"id": "user-1"},
	})
	require.NoError(t, err)
	initialCalls := callCount

	// Update attributes should trigger refresh
	UpdateAttributes(map[string]any{"id": "user-2"})

	// Give goroutine time to execute
	time.Sleep(200 * time.Millisecond)

	assert.Greater(t, callCount, initialCalls, "should have made additional HTTP call after attribute update")
}

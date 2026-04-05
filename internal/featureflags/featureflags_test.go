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

// --- Typed getter tests ---

func TestGetBool(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("bool_true", true)
	features.Store("bool_false", false)
	features.Store("float_nonzero", float64(1))
	features.Store("float_zero", float64(0))
	features.Store("string_true", "true")
	features.Store("string_1", "1")
	features.Store("string_false", "false")
	features.Store("string_other", "hello")
	features.Store("int_nonzero", 5)
	features.Store("int_zero", 0)

	assert.True(t, GetBool("bool_true"))
	assert.False(t, GetBool("bool_false"))
	assert.True(t, GetBool("float_nonzero"))
	assert.False(t, GetBool("float_zero"))
	assert.True(t, GetBool("string_true"))
	assert.True(t, GetBool("string_1"))
	assert.False(t, GetBool("string_false"))
	assert.False(t, GetBool("string_other"))
	assert.True(t, GetBool("int_nonzero"))
	assert.False(t, GetBool("int_zero"))
	assert.False(t, GetBool("missing"))
}

func TestGetBoolDefault(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	// Missing key should return custom default
	assert.True(t, GetBoolDefault("missing", true))
	assert.False(t, GetBoolDefault("missing", false))

	// Non-bool value that doesn't parse should return default
	features.Store("map_val", map[string]any{"key": "value"})
	assert.True(t, GetBoolDefault("map_val", true))
}

func TestGetString(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("str", "hello")
	features.Store("bool_t", true)
	features.Store("bool_f", false)
	features.Store("num", float64(42.5))
	features.Store("int_val", 7)

	assert.Equal(t, "hello", GetString("str"))
	assert.Equal(t, "true", GetString("bool_t"))
	assert.Equal(t, "false", GetString("bool_f"))
	assert.Equal(t, "42.5", GetString("num"))
	assert.Equal(t, "7", GetString("int_val"))
	assert.Equal(t, "", GetString("missing"))
}

func TestGetStringDefault(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	assert.Equal(t, "fallback", GetStringDefault("missing", "fallback"))

	// Complex types return default
	features.Store("map_val", map[string]any{"key": "value"})
	assert.Equal(t, "default", GetStringDefault("map_val", "default"))
}

func TestGetInt(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	// JSON numbers come as float64
	features.Store("float_val", float64(42))
	features.Store("float_frac", float64(3.7))
	features.Store("int_val", 99)
	features.Store("str_num", "123")
	features.Store("str_bad", "not-a-number")

	assert.Equal(t, 42, GetInt("float_val"))
	assert.Equal(t, 3, GetInt("float_frac")) // truncates
	assert.Equal(t, 99, GetInt("int_val"))
	assert.Equal(t, 123, GetInt("str_num"))
	assert.Equal(t, 0, GetInt("str_bad"))
	assert.Equal(t, 0, GetInt("missing"))
}

func TestGetIntDefault(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	assert.Equal(t, 42, GetIntDefault("missing", 42))

	features.Store("bool_val", true)
	assert.Equal(t, 10, GetIntDefault("bool_val", 10))
}

func TestGetFloat(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("f64", float64(3.14))
	features.Store("int_val", 7)
	features.Store("str_f", "2.718")

	assert.InDelta(t, 3.14, GetFloat("f64"), 0.001)
	assert.InDelta(t, 7.0, GetFloat("int_val"), 0.001)
	assert.InDelta(t, 2.718, GetFloat("str_f"), 0.001)
	assert.InDelta(t, 0.0, GetFloat("missing"), 0.001)
}

func TestGetFloatDefault(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	assert.InDelta(t, 1.5, GetFloatDefault("missing", 1.5), 0.001)
}

func TestGetJSON(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	features.Store("map_val", map[string]any{"key": "value", "num": float64(1)})
	features.Store("bool_val", true)
	features.Store("nil_val", nil)

	raw := GetJSON("map_val")
	assert.NotNil(t, raw)
	var m map[string]any
	err = json.Unmarshal(raw, &m)
	require.NoError(t, err)
	assert.Equal(t, "value", m["key"])

	raw = GetJSON("bool_val")
	assert.Equal(t, json.RawMessage("true"), raw)

	assert.Nil(t, GetJSON("nil_val"))
	assert.Nil(t, GetJSON("missing"))
}

// --- Convenience function tests ---

func TestConvenienceFunctions(t *testing.T) {
	defer Close()

	err := Init(Config{Enabled: false})
	require.NoError(t, err)

	mu.Lock()
	enabled = true
	mu.Unlock()

	// Test IsSessionMemoryEnabled delegates to GetBool
	features.Store(FlagSessionMemory, true)
	assert.True(t, IsSessionMemoryEnabled())

	features.Store(FlagSessionMemory, false)
	assert.False(t, IsSessionMemoryEnabled())

	// Test IsAttributionHeaderEnabled has default true
	assert.True(t, IsAttributionHeaderEnabled()) // missing key, default=true

	features.Store(FlagAttributionHeader, false)
	assert.False(t, IsAttributionHeaderEnabled())

	// Test IsCCRBridgeEnabled
	features.Store(FlagCCRBridge, true)
	assert.True(t, IsCCRBridgeEnabled())

	// Test GetWillowMode returns string
	assert.Equal(t, "off", GetWillowMode()) // missing, default "off"

	features.Store(FlagWillowMode, "active")
	assert.Equal(t, "active", GetWillowMode())

	// Test GetCicadaNapMS returns int from float64
	features.Store(FlagCicadaNapMS, float64(500))
	assert.Equal(t, 500, GetCicadaNapMS())

	// Test GetUltraplanModel
	features.Store(FlagUltraplanModel, "opus-4")
	assert.Equal(t, "opus-4", GetUltraplanModel())
}

func TestInitAttributes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGrowthBookResponse(map[string]any{"feat": true}))
	}))
	defer server.Close()
	defer Close()

	attrs := map[string]any{
		"id":              "user-123",
		"sessionId":       "sess-abc",
		"platform":        "linux",
		"appVersion":      "1.0.0",
		"deviceID":        "dev-xyz",
		"isRemoteSession": false,
		"entrypoint":      "cli",
		"installMethod":   "npm",
	}

	err := Init(Config{
		ClientKey:       "sdk-test-key",
		APIHost:         server.URL,
		RefreshInterval: 1 * time.Hour,
		Enabled:         true,
		Attributes:      attrs,
	})
	require.NoError(t, err)

	// Verify attributes were stored
	mu.RLock()
	storedAttrs := cfg.Attributes
	mu.RUnlock()

	assert.Equal(t, "user-123", storedAttrs["id"])
	assert.Equal(t, "sess-abc", storedAttrs["sessionId"])
	assert.Equal(t, "linux", storedAttrs["platform"])
	assert.Equal(t, "1.0.0", storedAttrs["appVersion"])
	assert.Equal(t, "dev-xyz", storedAttrs["deviceID"])
	assert.Equal(t, false, storedAttrs["isRemoteSession"])
	assert.Equal(t, "cli", storedAttrs["entrypoint"])
	assert.Equal(t, "npm", storedAttrs["installMethod"])
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

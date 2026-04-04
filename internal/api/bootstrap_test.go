package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapFetchConfig(t *testing.T) {
	t.Run("fetches and parses bootstrap data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/claude_cli/bootstrap", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"client_data":             map[string]string{"feature": "enabled"},
				"additional_model_options": []string{"claude-opus-4-20250514", "claude-haiku-3.5"},
			})
		}))
		defer server.Close()

		client := NewBootstrapClient(server.URL, "boot-token")
		data, err := client.FetchConfig(context.Background())

		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Equal(t, []string{"claude-opus-4-20250514", "claude-haiku-3.5"}, data.AdditionalModelOptions)
		assert.NotNil(t, data.ClientData)
	})

	t.Run("sets Authorization Bearer header", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"client_data":             map[string]string{},
				"additional_model_options": []string{},
			})
		}))
		defer server.Close()

		client := NewBootstrapClient(server.URL, "my-secret-token")
		_, err := client.FetchConfig(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "Bearer my-secret-token", receivedAuth)
	})

	t.Run("caches result on second call", func(t *testing.T) {
		var callCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"client_data":             map[string]string{"call": "first"},
				"additional_model_options": []string{"model-a"},
			})
		}))
		defer server.Close()

		client := NewBootstrapClient(server.URL, "token")

		// First call
		data1, err := client.FetchConfig(context.Background())
		require.NoError(t, err)

		// Second call -- should be cached
		data2, err := client.FetchConfig(context.Background())
		require.NoError(t, err)

		// Only one HTTP request should have been made
		assert.Equal(t, int32(1), callCount.Load())
		// Both results should be identical
		assert.Equal(t, data1, data2)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "maintenance"}`))
		}))
		defer server.Close()

		client := NewBootstrapClient(server.URL, "token")
		// Use fast retry config to avoid test delays
		client.retryConfig = RetryConfig{
			MaxRetries:    0,
			InitialDelay:  1,
			MaxDelay:      10,
			BackoffFactor: 1.0,
			Jitter:        false,
		}
		data, err := client.FetchConfig(context.Background())

		assert.Error(t, err)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "503")
	})
}

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionIngressAppend(t *testing.T) {
	t.Run("sends PUT with JSON body to correct path", func(t *testing.T) {
		var receivedMethod string
		var receivedPath string
		var receivedBody map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewSessionIngressClient(server.URL, "ingress-token")
		entry := IngressEntry{
			UUID: "entry-uuid-1",
			Type: "message",
			Data: json.RawMessage(`{"role":"user","content":"hello"}`),
		}
		err := client.Append(context.Background(), "session-123", entry)

		require.NoError(t, err)
		assert.Equal(t, "PUT", receivedMethod)
		assert.Equal(t, "/api/session_ingress/session-123", receivedPath)
		assert.Equal(t, "entry-uuid-1", receivedBody["uuid"])
		assert.Equal(t, "message", receivedBody["type"])
	})

	t.Run("includes Last-Uuid header for optimistic concurrency", func(t *testing.T) {
		var firstLastUUID string
		var secondLastUUID string
		var callCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := callCount.Add(1)
			if count == 1 {
				firstLastUUID = r.Header.Get("Last-Uuid")
			} else {
				secondLastUUID = r.Header.Get("Last-Uuid")
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewSessionIngressClient(server.URL, "token")

		// First append -- no last UUID yet
		entry1 := IngressEntry{UUID: "uuid-aaa", Type: "message", Data: json.RawMessage(`{}`)}
		err := client.Append(context.Background(), "sess-1", entry1)
		require.NoError(t, err)

		// Second append -- should have the first UUID as Last-Uuid
		entry2 := IngressEntry{UUID: "uuid-bbb", Type: "message", Data: json.RawMessage(`{}`)}
		err = client.Append(context.Background(), "sess-1", entry2)
		require.NoError(t, err)

		// First call should have empty Last-Uuid
		assert.Equal(t, "", firstLastUUID)
		// Second call should have the first entry's UUID
		assert.Equal(t, "uuid-aaa", secondLastUUID)
	})

	t.Run("sets Authorization Bearer header", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewSessionIngressClient(server.URL, "my-ingress-token")
		entry := IngressEntry{UUID: "u1", Type: "msg", Data: json.RawMessage(`{}`)}
		err := client.Append(context.Background(), "s1", entry)

		require.NoError(t, err)
		assert.Equal(t, "Bearer my-ingress-token", receivedAuth)
	})

	t.Run("retries on 429", func(t *testing.T) {
		var callCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := callCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewSessionIngressClient(server.URL, "token")
		client.retryConfig = RetryConfig{
			MaxRetries:    3,
			InitialDelay:  1,
			MaxDelay:      10,
			BackoffFactor: 1.0,
			Jitter:        false,
		}

		entry := IngressEntry{UUID: "u1", Type: "msg", Data: json.RawMessage(`{}`)}
		err := client.Append(context.Background(), "s1", entry)

		require.NoError(t, err)
		assert.Equal(t, int32(2), callCount.Load())
	})
}

func TestSessionIngressFetch(t *testing.T) {
	t.Run("fetches transcript entries from session", func(t *testing.T) {
		entries := []IngressEntry{
			{
				UUID:      "uuid-1",
				Type:      "message",
				Data:      json.RawMessage(`{"role":"user","content":"hello"}`),
				Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				UUID:      "uuid-2",
				Type:      "message",
				Data:      json.RawMessage(`{"role":"assistant","content":"hi"}`),
				Timestamp: time.Date(2025, 1, 1, 0, 0, 1, 0, time.UTC),
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/session_ingress/sess-abc", r.URL.Path)
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "Bearer fetch-token", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(entries)
		}))
		defer server.Close()

		client := NewSessionIngressClient(server.URL, "fetch-token")
		result, err := client.Fetch(context.Background(), "sess-abc")

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "uuid-1", result[0].UUID)
		assert.Equal(t, "uuid-2", result[1].UUID)
		assert.Equal(t, "message", result[0].Type)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "session not found"}`))
		}))
		defer server.Close()

		client := NewSessionIngressClient(server.URL, "token")
		result, err := client.Fetch(context.Background(), "nonexistent")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "404")
	})
}

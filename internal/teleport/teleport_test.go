package teleport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_CreateSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sessions", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var body map[string]string
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "production", body["environment"])

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(SessionInfo{
			ID:          "sess-abc",
			Status:      "active",
			CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Environment: "production",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.CreateSession(context.Background(), "production")
	require.NoError(t, err)
	assert.Equal(t, "sess-abc", info.ID)
	assert.Equal(t, "active", info.Status)
	assert.Equal(t, "production", info.Environment)
}

func TestClient_FetchSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/sessions/sess-xyz", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionInfo{
			ID:          "sess-xyz",
			Status:      "running",
			CreatedAt:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			Environment: "staging",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.FetchSession(context.Background(), "sess-xyz")
	require.NoError(t, err)
	assert.Equal(t, "sess-xyz", info.ID)
	assert.Equal(t, "running", info.Status)
	assert.Equal(t, "staging", info.Environment)
}

func TestClient_ResumeSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sessions/sess-resume/resume", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionInfo{
			ID:          "sess-resume",
			Status:      "active",
			CreatedAt:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Environment: "production",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.ResumeSession(context.Background(), "sess-resume")
	require.NoError(t, err)
	assert.Equal(t, "sess-resume", info.ID)
	assert.Equal(t, "active", info.Status)
}

func TestClient_RetryOn429(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := reqCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionInfo{
			ID:     "sess-429",
			Status: "active",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.FetchSession(context.Background(), "sess-429")
	require.NoError(t, err)
	assert.Equal(t, "sess-429", info.ID)
	assert.True(t, reqCount.Load() >= 2, "expected at least 2 requests (1 retry)")
}

func TestClient_RetryOn500(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := reqCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionInfo{
			ID:     "sess-500",
			Status: "active",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.FetchSession(context.Background(), "sess-500")
	require.NoError(t, err)
	assert.Equal(t, "sess-500", info.ID)
	assert.True(t, reqCount.Load() >= 2, "expected at least 2 requests (1 retry)")
}

func TestClient_MaxRetries(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	_, err := client.FetchSession(context.Background(), "sess-fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retries")
	// maxRetries + 1 initial attempt = 4 total requests
	assert.Equal(t, int32(maxRetries+1), reqCount.Load())
}

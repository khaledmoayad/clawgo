package teleport

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

func TestClient_CreateSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sessions", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, CCR_BYOC_BETA, r.Header.Get("anthropic-beta"))

		// Decode request body
		var body map[string]string
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "production", body["environment"])

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(SessionResource{
			Type:          "session",
			ID:            "sess-abc",
			SessionStatus: "running",
			EnvironmentID: "production",
			CreatedAt:     "2026-01-01T00:00:00Z",
			UpdatedAt:     "2026-01-01T00:00:00Z",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.CreateSession(context.Background(), "production")
	require.NoError(t, err)
	assert.Equal(t, "sess-abc", info.ID)
	assert.Equal(t, "running", info.SessionStatus)
	assert.Equal(t, "production", info.EnvironmentID)
}

func TestClient_FetchSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/sessions/sess-xyz", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionResource{
			Type:          "session",
			ID:            "sess-xyz",
			SessionStatus: "running",
			EnvironmentID: "staging",
			CreatedAt:     "2026-02-01T00:00:00Z",
			UpdatedAt:     "2026-02-01T00:00:00Z",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.FetchSession(context.Background(), "sess-xyz")
	require.NoError(t, err)
	assert.Equal(t, "sess-xyz", info.ID)
	assert.Equal(t, "running", info.SessionStatus)
	assert.Equal(t, "staging", info.EnvironmentID)
}

func TestClient_FetchSession_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	_, err := client.FetchSession(context.Background(), "sess-missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestClient_FetchSession_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	_, err := client.FetchSession(context.Background(), "sess-expired")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication expired")
}

func TestClient_ResumeSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sessions/sess-resume/resume", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionResource{
			Type:          "session",
			ID:            "sess-resume",
			SessionStatus: "running",
			EnvironmentID: "production",
			CreatedAt:     "2026-03-01T00:00:00Z",
			UpdatedAt:     "2026-03-01T00:00:00Z",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	info, err := client.ResumeSession(context.Background(), "sess-resume")
	require.NoError(t, err)
	assert.Equal(t, "sess-resume", info.ID)
	assert.Equal(t, "running", info.SessionStatus)
}

func TestClient_SendEventToRemoteSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sessions/sess-event/events", r.URL.Path)
		assert.Equal(t, CCR_BYOC_BETA, r.Header.Get("anthropic-beta"))

		var body SendEventsRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		require.Len(t, body.Events, 1)
		assert.Equal(t, "user", body.Events[0].Type)
		assert.Equal(t, "user", body.Events[0].Message.Role)
		assert.Equal(t, "test-uuid", body.Events[0].UUID)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	ok, err := client.SendEventToRemoteSession(context.Background(), "sess-event", "hello", "test-uuid")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestClient_UpdateSessionTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v1/sessions/sess-title", r.URL.Path)

		var body UpdateSessionRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "New Title", body.Title)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	ok, err := client.UpdateSessionTitle(context.Background(), "sess-title", "New Title")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestClient_FetchCodeSessionsFromSessionsAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/sessions", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListSessionsResponse{
			Data: []SessionResource{
				{Type: "session", ID: "sess-1", SessionStatus: "running"},
				{Type: "session", ID: "sess-2", SessionStatus: "idle"},
			},
			HasMore: false,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, func() string { return "test-token" })
	sessions, err := client.FetchCodeSessionsFromSessionsAPI(context.Background())
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Equal(t, "sess-1", sessions[0].ID)
	assert.Equal(t, "sess-2", sessions[1].ID)
}

func TestGetBranchFromSession(t *testing.T) {
	outcomeJSON, _ := json.Marshal(GitRepositoryOutcome{
		Type: "git_repository",
		GitInfo: OutcomeGitInfo{
			Type:     "github",
			Repo:     "owner/repo",
			Branches: []string{"feature-branch", "main"},
		},
	})

	session := SessionResource{
		SessionContext: SessionContext{
			Outcomes: []json.RawMessage{outcomeJSON},
		},
	}
	assert.Equal(t, "feature-branch", GetBranchFromSession(session))
}

func TestGetBranchFromSession_NoOutcomes(t *testing.T) {
	session := SessionResource{}
	assert.Equal(t, "", GetBranchFromSession(session))
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
		json.NewEncoder(w).Encode(SessionResource{
			ID:            "sess-429",
			SessionStatus: "running",
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
		json.NewEncoder(w).Encode(SessionResource{
			ID:            "sess-500",
			SessionStatus: "running",
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
	// maxRetries + 1 initial attempt = 5 total requests
	assert.Equal(t, int32(maxRetries+1), reqCount.Load())
}

func TestClient_OrgUUIDHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "org-123", r.Header.Get("x-organization-uuid"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SessionResource{ID: "sess-org"})
	}))
	defer srv.Close()

	client := NewClientWithOrg(srv.URL, func() string { return "test-token" }, func() string { return "org-123" })
	info, err := client.FetchSession(context.Background(), "sess-org")
	require.NoError(t, err)
	assert.Equal(t, "sess-org", info.ID)
}

func TestGetOAuthHeaders(t *testing.T) {
	headers := GetOAuthHeaders("my-token")
	assert.Equal(t, "Bearer my-token", headers["Authorization"])
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "2023-06-01", headers["anthropic-version"])
}

func TestIsTransientNetworkError(t *testing.T) {
	assert.False(t, IsTransientNetworkError(nil))
	assert.True(t, IsTransientNetworkError(assert.AnError)) // wraps "assert.AnError" -- but let's test real ones
}

package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIClient_RegisterBridgeEnvironment(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/environments/bridge", r.URL.Path)
		receivedAuth = r.Header.Get("Authorization")

		err := json.NewDecoder(r.Body).Decode(&receivedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"environment_id":     "env-123",
			"environment_secret": "secret-abc",
		})
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "test-token" })
	cfg := BridgeConfig{
		EnvironmentName:       "my-machine",
		Dir:                   "/home/user/project",
		Branch:                "main",
		MaxConcurrentSessions: 5,
		WorkerType:            "claude_code",
	}
	envID, envSecret, err := client.RegisterBridgeEnvironment(context.Background(), cfg)

	require.NoError(t, err)
	assert.Equal(t, "env-123", envID)
	assert.Equal(t, "secret-abc", envSecret)
	assert.Equal(t, "my-machine", receivedBody["machine_name"])
	assert.Equal(t, "/home/user/project", receivedBody["directory"])
	assert.Equal(t, "Bearer test-token", receivedAuth)
}

func TestAPIClient_PollForWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/environments/env-123/work/poll", r.URL.Path)
		assert.Equal(t, "Bearer env-secret", r.Header.Get("Authorization"))

		json.NewEncoder(w).Encode(WorkResponse{
			ID:   "work-1",
			Type: "work",
			Data: WorkData{Type: "session", ID: "sess-1"},
		})
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	work, err := client.PollForWork(context.Background(), "env-123", "env-secret", nil)

	require.NoError(t, err)
	require.NotNil(t, work)
	assert.Equal(t, "work-1", work.ID)
	assert.Equal(t, "session", work.Data.Type)
	assert.Equal(t, "sess-1", work.Data.ID)
}

func TestAPIClient_PollForWork_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	work, err := client.PollForWork(context.Background(), "env-1", "secret", nil)

	require.NoError(t, err)
	assert.Nil(t, work)
}

func TestAPIClient_AcknowledgeWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/environments/env-1/work/work-1/ack", r.URL.Path)
		assert.Equal(t, "Bearer session-tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	err := client.AcknowledgeWork(context.Background(), "env-1", "work-1", "session-tok")
	require.NoError(t, err)
}

func TestAPIClient_HeartbeatWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/environments/env-1/work/work-1/heartbeat", r.URL.Path)
		assert.Equal(t, "Bearer session-tok", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(HeartbeatResponse{
			LeaseExtended: true,
			State:         "running",
		})
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	lease, state, err := client.HeartbeatWork(context.Background(), "env-1", "work-1", "session-tok")
	require.NoError(t, err)
	assert.True(t, lease)
	assert.Equal(t, "running", state)
}

func TestAPIClient_ArchiveSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/sessions/sess-1/archive", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	err := client.ArchiveSession(context.Background(), "sess-1")
	require.NoError(t, err)
}

func TestAPIClient_ArchiveSession_AlreadyArchived(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	err := client.ArchiveSession(context.Background(), "sess-1")
	require.NoError(t, err) // 409 is not an error
}

func TestAPIClient_DeregisterEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/environments/bridge/env-1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	err := client.DeregisterEnvironment(context.Background(), "env-1")
	require.NoError(t, err)
}

func TestDecodeWorkSecret(t *testing.T) {
	secret := WorkSecret{
		Version:             1,
		SessionIngressToken: "jwt-token-abc",
		APIBaseURL:          "https://api.anthropic.com",
		Sources:             []WorkSecretSource{{Type: "git"}},
		Auth:                []WorkSecretAuth{{Type: "bearer", Token: "auth-tok"}},
	}
	data, err := json.Marshal(secret)
	require.NoError(t, err)

	encoded := base64.RawURLEncoding.EncodeToString(data)
	decoded, err := DecodeWorkSecret(encoded)

	require.NoError(t, err)
	assert.Equal(t, 1, decoded.Version)
	assert.Equal(t, "jwt-token-abc", decoded.SessionIngressToken)
	assert.Equal(t, "https://api.anthropic.com", decoded.APIBaseURL)
}

func TestDecodeWorkSecret_InvalidVersion(t *testing.T) {
	secret := map[string]interface{}{
		"version":               2,
		"session_ingress_token": "tok",
		"api_base_url":          "https://example.com",
	}
	data, _ := json.Marshal(secret)
	encoded := base64.RawURLEncoding.EncodeToString(data)

	_, err := DecodeWorkSecret(encoded)
	assert.ErrorContains(t, err, "unsupported work secret version: 2")
}

func TestDecodeWorkSecret_MissingToken(t *testing.T) {
	secret := map[string]interface{}{
		"version":      1,
		"api_base_url": "https://example.com",
	}
	data, _ := json.Marshal(secret)
	encoded := base64.RawURLEncoding.EncodeToString(data)

	_, err := DecodeWorkSecret(encoded)
	assert.ErrorContains(t, err, "missing or empty session_ingress_token")
}

func TestDecodeWorkSecret_MissingBaseURL(t *testing.T) {
	secret := map[string]interface{}{
		"version":               1,
		"session_ingress_token": "tok",
	}
	data, _ := json.Marshal(secret)
	encoded := base64.RawURLEncoding.EncodeToString(data)

	_, err := DecodeWorkSecret(encoded)
	assert.ErrorContains(t, err, "missing api_base_url")
}

func TestSessionPool_CanSpawn(t *testing.T) {
	pool := NewSessionPool(2)
	assert.True(t, pool.CanSpawn())

	ctx := context.Background()
	ctx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	_, err := pool.Spawn(ctx1, &WorkResponse{ID: "w1", Data: WorkData{ID: "s1"}}, func(ctx context.Context, w *WorkResponse) {
		<-ctx.Done()
	})
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	_, err = pool.Spawn(ctx2, &WorkResponse{ID: "w2", Data: WorkData{ID: "s2"}}, func(ctx context.Context, w *WorkResponse) {
		<-ctx.Done()
	})
	require.NoError(t, err)

	assert.False(t, pool.CanSpawn())
	assert.Equal(t, 2, pool.ActiveCount())

	cancel1()
	cancel2()
	pool.StopAll()
}

func TestSessionPool_Spawn(t *testing.T) {
	pool := NewSessionPool(5)
	var handlerCalled atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handle, err := pool.Spawn(ctx, &WorkResponse{ID: "w1", Data: WorkData{ID: "s1"}}, func(ctx context.Context, w *WorkResponse) {
		handlerCalled.Store(true)
		assert.Equal(t, "w1", w.ID)
		assert.Equal(t, "s1", w.Data.ID)
		<-ctx.Done()
	})
	require.NoError(t, err)
	assert.Equal(t, "w1", handle.WorkID)
	assert.Equal(t, "s1", handle.SessionID)

	time.Sleep(50 * time.Millisecond)
	assert.True(t, handlerCalled.Load())
	assert.Equal(t, 1, pool.ActiveCount())

	cancel()
	<-handle.Done

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, pool.ActiveCount())
}

func TestSessionPool_GetByWorkID(t *testing.T) {
	pool := NewSessionPool(5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := pool.Spawn(ctx, &WorkResponse{ID: "w1", Data: WorkData{ID: "s1"}}, func(ctx context.Context, w *WorkResponse) {
		<-ctx.Done()
	})
	require.NoError(t, err)

	handle := pool.GetByWorkID("w1")
	assert.NotNil(t, handle)
	assert.Equal(t, "w1", handle.WorkID)

	missing := pool.GetByWorkID("nonexistent")
	assert.Nil(t, missing)

	cancel()
	pool.StopAll()
}

func TestSessionPool_StopAll(t *testing.T) {
	pool := NewSessionPool(5)
	ctx := context.Background()

	var cancelled1, cancelled2 atomic.Bool

	_, err := pool.Spawn(ctx, &WorkResponse{ID: "w1", Data: WorkData{ID: "s1"}}, func(ctx context.Context, w *WorkResponse) {
		<-ctx.Done()
		cancelled1.Store(true)
	})
	require.NoError(t, err)

	_, err = pool.Spawn(ctx, &WorkResponse{ID: "w2", Data: WorkData{ID: "s2"}}, func(ctx context.Context, w *WorkResponse) {
		<-ctx.Done()
		cancelled2.Store(true)
	})
	require.NoError(t, err)

	assert.Equal(t, 2, pool.ActiveCount())

	pool.StopAll()

	assert.True(t, cancelled1.Load())
	assert.True(t, cancelled2.Load())
	assert.Equal(t, 0, pool.ActiveCount())
}

func TestSessionHandle_UpdateAccessToken(t *testing.T) {
	handle := &SessionHandle{AccessToken: "initial"}
	handle.UpdateAccessToken("refreshed")
	assert.Equal(t, "refreshed", handle.AccessToken)
}

func TestSessionHandle_AddActivity(t *testing.T) {
	handle := &SessionHandle{}

	act := SessionActivity{Type: SessionActivityToolStart, Summary: "Reading file", Timestamp: 1234}
	handle.AddActivity(act)

	assert.Len(t, handle.Activities, 1)
	assert.NotNil(t, handle.CurrentActivity)
	assert.Equal(t, "Reading file", handle.CurrentActivity.Summary)
}

func TestWorkHandler_HandleWork_Healthcheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no API calls should be made for healthcheck")
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	handler := NewWorkHandler(client, "env-1", BridgeConfig{})
	pool := NewSessionPool(5)

	work := &WorkResponse{
		ID:   "work-hc",
		Data: WorkData{Type: "healthcheck", ID: "hc-1"},
	}
	err := handler.HandleWork(context.Background(), work, pool)
	require.NoError(t, err)
	assert.Equal(t, 0, pool.ActiveCount())
}

func TestBridge_PollLoop(t *testing.T) {
	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/environments/bridge":
			json.NewEncoder(w).Encode(map[string]string{
				"environment_id":     "env-1",
				"environment_secret": "secret-1",
			})
		case r.Method == http.MethodGet:
			pollCount.Add(1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("null"))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	b := NewBridge(BridgeConfig{
		APIBaseURL:            srv.URL,
		GetToken:              func() string { return "tok" },
		EnvironmentName:       "test-env",
		MaxConcurrentSessions: 2,
		PollInterval:          50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := b.Start(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	assert.GreaterOrEqual(t, pollCount.Load(), int32(2))
}

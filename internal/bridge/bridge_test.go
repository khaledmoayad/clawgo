package bridge

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

func TestAPIClient_RegisterEnvironment(t *testing.T) {
	var receivedName string
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/bridge/environments", r.URL.Path)
		receivedAuth = r.Header.Get("Authorization")

		var body map[string]string
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		receivedName = body["name"]

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Environment{
			ID:     "env-123",
			Name:   body["name"],
			Status: "online",
		})
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "test-token" })
	env, err := client.RegisterEnvironment(context.Background(), "my-machine")

	require.NoError(t, err)
	assert.Equal(t, "env-123", env.ID)
	assert.Equal(t, "my-machine", env.Name)
	assert.Equal(t, "online", env.Status)
	assert.Equal(t, "my-machine", receivedName)
	assert.Equal(t, "Bearer test-token", receivedAuth)
}

func TestAPIClient_PollWork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/bridge/environments/env-123/work", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))

		items := []WorkItem{
			{SessionID: "sess-1", Prompt: "hello", OrgUUID: "org-a"},
			{SessionID: "sess-2", Prompt: "world", OrgUUID: "org-b"},
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	items, err := client.PollWork(context.Background(), "env-123")

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sess-1", items[0].SessionID)
	assert.Equal(t, "hello", items[0].Prompt)
	assert.Equal(t, "sess-2", items[1].SessionID)
	assert.Equal(t, "world", items[1].Prompt)
}

func TestAPIClient_PollWork_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]WorkItem{})
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, func() string { return "tok" })
	items, err := client.PollWork(context.Background(), "env-1")

	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestSessionPool_CanSpawn(t *testing.T) {
	pool := NewSessionPool(2)
	assert.True(t, pool.CanSpawn())

	ctx := context.Background()
	// Spawn 2 sessions that block until cancelled
	ctx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	_, err := pool.Spawn(ctx1, WorkItem{SessionID: "s1"}, func(ctx context.Context, w WorkItem) {
		<-ctx.Done()
	})
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	_, err = pool.Spawn(ctx2, WorkItem{SessionID: "s2"}, func(ctx context.Context, w WorkItem) {
		<-ctx.Done()
	})
	require.NoError(t, err)

	assert.False(t, pool.CanSpawn())
	assert.Equal(t, 2, pool.ActiveCount())

	// Cleanup
	cancel1()
	cancel2()
	pool.StopAll()
}

func TestSessionPool_Spawn(t *testing.T) {
	pool := NewSessionPool(5)
	var handlerCalled atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handle, err := pool.Spawn(ctx, WorkItem{SessionID: "s1", Prompt: "test"}, func(ctx context.Context, w WorkItem) {
		handlerCalled.Store(true)
		assert.Equal(t, "s1", w.SessionID)
		assert.Equal(t, "test", w.Prompt)
		<-ctx.Done()
	})
	require.NoError(t, err)
	assert.Equal(t, "s1", handle.ID)

	// Give handler goroutine time to start
	time.Sleep(50 * time.Millisecond)
	assert.True(t, handlerCalled.Load())
	assert.Equal(t, 1, pool.ActiveCount())

	cancel()
	<-handle.Done

	// Session should be removed from pool after completion
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, pool.ActiveCount())
}

func TestSessionPool_StopAll(t *testing.T) {
	pool := NewSessionPool(5)
	ctx := context.Background()

	var cancelled1, cancelled2 atomic.Bool

	_, err := pool.Spawn(ctx, WorkItem{SessionID: "s1"}, func(ctx context.Context, w WorkItem) {
		<-ctx.Done()
		cancelled1.Store(true)
	})
	require.NoError(t, err)

	_, err = pool.Spawn(ctx, WorkItem{SessionID: "s2"}, func(ctx context.Context, w WorkItem) {
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

func TestBridge_PollLoop(t *testing.T) {
	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/bridge/environments":
			json.NewEncoder(w).Encode(Environment{ID: "env-1", Name: "test", Status: "online"})
		case r.Method == http.MethodGet:
			pollCount.Add(1)
			json.NewEncoder(w).Encode([]WorkItem{})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	b := NewBridge(Config{
		APIBaseURL:            srv.URL,
		GetToken:              func() string { return "tok" },
		EnvironmentName:       "test-env",
		MaxConcurrentSessions: 2,
		PollInterval:          50 * time.Millisecond, // Short interval for testing
	})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Start returns when context is cancelled
	err := b.Start(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Should have polled multiple times in 300ms at 50ms intervals
	assert.GreaterOrEqual(t, pollCount.Load(), int32(2))
}

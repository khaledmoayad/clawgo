package analytics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDatadogLogger_Disabled(t *testing.T) {
	d := &DatadogLogger{
		client:  &http.Client{Timeout: networkTimeout},
		enabled: false,
	}

	d.Track("test-event", map[string]interface{}{"key": "val"})

	d.mu.Lock()
	batchLen := len(d.batch)
	d.mu.Unlock()

	if batchLen != 0 {
		t.Errorf("disabled logger accumulated %d events, want 0", batchLen)
	}
}

func TestDatadogLogger_Track(t *testing.T) {
	// Create logger directly to avoid RegisterCleanup side effect in tests
	d := &DatadogLogger{
		client:  &http.Client{Timeout: networkTimeout},
		enabled: true,
	}

	d.Track("click_button", map[string]interface{}{"page": "home"})

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.batch) != 1 {
		t.Fatalf("batch length = %d, want 1", len(d.batch))
	}

	entry := d.batch[0]
	if entry.DDSource != "go" {
		t.Errorf("DDSource = %q, want %q", entry.DDSource, "go")
	}
	if entry.Service != "claude-code" {
		t.Errorf("Service = %q, want %q", entry.Service, "claude-code")
	}
	if entry.Hostname != "claude-code" {
		t.Errorf("Hostname = %q, want %q", entry.Hostname, "claude-code")
	}
	if entry.Message != "click_button" {
		t.Errorf("Message = %q, want %q", entry.Message, "click_button")
	}

	// Stop timer to avoid goroutine leaks
	if d.timer != nil {
		d.timer.Stop()
	}
}

func TestDatadogLogger_BatchFlush(t *testing.T) {
	var mu sync.Mutex
	var received []DatadogLog

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var logs []DatadogLog
		if err := json.Unmarshal(body, &logs); err == nil {
			mu.Lock()
			received = append(received, logs...)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	d := &DatadogLogger{
		client:  server.Client(),
		enabled: true,
	}

	// Override the send method to use test server
	origSend := d.send
	_ = origSend
	d.client = server.Client()

	// We need to intercept the endpoint. Since we can't easily swap the
	// const, we'll test that batch flush triggers by tracking maxBatchSize
	// events and verifying the batch is cleared.
	for i := 0; i < maxBatchSize; i++ {
		d.Track("batch-event", map[string]interface{}{"i": i})
	}

	// After reaching maxBatchSize, the batch should have been flushed
	// (goroutine fires async). Give it a moment.
	time.Sleep(100 * time.Millisecond)

	d.mu.Lock()
	batchLen := len(d.batch)
	d.mu.Unlock()

	if batchLen != 0 {
		t.Errorf("batch not flushed after maxBatchSize: got %d events remaining", batchLen)
	}
}

func TestDatadogLogger_ShutdownFlush(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	d := &DatadogLogger{
		client:  server.Client(),
		enabled: true,
	}

	// Track a few events (below maxBatchSize)
	for i := 0; i < 5; i++ {
		d.Track("shutdown-test", nil)
	}

	// Verify events are batched
	d.mu.Lock()
	preShutdownLen := len(d.batch)
	d.mu.Unlock()

	if preShutdownLen != 5 {
		t.Fatalf("pre-shutdown batch = %d, want 5", preShutdownLen)
	}

	// Shutdown should flush the remaining events
	d.Shutdown()

	d.mu.Lock()
	postShutdownLen := len(d.batch)
	d.mu.Unlock()

	if postShutdownLen != 0 {
		t.Errorf("post-shutdown batch = %d, want 0", postShutdownLen)
	}
}

func TestDatadogLogger_ShutdownIdempotent(t *testing.T) {
	d := &DatadogLogger{
		client:  &http.Client{Timeout: networkTimeout},
		enabled: true,
	}

	d.Track("idem-test", nil)

	// Calling Shutdown multiple times should not panic
	d.Shutdown()
	d.Shutdown()
	d.Shutdown()
}

func TestBuildTags(t *testing.T) {
	tags := buildTags("page_view", map[string]interface{}{
		"page":    "home",
		"version": "1.0",
	})

	// Tags should be sorted: event:page_view,page:home,version:1.0
	expected := "event:page_view,page:home,version:1.0"
	if tags != expected {
		t.Errorf("buildTags = %q, want %q", tags, expected)
	}
}

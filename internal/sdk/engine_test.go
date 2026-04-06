package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// newTestClient creates an API client pointed at a test server.
func newTestClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	// Set env for the SDK client
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	client, err := api.NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}
	return client
}

// sseResponse builds a minimal Anthropic streaming SSE response that produces a text message.
func sseResponse(text string) string {
	// Minimal SSE stream matching the Anthropic Messages API streaming format
	var b strings.Builder

	// message_start
	b.WriteString("event: message_start\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}%s`, "\n\n"))

	// content_block_start (text)
	b.WriteString("event: content_block_start\n")
	b.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n")

	// content_block_delta (text)
	escaped, _ := json.Marshal(text)
	b.WriteString("event: content_block_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%s}}`, escaped) + "\n\n")

	// content_block_stop
	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}` + "\n\n")

	// message_delta (stop_reason=end_turn)
	b.WriteString("event: message_delta\n")
	b.WriteString(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}` + "\n\n")

	// message_stop
	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}` + "\n\n")

	return b.String()
}

func TestQueryEngineCreation(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	registry := tools.NewRegistry()

	cfg := QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		MaxTurns:     5,
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	}

	engine := NewQueryEngine(cfg)
	if engine == nil {
		t.Fatal("NewQueryEngine returned nil")
	}
	if engine.SessionCost() != 0 {
		t.Errorf("initial cost = %v, want 0", engine.SessionCost())
	}
	msgs := engine.Messages()
	if len(msgs) != 0 {
		t.Errorf("initial messages = %d, want 0", len(msgs))
	}
}

func TestQueryEngineAsk(t *testing.T) {
	responseText := "Hello from Claude"
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse(responseText))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch := engine.Ask(ctx, "Hello")

	if ch == nil {
		t.Fatal("Ask returned nil channel")
	}

	// Collect events
	var events []SDKEvent
	for evt := range ch {
		events = append(events, evt)
	}

	// Should have at least text delta and turn complete events
	if len(events) == 0 {
		t.Fatal("no events received")
	}

	// Check for text delta
	hasTextDelta := false
	for _, evt := range events {
		if evt.Type == EventTextDelta {
			hasTextDelta = true
			if evt.Text != responseText {
				t.Errorf("text delta = %q, want %q", evt.Text, responseText)
			}
		}
	}
	if !hasTextDelta {
		t.Error("expected at least one TextDelta event")
	}

	// Check for turn complete
	hasTurnComplete := false
	for _, evt := range events {
		if evt.Type == EventTurnComplete {
			hasTurnComplete = true
		}
	}
	if !hasTurnComplete {
		t.Error("expected TurnComplete event")
	}
}

func TestQueryEngineHistory(t *testing.T) {
	callCount := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse(fmt.Sprintf("Response %d", callCount)))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	})

	ctx := context.Background()

	// First ask
	for range engine.Ask(ctx, "First message") {
		// drain
	}

	// After first ask, should have user + assistant messages
	msgs := engine.Messages()
	if len(msgs) < 2 {
		t.Fatalf("after first ask: messages = %d, want >= 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("first message role = %q, want %q", msgs[0].Role, "user")
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("second message role = %q, want %q", msgs[1].Role, "assistant")
	}

	// Second ask should preserve history
	for range engine.Ask(ctx, "Second message") {
		// drain
	}

	msgs = engine.Messages()
	if len(msgs) < 4 {
		t.Fatalf("after second ask: messages = %d, want >= 4", len(msgs))
	}
	// Messages should be: user1, assistant1, user2, assistant2
	if msgs[2].Role != "user" {
		t.Errorf("third message role = %q, want %q", msgs[2].Role, "user")
	}
	if msgs[3].Role != "assistant" {
		t.Errorf("fourth message role = %q, want %q", msgs[3].Role, "assistant")
	}
}

func TestQueryEngineCancel(t *testing.T) {
	// Server that delays response to test cancellation
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Send message_start but then hang
		fmt.Fprintf(w, "event: message_start\n")
		fmt.Fprintf(w, `data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`+"\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Wait long enough that cancellation will trigger
		<-r.Context().Done()
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	})

	ctx, cancel := context.WithCancel(context.Background())
	ch := engine.Ask(ctx, "Hello")

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Channel should close without blocking indefinitely
	done := make(chan struct{})
	go func() {
		for range ch {
			// drain
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - channel closed
	case <-time.After(5 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}

func sseResponseWithTokens(text string, inputTokens, outputTokens int) string {
	// SSE stream matching the Anthropic Messages API streaming format with custom token counts
	var b strings.Builder

	// message_start
	b.WriteString("event: message_start\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":%d,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`, inputTokens) + "\n\n")

	// content_block_start (text)
	b.WriteString("event: content_block_start\n")
	b.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n")

	// content_block_delta (text)
	escaped, _ := json.Marshal(text)
	b.WriteString("event: content_block_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%s}}`, escaped) + "\n\n")

	// content_block_stop
	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}` + "\n\n")

	// message_delta (stop_reason=end_turn)
	b.WriteString("event: message_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":%d}}`, outputTokens) + "\n\n")

	// message_stop
	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}` + "\n\n")

	return b.String()
}

func TestQueryEngineBudgetEnforcement(t *testing.T) {
	// Each call uses 100000 input tokens + 100000 output tokens.
	// Sonnet 4 pricing: $3/MTok input + $15/MTok output
	// Cost per call: 100000 * 3e-6 + 100000 * 15e-6 = 0.30 + 1.50 = $1.80
	// Budget set to $1.00 so it should stop after first call.
	callCount := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponseWithTokens("response", 100000, 100000))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		MaxBudgetUSD: 1.00,
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	})

	ctx := context.Background()
	var events []SDKEvent
	for evt := range engine.Ask(ctx, "Hello") {
		events = append(events, evt)
	}

	// Should have received a result event indicating budget exceeded
	hasBudgetError := false
	for _, evt := range events {
		if evt.Type == EventResult && evt.IsError {
			hasBudgetError = true
		}
	}
	if !hasBudgetError {
		t.Error("expected a result event with IsError=true indicating budget exceeded")
	}

	// Should only have made one API call since budget exceeded after first
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

func TestQueryEngineConfigCustomSystemPrompt(t *testing.T) {
	var capturedBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(map[string]any{})
		bodyBytes := make([]byte, r.ContentLength)
		r.Body.Read(bodyBytes)
		capturedBody = string(bodyBytes)
		_ = body
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse("ok"))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:             client,
		Registry:           registry,
		SystemPrompt:       "Original prompt",
		CustomSystemPrompt: "Custom override prompt",
		WorkingDir:         "/tmp",
		ProjectRoot:        "/tmp",
	})

	ctx := context.Background()
	for range engine.Ask(ctx, "Hello") {
		// drain
	}

	// The request body should contain the custom prompt, not the original
	if !strings.Contains(capturedBody, "Custom override prompt") {
		t.Error("expected CustomSystemPrompt to override SystemPrompt in API request")
	}
	if strings.Contains(capturedBody, "Original prompt") {
		t.Error("expected original SystemPrompt to be overridden by CustomSystemPrompt")
	}
}

func TestQueryEngineConfigAppendSystemPrompt(t *testing.T) {
	var capturedBody string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		bodyBytes := make([]byte, r.ContentLength)
		r.Body.Read(bodyBytes)
		capturedBody = string(bodyBytes)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse("ok"))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:             client,
		Registry:           registry,
		SystemPrompt:       "Base prompt",
		AppendSystemPrompt: "Appended instructions",
		WorkingDir:         "/tmp",
		ProjectRoot:        "/tmp",
	})

	ctx := context.Background()
	for range engine.Ask(ctx, "Hello") {
		// drain
	}

	if !strings.Contains(capturedBody, "Base prompt") {
		t.Error("expected base SystemPrompt to be present")
	}
	if !strings.Contains(capturedBody, "Appended instructions") {
		t.Error("expected AppendSystemPrompt to be appended to system prompt")
	}
}

func TestQueryEngineConfigPermissionMode(t *testing.T) {
	// This test verifies that the PermissionMode field exists and is wired
	// into the engine config. We verify the config field is accepted.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse("ok"))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:         client,
		Registry:       registry,
		SystemPrompt:   "You are helpful.",
		PermissionMode: permissions.ModeDefault,
		WorkingDir:     "/tmp",
		ProjectRoot:    "/tmp",
	})

	if engine.config.PermissionMode != permissions.ModeDefault {
		t.Errorf("PermissionMode = %v, want %v", engine.config.PermissionMode, permissions.ModeDefault)
	}
}

func TestQueryEngineVerbose(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse("ok"))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		Verbose:      true,
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	})

	if !engine.config.Verbose {
		t.Error("expected Verbose to be true in config")
	}
}

func TestQueryEngineInitialMessages(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse("ok"))
	})
	registry := tools.NewRegistry()

	initialMsgs := []api.Message{
		api.UserMessage("previous question"),
		{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "previous answer"}}},
	}

	engine := NewQueryEngine(QueryEngineConfig{
		Client:          client,
		Registry:        registry,
		SystemPrompt:    "You are helpful.",
		InitialMessages: initialMsgs,
		WorkingDir:      "/tmp",
		ProjectRoot:     "/tmp",
	})

	// InitialMessages should be pre-populated in conversation history
	msgs := engine.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 initial messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("first initial message role = %q, want %q", msgs[0].Role, "user")
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("second initial message role = %q, want %q", msgs[1].Role, "assistant")
	}
}

func TestQueryEngineCostTracking(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseResponse("cost test"))
	})
	registry := tools.NewRegistry()

	engine := NewQueryEngine(QueryEngineConfig{
		Client:       client,
		Registry:     registry,
		SystemPrompt: "You are helpful.",
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	})

	ctx := context.Background()

	// Collect events and look for cost update
	hasCostUpdate := false
	for evt := range engine.Ask(ctx, "Hello") {
		if evt.Type == EventCostUpdate {
			hasCostUpdate = true
			// Cost should be non-negative (could be 0 for test model pricing)
			if evt.Cost < 0 {
				t.Errorf("cost should be >= 0, got %v", evt.Cost)
			}
		}
	}

	if !hasCostUpdate {
		t.Error("expected at least one CostUpdate event")
	}

	// SessionCost should match
	sessionCost := engine.SessionCost()
	if sessionCost < 0 {
		t.Errorf("session cost should be >= 0, got %v", sessionCost)
	}
}

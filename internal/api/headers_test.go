package api

import (
	"testing"
)

func TestInjectCustomHeaders_AllFields(t *testing.T) {
	headers := InjectCustomHeaders(RequestHeaders{
		AppName:         "cli",
		UserAgent:       "ClawGo/1.0.0",
		SessionID:       "session-123",
		ContainerID:     "container-456",
		RemoteSessionID: "remote-789",
		ClientApp:       "my-sdk-app",
		ClientRequestID: "req-abc",
	})

	expected := map[string]string{
		"x-app":                        "cli",
		"User-Agent":                   "ClawGo/1.0.0",
		"X-Claude-Code-Session-Id":     "session-123",
		"x-claude-remote-container-id": "container-456",
		"x-claude-remote-session-id":   "remote-789",
		"x-client-app":                 "my-sdk-app",
		"x-client-request-id":          "req-abc",
	}

	for key, want := range expected {
		got, ok := headers[key]
		if !ok {
			t.Errorf("missing header %q", key)
			continue
		}
		if got != want {
			t.Errorf("header %q: got %q, want %q", key, got, want)
		}
	}
}

func TestInjectCustomHeaders_DefaultAppName(t *testing.T) {
	headers := InjectCustomHeaders(RequestHeaders{})

	if headers["x-app"] != "cli" {
		t.Errorf("expected default x-app 'cli', got %q", headers["x-app"])
	}
}

func TestInjectCustomHeaders_ConditionalFields(t *testing.T) {
	// Only set required fields, optional fields should be absent
	headers := InjectCustomHeaders(RequestHeaders{
		SessionID: "test-session",
	})

	// x-app should always be present
	if _, ok := headers["x-app"]; !ok {
		t.Error("x-app should always be present")
	}

	// Session ID should be present
	if _, ok := headers["X-Claude-Code-Session-Id"]; !ok {
		t.Error("session ID should be present")
	}

	// Client request ID should be auto-generated
	if _, ok := headers[ClientRequestIDHeader]; !ok {
		t.Error("client request ID should be auto-generated")
	}

	// Container ID should NOT be present
	if _, ok := headers["x-claude-remote-container-id"]; ok {
		t.Error("container ID should not be present when empty")
	}

	// Remote session ID should NOT be present
	if _, ok := headers["x-claude-remote-session-id"]; ok {
		t.Error("remote session ID should not be present when empty")
	}

	// Client app should NOT be present
	if _, ok := headers["x-client-app"]; ok {
		t.Error("client app should not be present when empty")
	}
}

func TestInjectCustomHeaders_AutoGenerateClientRequestID(t *testing.T) {
	headers := InjectCustomHeaders(RequestHeaders{})

	reqID, ok := headers[ClientRequestIDHeader]
	if !ok {
		t.Fatal("client request ID should be auto-generated")
	}
	if reqID == "" {
		t.Error("auto-generated client request ID should not be empty")
	}
	// Should look like a UUID (36 chars with dashes)
	if len(reqID) != 36 {
		t.Errorf("auto-generated client request ID should be UUID format, got length %d", len(reqID))
	}
}

func TestInjectCustomHeadersAsOptions_ReturnsOptions(t *testing.T) {
	opts := InjectCustomHeadersAsOptions(RequestHeaders{
		AppName:   "cli",
		SessionID: "test-session",
	})

	// Should have at least 3 options: x-app, session ID, client request ID
	if len(opts) < 3 {
		t.Errorf("expected at least 3 options, got %d", len(opts))
	}
}

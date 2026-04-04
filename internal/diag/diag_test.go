package diag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	l, err := NewLogger(logPath, LevelInfo, false)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer l.Close()

	// File should exist
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log file not created: %v", err)
	}

	// Level should be set
	if l.level != LevelInfo {
		t.Errorf("level = %d, want %d", l.level, LevelInfo)
	}
}

func TestLogLevelGating(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "gated.log")

	l, err := NewLogger(logPath, LevelWarn, false)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer l.Close()

	// Debug entry should NOT be written (below Warn threshold)
	l.Debug("should-not-appear", map[string]interface{}{"key": "val"})

	// Info entry should NOT be written either
	l.Info("also-not-appear", nil)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty file for gated entries, got %d bytes: %s", len(data), string(data))
	}
}

func TestLogWritesJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "json.log")

	l, err := NewLogger(logPath, LevelDebug, false)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	l.Info("test-event", map[string]interface{}{
		"count": float64(42),
		"name":  "test",
	})
	l.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("JSON unmarshal: %v (data=%q)", err, string(data))
	}

	if entry.Level != "info" {
		t.Errorf("level = %q, want %q", entry.Level, "info")
	}
	if entry.Event != "test-event" {
		t.Errorf("event = %q, want %q", entry.Event, "test-event")
	}
	if entry.Timestamp.IsZero() {
		t.Error("timestamp is zero")
	}
	if entry.Data["name"] != "test" {
		t.Errorf("data[name] = %v, want %q", entry.Data["name"], "test")
	}
}

func TestFilterPII_Paths(t *testing.T) {
	data := map[string]interface{}{
		"file": "/home/user/project/file.go",
	}
	result := filterPII(data)
	if result["file"] != "[PATH]" {
		t.Errorf("path not filtered: got %q", result["file"])
	}

	// Also test /Users/ path (macOS)
	data2 := map[string]interface{}{
		"file": "/Users/alice/Documents/secret.txt",
	}
	result2 := filterPII(data2)
	if result2["file"] != "[PATH]" {
		t.Errorf("macOS path not filtered: got %q", result2["file"])
	}
}

func TestFilterPII_Emails(t *testing.T) {
	data := map[string]interface{}{
		"contact": "user@example.com",
	}
	result := filterPII(data)
	if result["contact"] != "[EMAIL]" {
		t.Errorf("email not filtered: got %q", result["contact"])
	}
}

func TestFilterPII_IPs(t *testing.T) {
	data := map[string]interface{}{
		"host": "192.168.1.1",
	}
	result := filterPII(data)
	if result["host"] != "[IP]" {
		t.Errorf("IP not filtered: got %q", result["host"])
	}
}

func TestFilterPII_NoPIIMode(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nopii.log")

	l, err := NewLogger(logPath, LevelDebug, true)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	l.Info("pii-test", map[string]interface{}{
		"path":  "/home/user/secret/file.go",
		"email": "admin@corp.com",
		"ip":    "10.0.0.1",
	})
	l.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if entry.Data["path"] != "[PATH]" {
		t.Errorf("path not sanitized: got %q", entry.Data["path"])
	}
	if entry.Data["email"] != "[EMAIL]" {
		t.Errorf("email not sanitized: got %q", entry.Data["email"])
	}
	if entry.Data["ip"] != "[IP]" {
		t.Errorf("ip not sanitized: got %q", entry.Data["ip"])
	}
}

func TestFilterPII_PreservesOriginal(t *testing.T) {
	original := map[string]interface{}{
		"path":  "/home/user/project/main.go",
		"email": "test@test.com",
		"ip":    "172.16.0.1",
		"safe":  "hello world",
	}

	// Keep a copy of original values
	origPath := original["path"]
	origEmail := original["email"]
	origIP := original["ip"]

	result := filterPII(original)

	// Original must NOT be mutated
	if original["path"] != origPath {
		t.Errorf("original path mutated: got %q, want %q", original["path"], origPath)
	}
	if original["email"] != origEmail {
		t.Errorf("original email mutated: got %q, want %q", original["email"], origEmail)
	}
	if original["ip"] != origIP {
		t.Errorf("original ip mutated: got %q, want %q", original["ip"], origIP)
	}

	// Result should be sanitized
	if result["path"] != "[PATH]" {
		t.Errorf("result path = %q, want [PATH]", result["path"])
	}
	if result["email"] != "[EMAIL]" {
		t.Errorf("result email = %q, want [EMAIL]", result["email"])
	}
	if result["ip"] != "[IP]" {
		t.Errorf("result ip = %q, want [IP]", result["ip"])
	}

	// Safe value should pass through unchanged
	if result["safe"] != "hello world" {
		t.Errorf("safe value changed: got %q", result["safe"])
	}
}

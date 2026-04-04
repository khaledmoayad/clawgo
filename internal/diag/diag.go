// Package diag provides structured diagnostic logging with PII filtering.
// It writes JSON-line entries to a file with level gating, matching the
// TypeScript logForDiagnosticsNoPII pattern from utils/diagLogs.ts.
package diag

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
)

// Level represents the severity of a diagnostic log entry.
type Level int

const (
	// LevelDebug is the most verbose level, for detailed troubleshooting.
	LevelDebug Level = 0
	// LevelInfo is for general informational messages.
	LevelInfo Level = 1
	// LevelWarn is for warning conditions that may need attention.
	LevelWarn Level = 2
	// LevelError is for error conditions.
	LevelError Level = 3
)

// levelNames maps Level values to their string representation for JSON output.
var levelNames = map[Level]string{
	LevelDebug: "debug",
	LevelInfo:  "info",
	LevelWarn:  "warn",
	LevelError: "error",
}

// PII-matching regular expressions compiled once at package init.
var (
	// Matches absolute paths: /home/..., /Users/..., /tmp/..., C:\..., etc.
	pathRe = regexp.MustCompile(`(?:/(?:home|Users|tmp|var|root)/\S+|[A-Z]:\\[^\s"]+)`)

	// Matches email addresses.
	emailRe = regexp.MustCompile(`\b\S+@\S+\.\S+\b`)

	// Matches IPv4 addresses.
	ipRe = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
)

// Entry is a single diagnostic log record.
type Entry struct {
	Timestamp time.Time              `json:"ts"`
	Level     string                 `json:"level"`
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Logger writes structured JSON diagnostic entries to a file.
type Logger struct {
	mu    sync.Mutex
	file  *os.File
	level Level
	noPII bool
}

// defaultLogger is the package-level singleton logger.
var (
	defaultMu     sync.RWMutex
	defaultLogger *Logger
)

// SetDefault sets the package-level default logger.
func SetDefault(l *Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultLogger = l
}

// Default returns the package-level default logger, or nil if not set.
func Default() *Logger {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultLogger
}

// NewLogger creates a new diagnostic logger that writes JSON lines to logPath.
// The level parameter gates which entries are written (entries below the level
// are silently dropped). When noPII is true, all string values in data maps
// are scrubbed of file paths, email addresses, and IP addresses before writing.
func NewLogger(logPath string, level Level, noPII bool) (*Logger, error) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("diag: open log file: %w", err)
	}

	return &Logger{
		file:  f,
		level: level,
		noPII: noPII,
	}, nil
}

// Log writes a structured JSON entry to the log file if the entry's level
// meets or exceeds the logger's configured level. When noPII mode is active,
// string values in the data map are sanitized before writing.
func (l *Logger) Log(level Level, event string, data map[string]interface{}) {
	if level < l.level {
		return
	}

	// Build entry data -- sanitize if noPII
	entryData := data
	if l.noPII && data != nil {
		entryData = filterPII(data)
	}

	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     levelNames[level],
		Event:     event,
		Data:      entryData,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return // silently drop malformed entries
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		_, _ = l.file.Write(append(line, '\n'))
	}
}

// Debug logs at LevelDebug.
func (l *Logger) Debug(event string, data map[string]interface{}) {
	l.Log(LevelDebug, event, data)
}

// Info logs at LevelInfo.
func (l *Logger) Info(event string, data map[string]interface{}) {
	l.Log(LevelInfo, event, data)
}

// Warn logs at LevelWarn.
func (l *Logger) Warn(event string, data map[string]interface{}) {
	l.Log(LevelWarn, event, data)
}

// Error logs at LevelError.
func (l *Logger) Error(event string, data map[string]interface{}) {
	l.Log(LevelError, event, data)
}

// Close flushes and closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// filterPII returns a sanitized copy of data with PII-containing strings
// replaced. The original map is never mutated.
func filterPII(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(data))
	for k, v := range data {
		switch val := v.(type) {
		case string:
			result[k] = sanitizeString(val)
		case map[string]interface{}:
			result[k] = filterPII(val)
		default:
			result[k] = v
		}
	}
	return result
}

// sanitizeString replaces PII patterns in a string.
func sanitizeString(s string) string {
	s = pathRe.ReplaceAllString(s, "[PATH]")
	s = emailRe.ReplaceAllString(s, "[EMAIL]")
	s = ipRe.ReplaceAllString(s, "[IP]")
	return s
}

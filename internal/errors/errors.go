package errors

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// ClawGoError is the base error type for all ClawGo errors.
// It mirrors the TypeScript ClaudeError hierarchy.
type ClawGoError struct {
	Name    string
	Message string
	Err     error
}

// Error implements the error interface.
func (e *ClawGoError) Error() string {
	if e.Err != nil {
		if e.Message != "" {
			return fmt.Sprintf("%s: %s: %v", e.Name, e.Message, e.Err)
		}
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Name, e.Message)
	}
	return e.Name
}

// Unwrap returns the wrapped error for errors.Is/As support.
func (e *ClawGoError) Unwrap() error {
	return e.Err
}

// New creates a new ClawGoError with a name and message.
func New(name, message string) *ClawGoError {
	return &ClawGoError{
		Name:    name,
		Message: message,
	}
}

// Wrap creates a new ClawGoError wrapping an existing error.
func Wrap(name string, err error) *ClawGoError {
	return &ClawGoError{
		Name: name,
		Err:  err,
	}
}

// Specific error types for different domains.

// ConfigError represents configuration-related errors.
type ConfigError struct {
	ClawGoError
}

// NewConfigError creates a new ConfigError.
func NewConfigError(message string) *ConfigError {
	return &ConfigError{ClawGoError{Name: "ConfigError", Message: message}}
}

// APIError represents API-related errors.
type APIError struct {
	ClawGoError
	StatusCode int
}

// NewAPIError creates a new APIError with an HTTP status code.
func NewAPIError(message string, statusCode int) *APIError {
	return &APIError{
		ClawGoError: ClawGoError{Name: "APIError", Message: message},
		StatusCode:  statusCode,
	}
}

// ToolError represents tool execution errors.
type ToolError struct {
	ClawGoError
	ToolName string
}

// NewToolError creates a new ToolError.
func NewToolError(toolName, message string) *ToolError {
	return &ToolError{
		ClawGoError: ClawGoError{Name: "ToolError", Message: message},
		ToolName:    toolName,
	}
}

// PermissionError represents permission-related errors.
type PermissionError struct {
	ClawGoError
}

// NewPermissionError creates a new PermissionError.
func NewPermissionError(message string) *PermissionError {
	return &PermissionError{ClawGoError{Name: "PermissionError", Message: message}}
}

// SessionError represents session-related errors.
type SessionError struct {
	ClawGoError
}

// NewSessionError creates a new SessionError.
func NewSessionError(message string) *SessionError {
	return &SessionError{ClawGoError{Name: "SessionError", Message: message}}
}

// ShellError represents shell/subprocess execution errors.
// Matches TS ShellError which carries an exit code.
type ShellError struct {
	ClawGoError
	ExitCode int
}

// NewShellError creates a new ShellError with a message and exit code.
func NewShellError(message string, exitCode int) *ShellError {
	return &ShellError{
		ClawGoError: ClawGoError{Name: "ShellError", Message: message},
		ExitCode:    exitCode,
	}
}

// AbortError represents user or system-initiated abort.
// Matches TS AbortError.
type AbortError struct {
	ClawGoError
}

// NewAbortError creates a new AbortError with a message.
func NewAbortError(message string) *AbortError {
	return &AbortError{ClawGoError{Name: "AbortError", Message: message}}
}

// NewAbortErrorFrom creates an AbortError wrapping an existing error
// (e.g. context.Canceled).
func NewAbortErrorFrom(err error) *AbortError {
	return &AbortError{ClawGoError{Name: "AbortError", Err: err}}
}

// FallbackTriggeredError triggers model fallback in the query loop.
// Matches TS FallbackTriggeredError.
type FallbackTriggeredError struct {
	ClawGoError
	FallbackModel string
}

// NewFallbackTriggeredError creates a FallbackTriggeredError for the given model.
func NewFallbackTriggeredError(fallbackModel string) *FallbackTriggeredError {
	return &FallbackTriggeredError{
		ClawGoError:   ClawGoError{Name: "FallbackTriggeredError", Message: "falling back to " + fallbackModel},
		FallbackModel: fallbackModel,
	}
}

// TeleportError represents teleport operation failures.
// Matches TS TeleportOperationError.
type TeleportError struct {
	ClawGoError
}

// NewTeleportError creates a new TeleportError.
func NewTeleportError(message string) *TeleportError {
	return &TeleportError{ClawGoError{Name: "TeleportError", Message: message}}
}

// OAuthError represents OAuth authentication failures.
type OAuthError struct {
	ClawGoError
}

// NewOAuthError creates a new OAuthError.
func NewOAuthError(message string) *OAuthError {
	return &OAuthError{ClawGoError{Name: "OAuthError", Message: message}}
}

// MalformedCommandError represents invalid user command input.
// Matches TS MalformedCommandError.
type MalformedCommandError struct {
	ClawGoError
}

// NewMalformedCommandError creates a new MalformedCommandError.
func NewMalformedCommandError(message string) *MalformedCommandError {
	return &MalformedCommandError{ClawGoError{Name: "MalformedCommandError", Message: message}}
}

// ConfigParseError represents configuration parsing failures.
// Matches TS ConfigParseError.
type ConfigParseError struct {
	ClawGoError
}

// NewConfigParseError creates a new ConfigParseError.
func NewConfigParseError(message string) *ConfigParseError {
	return &ConfigParseError{ClawGoError{Name: "ConfigParseError", Message: message}}
}

// --- Utility functions matching TS patterns ---

// IsAbortError checks if err is an AbortError, context.Canceled,
// or context.DeadlineExceeded. Matches TS isAbortError().
func IsAbortError(err error) bool {
	if err == nil {
		return false
	}
	var ae *AbortError
	if errors.As(err, &ae) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

// messageProvider is implemented by all error types that embed ClawGoError.
type messageProvider interface {
	GetMessage() string
}

// GetMessage returns the Message field. This allows ErrorMessage to work
// with all error types that embed ClawGoError, since Go's errors.As
// does not traverse struct embedding for value-embedded types.
func (e *ClawGoError) GetMessage() string {
	return e.Message
}

// ErrorMessage extracts a clean message string from any error.
// For ClawGoError types (and types embedding it), returns the Message field.
// For other errors, returns Error(). For nil, returns "".
// Matches TS errorMessage().
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if mp, ok := err.(messageProvider); ok {
		return mp.GetMessage()
	}
	return err.Error()
}

// ToError converts an arbitrary value to an error.
// Strings become errors, nil becomes nil, errors pass through.
// Other types are formatted with fmt.Sprintf.
// Matches TS toError().
func ToError(v interface{}) error {
	if v == nil {
		return nil
	}
	if err, ok := v.(error); ok {
		return err
	}
	if s, ok := v.(string); ok {
		return fmt.Errorf("%s", s)
	}
	return fmt.Errorf("%v", v)
}

// IsENOENT checks if err is a file-not-found error.
// Matches TS isENOENT(). Uses os.ErrNotExist for detection.
func IsENOENT(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrNotExist)
}

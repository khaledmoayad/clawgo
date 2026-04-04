package errors

import "fmt"

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
		return fmt.Sprintf("%s: %s: %v", e.Name, e.Message, e.Err)
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

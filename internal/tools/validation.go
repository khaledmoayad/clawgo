package tools

import (
	"encoding/json"
	"fmt"
)

// Validatable is implemented by tool input structs that can self-validate.
// If a target passed to ValidateInput implements this interface,
// Validate() is called after JSON unmarshaling.
type Validatable interface {
	Validate() error
}

// ValidateInput unmarshals JSON input into the target struct.
// Returns a descriptive error if JSON is invalid or required fields are missing.
// If the target implements Validatable, its Validate() method is called.
func ValidateInput(input json.RawMessage, target any) error {
	if err := json.Unmarshal(input, target); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	if v, ok := target.(Validatable); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// RequireString extracts a string field from a map and errors if missing or empty.
func RequireString(data map[string]any, key string) (string, error) {
	v, ok := data[key]
	if !ok {
		return "", fmt.Errorf("required field %q is missing or empty", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("required field %q is missing or empty", key)
	}
	return s, nil
}

// OptionalString extracts a string field from a map, returns defaultVal if missing.
func OptionalString(data map[string]any, key, defaultVal string) string {
	v, ok := data[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}

// OptionalInt extracts an int field from a map, returns defaultVal if missing.
// JSON numbers are unmarshaled as float64, so we handle that conversion.
func OptionalInt(data map[string]any, key string, defaultVal int) int {
	v, ok := data[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}

// OptionalBool extracts a bool field from a map, returns defaultVal if missing.
func OptionalBool(data map[string]any, key string, defaultVal bool) bool {
	v, ok := data[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}

// ParseRawInput parses json.RawMessage into a map for flexible field access.
func ParseRawInput(input json.RawMessage) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal(input, &data); err != nil {
		return nil, fmt.Errorf("invalid tool input: %w", err)
	}
	return data, nil
}

package tools

import (
	"fmt"
	"regexp"
	"strconv"
)

// numberRegex matches valid decimal number literals: integers and decimals,
// optionally negative. Matches the TypeScript /^-?\d+(\.\d+)?$/ exactly.
var numberRegex = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// SemanticBoolean coerces model-generated string booleans to native bool.
// Accepts: bool values directly, string "true"/"false" (case-sensitive).
// Rejects: any other string (unlike Go's strconv.ParseBool which accepts "1", "t", "yes").
// This matches Claude Code's z.preprocess behavior where only literal "true"/"false" are coerced.
func SemanticBoolean(v interface{}) (bool, error) {
	if v == nil {
		return false, fmt.Errorf("semantic boolean: expected bool or \"true\"/\"false\" string, got nil")
	}
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		switch val {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return false, fmt.Errorf("semantic boolean: expected \"true\" or \"false\", got %q", val)
		}
	default:
		return false, fmt.Errorf("semantic boolean: expected bool or string, got %T", v)
	}
}

// SemanticNumber coerces model-generated string numbers to float64.
// Accepts: float64/int/int64 values directly, strings matching ^-?\d+(\.\d+)?$.
// Rejects: empty string, non-numeric strings, nil.
// This matches Claude Code's z.preprocess behavior with strict regex validation.
func SemanticNumber(v interface{}) (float64, error) {
	if v == nil {
		return 0, fmt.Errorf("semantic number: expected number or numeric string, got nil")
	}
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		if !numberRegex.MatchString(val) {
			return 0, fmt.Errorf("semantic number: %q is not a valid number", val)
		}
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, fmt.Errorf("semantic number: failed to parse %q: %w", val, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("semantic number: expected number or string, got %T", v)
	}
}

// OptionalSemanticBool extracts a bool field from a map with semantic coercion.
// Returns defaultVal if the key is not present.
func OptionalSemanticBool(data map[string]any, key string, defaultVal bool) (bool, error) {
	v, ok := data[key]
	if !ok {
		return defaultVal, nil
	}
	return SemanticBoolean(v)
}

// OptionalSemanticNumber extracts a number field from a map with semantic coercion.
// Returns defaultVal if the key is not present.
func OptionalSemanticNumber(data map[string]any, key string, defaultVal float64) (float64, error) {
	v, ok := data[key]
	if !ok {
		return defaultVal, nil
	}
	return SemanticNumber(v)
}

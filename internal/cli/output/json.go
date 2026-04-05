package output

import "encoding/json"

// FormatJSON produces the json output format.
// With verbose=true and a non-empty allMessages slice, returns the full
// messages array (matching Claude Code's --verbose json behavior).
// Otherwise returns just the final result message.
func FormatJSON(result *ResultMessage, verbose bool, allMessages []any) ([]byte, error) {
	if verbose && len(allMessages) > 0 {
		return json.Marshal(allMessages)
	}
	return json.Marshal(result)
}

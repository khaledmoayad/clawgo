package swarm

import (
	"fmt"
	"strings"
)

// FormatAgentID creates a deterministic agent ID in the format agentName@teamName.
// This matches the TypeScript agentId.ts format. Deterministic IDs enable:
//   - Reconnection after crashes/restarts (same name+team = same ID)
//   - Human-readable, debuggable identifiers (e.g., "tester@my-project")
//   - Predictable routing (leader can compute teammate's ID without lookup)
//
// Agent names must NOT contain '@' -- use SanitizeAgentName first.
func FormatAgentID(agentName, teamName string) string {
	return agentName + "@" + teamName
}

// ParseAgentID splits an agent ID into its agentName and teamName components.
// Returns empty strings and false if the ID doesn't contain the '@' separator.
func ParseAgentID(agentID string) (agentName, teamName string, ok bool) {
	idx := strings.Index(agentID, "@")
	if idx == -1 {
		return "", "", false
	}
	return agentID[:idx], agentID[idx+1:], true
}

// SanitizeAgentName strips '@' characters from an agent name since '@' is
// used as the separator in agent IDs (agentName@teamName).
func SanitizeAgentName(name string) string {
	return strings.ReplaceAll(name, "@", "")
}

// FormatRequestID creates a request ID in the format {requestType}-{timestamp}@{agentID}.
// Used for shutdown requests, plan approvals, etc. This is distinct from
// permission_sync.go's GenerateRequestID which creates permission-specific IDs.
func FormatRequestID(requestType string, agentID string, timestampMs int64) string {
	return fmt.Sprintf("%s-%d@%s", requestType, timestampMs, agentID)
}

// ParseAgentRequestID splits a request ID into its requestType, timestamp, and agentID.
// Format: {requestType}-{timestamp}@{agentID}
// Returns zero values and false if the format doesn't match.
// This is distinct from permission_sync.go's GenerateRequestID format.
func ParseAgentRequestID(requestID string) (requestType string, timestampMs int64, agentID string, ok bool) {
	atIdx := strings.Index(requestID, "@")
	if atIdx == -1 {
		return "", 0, "", false
	}

	prefix := requestID[:atIdx]
	agentID = requestID[atIdx+1:]

	lastDash := strings.LastIndex(prefix, "-")
	if lastDash == -1 {
		return "", 0, "", false
	}

	requestType = prefix[:lastDash]
	timestampStr := prefix[lastDash+1:]

	var ts int64
	n, err := fmt.Sscanf(timestampStr, "%d", &ts)
	if err != nil || n != 1 {
		return "", 0, "", false
	}

	return requestType, ts, agentID, true
}

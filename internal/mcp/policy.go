package mcp

import "github.com/khaledmoayad/clawgo/internal/config"

// PolicyDecision represents the result of evaluating enterprise allow/deny
// policy for an MCP server.
type PolicyDecision string

const (
	// PolicyAllowed means the server is permitted to connect.
	PolicyAllowed PolicyDecision = "allowed"

	// PolicyDenied means the server is explicitly blocked by policy.
	PolicyDenied PolicyDecision = "denied"

	// PolicyDisabled means the server's config has Disabled set to true.
	PolicyDisabled PolicyDecision = "disabled"
)

// EvaluateServerPolicy checks enterprise allow/deny policy for a named MCP
// server. The rules are applied in order:
//
//  1. If DeniedMCPServers contains the server name, deny.
//  2. If AllowedMCPServers is non-empty and does NOT contain the server name, deny.
//  3. Otherwise, allow.
//
// This mirrors the TypeScript settings-driven policy evaluation where
// enterprise/MDM settings gate which MCP servers are surfaced to the runtime.
func EvaluateServerPolicy(name string, settings *config.Settings) PolicyDecision {
	if settings == nil {
		return PolicyAllowed
	}

	// Rule 1: explicit deny list takes priority
	for _, denied := range settings.DeniedMCPServers {
		if denied == name {
			return PolicyDenied
		}
	}

	// Rule 2: if an allow list is present, the server must appear in it
	if len(settings.AllowedMCPServers) > 0 {
		for _, allowed := range settings.AllowedMCPServers {
			if allowed == name {
				return PolicyAllowed
			}
		}
		// Non-empty allow list and server not found -- deny
		return PolicyDenied
	}

	// Rule 3: no deny match and no allow list restriction -- allow
	return PolicyAllowed
}

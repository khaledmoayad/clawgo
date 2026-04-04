package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
)

// settingsWrapper is used to extract the mcpServers field from settings JSON.
type settingsWrapper struct {
	MCPServers json.RawMessage `json:"mcpServers"`
}

// serverConfigRaw matches the per-server config shape in the map format.
type serverConfigRaw struct {
	Type    MCPTransportType  `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// LoadMCPConfig parses the mcpServers field from settings JSON into a slice
// of MCPServerConfig. It supports both the map format (matching the TypeScript
// original where keys are server names) and a direct array format.
func LoadMCPConfig(configJSON json.RawMessage) ([]MCPServerConfig, error) {
	var wrapper settingsWrapper
	if err := json.Unmarshal(configJSON, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing settings: %w", err)
	}

	if len(wrapper.MCPServers) == 0 || string(wrapper.MCPServers) == "null" {
		return nil, nil
	}

	// Try map format first: {"server-name": {...config...}}
	var serverMap map[string]serverConfigRaw
	if err := json.Unmarshal(wrapper.MCPServers, &serverMap); err == nil {
		configs := make([]MCPServerConfig, 0, len(serverMap))
		for name, raw := range serverMap {
			configs = append(configs, MCPServerConfig{
				Name:    name,
				Type:    raw.Type,
				Command: raw.Command,
				Args:    raw.Args,
				Env:     raw.Env,
				URL:     raw.URL,
				Headers: raw.Headers,
			})
		}
		// Sort by name for deterministic ordering
		sort.Slice(configs, func(i, j int) bool {
			return configs[i].Name < configs[j].Name
		})
		return configs, nil
	}

	// Try array format: [{name: "...", ...}]
	var configs []MCPServerConfig
	if err := json.Unmarshal(wrapper.MCPServers, &configs); err != nil {
		return nil, fmt.Errorf("parsing mcpServers: %w", err)
	}
	return configs, nil
}

package mcp

import (
	"context"
	"sync"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MaxDiscoveryDescriptionLength caps tool/prompt descriptions stored in the
// discovery cache. OpenAPI-generated MCP servers can dump 15-60 KB of endpoint
// docs; this caps the p95 tail without losing the intent.
const MaxDiscoveryDescriptionLength = 2048

// DiscoveredTool represents a tool reported by a connected MCP server,
// stored with its original and normalized names.
type DiscoveredTool struct {
	// OriginalName is the name as reported by the server.
	OriginalName string
	// NormalizedName is the Claude Code-compatible mcp__<server>__<tool> name.
	NormalizedName string
	// Description is the tool description, truncated to MaxDiscoveryDescriptionLength.
	Description string
	// InputSchema is the raw JSON schema for the tool's input parameters.
	InputSchema map[string]any
}

// DiscoveredResource represents a resource reported by a connected MCP server.
type DiscoveredResource struct {
	// ServerName is the normalized server name that provides this resource.
	ServerName string
	// URI is the resource URI as reported by the server.
	URI string
	// Name is the human-readable resource name.
	Name string
	// Title is the optional human-friendly title.
	Title string
	// Description is the resource description, truncated to MaxDiscoveryDescriptionLength.
	Description string
	// MIMEType is the MIME type of the resource, if known.
	MIMEType string
}

// DiscoveredPrompt represents a prompt reported by a connected MCP server.
type DiscoveredPrompt struct {
	// OriginalName is the prompt name as reported by the server.
	OriginalName string
	// NormalizedName is the Claude Code-compatible mcp__<server>__<prompt> name.
	NormalizedName string
	// Description is the prompt description, truncated to MaxDiscoveryDescriptionLength.
	Description string
	// Arguments describes the prompt's expected parameters.
	Arguments []*gomcp.PromptArgument
}

// discoveryCache stores the cached discovery results for a ConnectedServer.
type discoveryCache struct {
	mu        sync.RWMutex
	tools     []DiscoveredTool
	resources []DiscoveredResource
	prompts   []DiscoveredPrompt
}

// RefreshDiscovery queries the live MCP session for tools, resources, and
// prompts, normalizes names, truncates descriptions, and caches the results
// on the ConnectedServer. It is safe to call concurrently.
func (cs *ConnectedServer) RefreshDiscovery(ctx context.Context) error {
	if cs.session == nil {
		return nil
	}

	serverName := cs.Config.Name

	// --- Tools (always available) ---
	toolResult, err := cs.session.ListTools(ctx, nil)
	if err != nil {
		return err
	}
	discoveredTools := make([]DiscoveredTool, 0, len(toolResult.Tools))
	for _, t := range toolResult.Tools {
		dt := DiscoveredTool{
			OriginalName:   t.Name,
			NormalizedName: NormalizeToolName(serverName, t.Name),
			Description:    truncateDescription(t.Description),
		}
		if schemaMap, ok := t.InputSchema.(map[string]any); ok {
			dt.InputSchema = schemaMap
		}
		discoveredTools = append(discoveredTools, dt)
	}

	// Update the legacy tools cache too
	cs.tools = toolResult.Tools
	cs.normalizedTools = make(map[string]string, len(discoveredTools))
	for _, dt := range discoveredTools {
		cs.normalizedTools[dt.NormalizedName] = dt.OriginalName
	}

	// --- Resources (only if server supports them) ---
	var discoveredResources []DiscoveredResource
	if caps := cs.serverCapabilities(); caps != nil && caps.Resources != nil {
		resResult, err := cs.session.ListResources(ctx, nil)
		if err == nil && resResult != nil {
			discoveredResources = make([]DiscoveredResource, 0, len(resResult.Resources))
			for _, r := range resResult.Resources {
				dr := DiscoveredResource{
					ServerName:  NormalizeServerName(serverName),
					URI:         r.URI,
					Name:        r.Name,
					Title:       r.Title,
					Description: truncateDescription(r.Description),
					MIMEType:    r.MIMEType,
				}
				discoveredResources = append(discoveredResources, dr)
			}
		}
	}

	// --- Prompts (only if server supports them) ---
	var discoveredPrompts []DiscoveredPrompt
	if caps := cs.serverCapabilities(); caps != nil && caps.Prompts != nil {
		promptResult, err := cs.session.ListPrompts(ctx, nil)
		if err == nil && promptResult != nil {
			discoveredPrompts = make([]DiscoveredPrompt, 0, len(promptResult.Prompts))
			for _, p := range promptResult.Prompts {
				dp := DiscoveredPrompt{
					OriginalName:   p.Name,
					NormalizedName: NormalizePromptName(serverName, p.Name),
					Description:    truncateDescription(p.Description),
					Arguments:      p.Arguments,
				}
				discoveredPrompts = append(discoveredPrompts, dp)
			}
		}
	}

	// Atomically update the cache
	cs.discovery.mu.Lock()
	cs.discovery.tools = discoveredTools
	cs.discovery.resources = discoveredResources
	cs.discovery.prompts = discoveredPrompts
	cs.discovery.mu.Unlock()

	return nil
}

// ListResources returns the cached discovered resources for this server.
func (cs *ConnectedServer) ListResources(ctx context.Context) ([]DiscoveredResource, error) {
	cs.discovery.mu.RLock()
	defer cs.discovery.mu.RUnlock()
	result := make([]DiscoveredResource, len(cs.discovery.resources))
	copy(result, cs.discovery.resources)
	return result, nil
}

// ReadResource reads a resource from the live MCP session by URI.
func (cs *ConnectedServer) ReadResource(ctx context.Context, uri string) (*gomcp.ReadResourceResult, error) {
	if cs.session == nil {
		return nil, errNoSession
	}
	return cs.session.ReadResource(ctx, &gomcp.ReadResourceParams{URI: uri})
}

// ListPrompts returns the cached discovered prompts for this server.
func (cs *ConnectedServer) ListPrompts(ctx context.Context) ([]DiscoveredPrompt, error) {
	cs.discovery.mu.RLock()
	defer cs.discovery.mu.RUnlock()
	result := make([]DiscoveredPrompt, len(cs.discovery.prompts))
	copy(result, cs.discovery.prompts)
	return result, nil
}

// DiscoveredTools returns the cached discovered tools for this server.
func (cs *ConnectedServer) DiscoveredTools() []DiscoveredTool {
	cs.discovery.mu.RLock()
	defer cs.discovery.mu.RUnlock()
	result := make([]DiscoveredTool, len(cs.discovery.tools))
	copy(result, cs.discovery.tools)
	return result
}

// serverCapabilities returns the server's declared capabilities from the
// initialize handshake, or nil if not available.
func (cs *ConnectedServer) serverCapabilities() *gomcp.ServerCapabilities {
	if cs.session == nil {
		return nil
	}
	initResult := cs.session.InitializeResult()
	if initResult == nil {
		return nil
	}
	return initResult.Capabilities
}

// truncateDescription truncates a description to MaxDiscoveryDescriptionLength.
func truncateDescription(desc string) string {
	if len(desc) <= MaxDiscoveryDescriptionLength {
		return desc
	}
	return desc[:MaxDiscoveryDescriptionLength]
}

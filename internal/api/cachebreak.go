package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CacheBreakReason describes what changed between API requests that would
// invalidate the prompt cache. Matches Claude Code's cache break detection.
type CacheBreakReason string

const (
	CacheBreakSystem       CacheBreakReason = "system_changed"
	CacheBreakTools        CacheBreakReason = "tools_changed"
	CacheBreakModel        CacheBreakReason = "model_changed"
	CacheBreakBetas        CacheBreakReason = "betas_changed"
	CacheBreakEffort       CacheBreakReason = "effort_changed"
	CacheBreakCacheControl CacheBreakReason = "cache_control_changed"
)

// CacheBreakReport describes what changed between two consecutive API requests
// that would cause a prompt cache miss.
type CacheBreakReport struct {
	// Reasons lists all detected cache-breaking changes.
	Reasons []CacheBreakReason

	// Details provides human-readable descriptions of each change.
	Details []string

	// ChangedTools lists the names of tools whose schemas changed
	// (only populated when CacheBreakTools is in Reasons).
	ChangedTools []string
}

// HasBreak returns true if any cache-breaking change was detected.
func (r *CacheBreakReport) HasBreak() bool {
	return len(r.Reasons) > 0
}

// CacheBreakDetector tracks the state of cache-affecting request parameters
// across consecutive API calls and detects when changes would invalidate the
// prompt cache. This mirrors Claude Code's cache break detection logic.
type CacheBreakDetector struct {
	// Hashes of the previous request's cache-affecting parameters
	prevSystemHash  string
	prevToolsHash   string
	prevModel       string
	prevBetas       string
	prevEffort      string
	prevCacheCtrl   bool

	// Per-tool schema hashes for identifying which specific tool changed
	prevToolHashes map[string]string

	// CacheReadTokens tracks the cache read tokens from the last response
	// to validate detection accuracy.
	CacheReadTokens int

	// initialized tracks whether we have a previous state to compare against.
	initialized bool
}

// NewCacheBreakDetector creates a new detector with no prior state.
func NewCacheBreakDetector() *CacheBreakDetector {
	return &CacheBreakDetector{
		prevToolHashes: make(map[string]string),
	}
}

// CacheBreakParams captures the cache-affecting parameters from an API request.
type CacheBreakParams struct {
	// SystemBlocks are the system prompt text blocks (hashed for comparison).
	SystemBlocks []string

	// Tools are the tool definitions as JSON blobs (name -> schema hash).
	Tools map[string]json.RawMessage

	// Model is the model name.
	Model string

	// Betas is the sorted list of beta header values.
	Betas []string

	// Effort is the effort level string.
	Effort string

	// CacheControlEnabled indicates whether cache control is enabled.
	CacheControlEnabled bool
}

// DetectBreak compares the current request parameters against the previous
// state and returns a report listing what changed. After detection, the
// current parameters become the new "previous" state.
func (d *CacheBreakDetector) DetectBreak(params CacheBreakParams) *CacheBreakReport {
	report := &CacheBreakReport{}

	// Compute current hashes
	systemHash := hashStringSlice(params.SystemBlocks)
	toolsHash := hashToolSchemas(params.Tools)
	betasStr := sortedJoin(params.Betas)

	// Compute per-tool hashes
	currentToolHashes := make(map[string]string, len(params.Tools))
	for name, schema := range params.Tools {
		currentToolHashes[name] = hashBytes(schema)
	}

	if d.initialized {
		// Compare system prompt
		if systemHash != d.prevSystemHash {
			report.Reasons = append(report.Reasons, CacheBreakSystem)
			report.Details = append(report.Details, "System prompt content changed")
		}

		// Compare tools
		if toolsHash != d.prevToolsHash {
			report.Reasons = append(report.Reasons, CacheBreakTools)
			report.Details = append(report.Details, "Tool definitions changed")
			report.ChangedTools = findChangedTools(d.prevToolHashes, currentToolHashes)
		}

		// Compare model
		if params.Model != d.prevModel {
			report.Reasons = append(report.Reasons, CacheBreakModel)
			report.Details = append(report.Details, fmt.Sprintf("Model changed from %q to %q", d.prevModel, params.Model))
		}

		// Compare betas
		if betasStr != d.prevBetas {
			report.Reasons = append(report.Reasons, CacheBreakBetas)
			report.Details = append(report.Details, "Beta headers changed")
		}

		// Compare effort
		if params.Effort != d.prevEffort {
			report.Reasons = append(report.Reasons, CacheBreakEffort)
			report.Details = append(report.Details, fmt.Sprintf("Effort level changed from %q to %q", d.prevEffort, params.Effort))
		}

		// Compare cache control
		if params.CacheControlEnabled != d.prevCacheCtrl {
			report.Reasons = append(report.Reasons, CacheBreakCacheControl)
			report.Details = append(report.Details, "Cache control setting changed")
		}
	}

	// Update state for next comparison
	d.prevSystemHash = systemHash
	d.prevToolsHash = toolsHash
	d.prevModel = params.Model
	d.prevBetas = betasStr
	d.prevEffort = params.Effort
	d.prevCacheCtrl = params.CacheControlEnabled
	d.prevToolHashes = currentToolHashes
	d.initialized = true

	return report
}

// RecordCacheReadTokens records the cache read token count from an API response.
// This is used to validate the accuracy of cache break detection -- if we
// predicted no break but got zero cache reads, something unexpected changed.
func (d *CacheBreakDetector) RecordCacheReadTokens(tokens int) {
	d.CacheReadTokens = tokens
}

// --- DJB2 hash function ---

// djb2Hash implements the DJB2 hash function for string hashing, matching
// Claude Code's djb2Hash() implementation. Returns a hex string.
func djb2Hash(s string) string {
	var hash uint32 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint32(s[i])
	}
	return fmt.Sprintf("%08x", hash)
}

// --- Internal helpers ---

// hashStringSlice computes a hash of a string slice by joining and hashing.
func hashStringSlice(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return djb2Hash(strings.Join(ss, "\x00"))
}

// hashToolSchemas computes a composite hash of all tool schemas.
func hashToolSchemas(tools map[string]json.RawMessage) string {
	if len(tools) == 0 {
		return ""
	}
	// Sort tool names for deterministic hashing
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write(tools[name])
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}

// hashBytes computes a SHA-256 hash of raw bytes, truncated to 16 hex chars.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

// sortedJoin sorts a string slice and joins with commas.
func sortedJoin(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	sorted := make([]string, len(ss))
	copy(sorted, ss)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// findChangedTools compares two per-tool hash maps and returns the names of
// tools that were added, removed, or had their schema change.
func findChangedTools(prev, current map[string]string) []string {
	var changed []string

	// Check for changed or removed tools
	for name, prevHash := range prev {
		curHash, exists := current[name]
		if !exists || curHash != prevHash {
			changed = append(changed, name)
		}
	}

	// Check for added tools
	for name := range current {
		if _, exists := prev[name]; !exists {
			changed = append(changed, name)
		}
	}

	sort.Strings(changed)
	return changed
}

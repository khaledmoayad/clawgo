// Package memory provides session memory extraction, persistence, and recall for ClawGo.
// Memories are extracted from conversations exceeding token/tool-call thresholds and
// persisted as markdown files in ~/.claude/session-memory/, matching the TypeScript
// SessionMemory behavior.
package memory

import (
	"path/filepath"

	"github.com/khaledmoayad/clawgo/internal/config"
)

// MemoryConfig controls session memory extraction and persistence behavior.
type MemoryConfig struct {
	// Enabled controls whether memory extraction runs after conversations.
	Enabled bool

	// MinTokensForExtraction is the minimum token count required before memory
	// extraction is triggered. Conversations below this threshold are too short
	// to contain meaningful context worth preserving.
	MinTokensForExtraction int

	// MinToolCallsForExtraction is the minimum number of tool calls required
	// before memory extraction is triggered. Conversations with fewer tool calls
	// typically lack enough interaction context to extract useful memories.
	MinToolCallsForExtraction int

	// MemoryDir is the directory where memory files are stored.
	MemoryDir string
}

// DefaultMemoryConfig returns the default memory configuration.
// Thresholds match the TypeScript behavior: only extract memory from conversations
// that had significant interaction (enough tokens and tool calls to have meaningful
// context worth preserving).
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		Enabled:                   true,
		MinTokensForExtraction:    10000,
		MinToolCallsForExtraction: 5,
		MemoryDir:                 filepath.Join(config.ConfigDir(), "session-memory"),
	}
}

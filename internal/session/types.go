// Package session provides session persistence and command history for ClawGo.
// types.go defines all JSONL entry types matching Claude Code's types/logs.ts,
// including the UUID chain (uuid + parentUuid) for transcript messages.
package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
)

// --- Core transcript types ---

// SerializedMessage extends a message with session metadata.
// Matches Claude Code's SerializedMessage type from types/logs.ts.
type SerializedMessage struct {
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	CWD        string          `json:"cwd"`
	UserType   string          `json:"userType"`
	Entrypoint string          `json:"entrypoint,omitempty"`
	SessionID  string          `json:"sessionId"`
	Timestamp  string          `json:"timestamp"`
	Version    string          `json:"version"`
	GitBranch  string          `json:"gitBranch,omitempty"`
	Slug       string          `json:"slug,omitempty"`
}

// TranscriptMessage extends SerializedMessage with UUID chain fields.
// Matches Claude Code's TranscriptMessage type from types/logs.ts.
// The UUID chain links messages via parentUuid, enabling conversation
// tree reconstruction for resume, branching, and sidechain support.
type TranscriptMessage struct {
	SerializedMessage

	// UUID is the unique identifier for this message (crypto/rand UUID v4).
	UUID string `json:"uuid"`

	// ParentUUID links to the previous chain participant. Nil for the first message.
	ParentUUID *string `json:"parentUuid"`

	// LogicalParentUUID preserves logical parent when parentUuid is nullified
	// for session breaks (e.g., compaction boundaries).
	LogicalParentUUID *string `json:"logicalParentUuid,omitempty"`

	// IsSidechain indicates whether this message belongs to a sidechain
	// (e.g., agent sub-conversations).
	IsSidechain bool `json:"isSidechain"`

	// AgentID identifies the agent for sidechain transcripts.
	AgentID string `json:"agentId,omitempty"`

	// TeamName is the team name if this is a spawned agent session.
	TeamName string `json:"teamName,omitempty"`

	// AgentName is the agent's custom name (from /rename or swarm).
	AgentName string `json:"agentName,omitempty"`

	// AgentColor is the agent's color (from /rename or swarm).
	AgentColor string `json:"agentColor,omitempty"`

	// PromptID correlates with OTel prompt.id for user prompt messages.
	PromptID string `json:"promptId,omitempty"`
}

// --- Non-transcript entry types ---

// SummaryMessage records a conversation summary anchored at a leaf UUID.
type SummaryMessage struct {
	Type     string `json:"type"` // "summary"
	LeafUUID string `json:"leafUuid"`
	Summary  string `json:"summary"`
}

// CustomTitleMessage records a user-set custom session title.
type CustomTitleMessage struct {
	Type        string `json:"type"` // "custom-title"
	SessionID   string `json:"sessionId"`
	CustomTitle string `json:"customTitle"`
}

// AiTitleMessage records an AI-generated session title.
// Distinct from CustomTitleMessage: user renames always win.
type AiTitleMessage struct {
	Type      string `json:"type"` // "ai-title"
	SessionID string `json:"sessionId"`
	AiTitle   string `json:"aiTitle"`
}

// LastPromptMessage records the last user prompt for session preview.
type LastPromptMessage struct {
	Type       string `json:"type"` // "last-prompt"
	SessionID  string `json:"sessionId"`
	LastPrompt string `json:"lastPrompt"`
}

// TaskSummaryMessage records a periodic fork-generated summary of current agent activity.
type TaskSummaryMessage struct {
	Type      string `json:"type"` // "task-summary"
	SessionID string `json:"sessionId"`
	Summary   string `json:"summary"`
	Timestamp string `json:"timestamp"`
}

// TagMessage records a tag for session search/filtering.
type TagMessage struct {
	Type      string `json:"type"` // "tag"
	SessionID string `json:"sessionId"`
	Tag       string `json:"tag"`
}

// AgentNameMessage records the agent's custom name.
type AgentNameMessage struct {
	Type      string `json:"type"` // "agent-name"
	SessionID string `json:"sessionId"`
	AgentName string `json:"agentName"`
}

// AgentColorMessage records the agent's color.
type AgentColorMessage struct {
	Type       string `json:"type"` // "agent-color"
	SessionID  string `json:"sessionId"`
	AgentColor string `json:"agentColor"`
}

// AgentSettingMessage records the agent definition used.
type AgentSettingMessage struct {
	Type         string `json:"type"` // "agent-setting"
	SessionID    string `json:"sessionId"`
	AgentSetting string `json:"agentSetting"`
}

// PRLinkMessage links a session to a GitHub pull request.
type PRLinkMessage struct {
	Type         string `json:"type"` // "pr-link"
	SessionID    string `json:"sessionId"`
	PRNumber     int    `json:"prNumber"`
	PRUrl        string `json:"prUrl"`
	PRRepository string `json:"prRepository"` // e.g., "owner/repo"
	Timestamp    string `json:"timestamp"`
}

// FileHistorySnapshotMessage records a file history snapshot.
type FileHistorySnapshotMessage struct {
	Type             string          `json:"type"` // "file-history-snapshot"
	MessageID        string          `json:"messageId"`
	Snapshot         json.RawMessage `json:"snapshot"`
	IsSnapshotUpdate bool            `json:"isSnapshotUpdate"`
}

// AttributionSnapshotMessage tracks character-level contributions by Claude.
type AttributionSnapshotMessage struct {
	Type                              string          `json:"type"` // "attribution-snapshot"
	MessageID                         string          `json:"messageId"`
	Surface                           string          `json:"surface"`
	FileStates                        json.RawMessage `json:"fileStates"`
	PromptCount                       *int            `json:"promptCount,omitempty"`
	PromptCountAtLastCommit           *int            `json:"promptCountAtLastCommit,omitempty"`
	PermissionPromptCount             *int            `json:"permissionPromptCount,omitempty"`
	PermissionPromptCountAtLastCommit *int            `json:"permissionPromptCountAtLastCommit,omitempty"`
	EscapeCount                       *int            `json:"escapeCount,omitempty"`
	EscapeCountAtLastCommit           *int            `json:"escapeCountAtLastCommit,omitempty"`
}

// SpeculationAcceptMessage records an accepted speculative response.
type SpeculationAcceptMessage struct {
	Type        string `json:"type"` // "speculation-accept"
	Timestamp   string `json:"timestamp"`
	TimeSavedMs int    `json:"timeSavedMs"`
}

// ModeEntry records a session mode change (coordinator/normal).
type ModeEntry struct {
	Type      string `json:"type"` // "mode"
	SessionID string `json:"sessionId"`
	Mode      string `json:"mode"` // "coordinator" or "normal"
}

// WorktreeStateEntry records worktree session state for resume.
// WorktreeSession is nil when exited, non-nil when entered.
type WorktreeStateEntry struct {
	Type            string          `json:"type"` // "worktree-state"
	SessionID       string          `json:"sessionId"`
	WorktreeSession json.RawMessage `json:"worktreeSession"` // null | PersistedWorktreeSession
}

// ContentReplacementEntry records content blocks whose in-context representation
// was replaced with a smaller stub.
type ContentReplacementEntry struct {
	Type         string          `json:"type"` // "content-replacement"
	SessionID    string          `json:"sessionId"`
	AgentID      string          `json:"agentId,omitempty"`
	Replacements json.RawMessage `json:"replacements"`
}

// ContextCollapseCommitEntry records a committed context collapse.
// Discriminator is obfuscated to match the gate name in Claude Code.
type ContextCollapseCommitEntry struct {
	Type              string `json:"type"` // "marble-origami-commit"
	SessionID         string `json:"sessionId"`
	CollapseID        string `json:"collapseId"`
	SummaryUUID       string `json:"summaryUuid"`
	SummaryContent    string `json:"summaryContent"`
	Summary           string `json:"summary"`
	FirstArchivedUUID string `json:"firstArchivedUuid"`
	LastArchivedUUID  string `json:"lastArchivedUuid"`
}

// ContextCollapseSnapshotEntry records a snapshot of the staged collapse queue.
// Last-wins: only the most recent snapshot entry is applied on restore.
type ContextCollapseSnapshotEntry struct {
	Type            string          `json:"type"` // "marble-origami-snapshot"
	SessionID       string          `json:"sessionId"`
	Staged          json.RawMessage `json:"staged"` // Array of staged spans
	Armed           bool            `json:"armed"`
	LastSpawnTokens int             `json:"lastSpawnTokens"`
}

// --- Polymorphic entry wrapper ---

// Entry is a generic wrapper for polymorphic JSONL deserialization.
// The Type field is extracted first, then Raw holds the full JSON bytes
// for later typed parsing. Message is kept for backward compatibility
// with old-format entries that used {type, message} structure.
type Entry struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message,omitempty"` // Legacy: full message content
	Raw     json.RawMessage `json:"-"`                 // The full raw JSON line (not serialized)
}

// ParseEntry extracts the type field from raw JSON bytes and stores the
// full raw bytes for later typed parsing.
func ParseEntry(data []byte) (*Entry, error) {
	// Extract just the type field
	var typeOnly struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeOnly); err != nil {
		return nil, fmt.Errorf("parse entry type: %w", err)
	}
	if typeOnly.Type == "" {
		// Legacy entries may use "role" as the type discriminator
		var roleOnly struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(data, &roleOnly); err == nil && roleOnly.Role != "" {
			typeOnly.Type = roleOnly.Role
		}
	}
	return &Entry{
		Type: typeOnly.Type,
		Raw:  json.RawMessage(append([]byte(nil), data...)),
	}, nil
}

// AsTranscriptMessage parses the raw entry as a TranscriptMessage.
// Returns nil if parsing fails.
func (e *Entry) AsTranscriptMessage() *TranscriptMessage {
	if e.Raw == nil {
		return nil
	}
	var tm TranscriptMessage
	if err := json.Unmarshal(e.Raw, &tm); err != nil {
		return nil
	}
	return &tm
}

// --- LogOption for session listing ---

// LogOption holds metadata about a session for listing/browsing.
// Matches Claude Code's LogOption type from types/logs.ts.
type LogOption struct {
	Date                    string                         `json:"date"`
	Messages                []SerializedMessage            `json:"messages,omitempty"`
	FullPath                string                         `json:"fullPath,omitempty"`
	Value                   int                            `json:"value"`
	Created                 int64                          `json:"created"`  // Unix timestamp
	Modified                int64                          `json:"modified"` // Unix timestamp
	FirstPrompt             string                         `json:"firstPrompt"`
	MessageCount            int                            `json:"messageCount"`
	FileSize                int64                          `json:"fileSize,omitempty"`
	IsSidechain             bool                           `json:"isSidechain"`
	IsLite                  bool                           `json:"isLite,omitempty"`
	SessionID               string                         `json:"sessionId,omitempty"`
	TeamName                string                         `json:"teamName,omitempty"`
	AgentName               string                         `json:"agentName,omitempty"`
	AgentColor              string                         `json:"agentColor,omitempty"`
	AgentSetting            string                         `json:"agentSetting,omitempty"`
	IsTeammate              bool                           `json:"isTeammate,omitempty"`
	LeafUUID                string                         `json:"leafUuid,omitempty"`
	Summary                 string                         `json:"summary,omitempty"`
	CustomTitle             string                         `json:"customTitle,omitempty"`
	Tag                     string                         `json:"tag,omitempty"`
	GitBranch               string                         `json:"gitBranch,omitempty"`
	ProjectPath             string                         `json:"projectPath,omitempty"`
	PRNumber                int                            `json:"prNumber,omitempty"`
	PRUrl                   string                         `json:"prUrl,omitempty"`
	PRRepository            string                         `json:"prRepository,omitempty"`
	Mode                    string                         `json:"mode,omitempty"` // "coordinator" or "normal"
	WorktreeSession         json.RawMessage                `json:"worktreeSession,omitempty"`
	ContentReplacements     json.RawMessage                `json:"contentReplacements,omitempty"`
	ContextCollapseCommits  []ContextCollapseCommitEntry   `json:"contextCollapseCommits,omitempty"`
	ContextCollapseSnapshot *ContextCollapseSnapshotEntry  `json:"contextCollapseSnapshot,omitempty"`
	FileHistorySnapshots    []FileHistorySnapshotMessage   `json:"fileHistorySnapshots,omitempty"`
	AttributionSnapshots    []AttributionSnapshotMessage   `json:"attributionSnapshots,omitempty"`
}

// --- Helper functions ---

// IsTranscriptMessage returns true if the entry type represents a transcript
// message (user, assistant, attachment, system). Progress messages are NOT
// transcript messages -- they are ephemeral UI state.
func IsTranscriptMessage(entryType string) bool {
	switch entryType {
	case "user", "assistant", "attachment", "system":
		return true
	}
	return false
}

// IsChainParticipant returns true for all entry types except "progress".
// Used on the write path to skip progress when assigning parentUuid.
func IsChainParticipant(entryType string) bool {
	return entryType != "progress"
}

// NewUUID generates a UUID v4 using crypto/rand.
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// This should never happen; crypto/rand is always available
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	// Set version (4) and variant (RFC 4122)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Package session provides session persistence and command history for ClawGo.
// Sessions are stored as JSONL files in ~/.claude/projects/<hash>/<session-id>.jsonl,
// matching the TypeScript version's session storage format.
package session

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/config"
)

// ChainTracker maintains the UUID chain state for transcript messages.
// It tracks the last chain participant's UUID so the next message can
// reference it as parentUuid, creating a linked list of conversation turns.
type ChainTracker struct {
	mu       sync.Mutex
	lastUUID string
}

// NewChainTracker creates a new ChainTracker with no parent.
func NewChainTracker() *ChainTracker {
	return &ChainTracker{}
}

// NextUUID generates a new UUID and returns it along with a pointer to the
// previous chain participant's UUID (nil for the first message in the chain).
func (ct *ChainTracker) NextUUID() (uuid string, parentUUID *string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	uuid = NewUUID()
	if ct.lastUUID != "" {
		parent := ct.lastUUID
		parentUUID = &parent
	}
	ct.lastUUID = uuid
	return uuid, parentUUID
}

// SetLastUUID sets the last UUID in the chain. Used when loading existing
// chains from disk to resume chain tracking from the correct position.
func (ct *ChainTracker) SetLastUUID(uuid string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.lastUUID = uuid
}

// LastUUID returns the current last UUID in the chain (for inspection/testing).
func (ct *ChainTracker) LastUUID() string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.lastUUID
}

// NewSessionID generates a new session ID using crypto/rand.
// This is an alias for NewUUID for backward compatibility.
func NewSessionID() string {
	return NewUUID()
}

// GetSessionDir returns the session storage directory for a project.
// Path: ~/.claude/projects/<path-hash>/
func GetSessionDir(projectPath string) string {
	hash := hashPath(projectPath)
	return filepath.Join(config.ConfigDir(), "projects", hash)
}

// GetSessionPath returns the full path for a session file.
func GetSessionPath(projectPath, sessionID string) string {
	return filepath.Join(GetSessionDir(projectPath), sessionID+".jsonl")
}

// AppendEntry appends a JSONL entry to the session file.
// Works with both legacy entries (type + message) and new-format entries.
func AppendEntry(sessionPath string, entry Entry) error {
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(sessionPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// AppendTranscriptMessage appends a TranscriptMessage to the session file.
// The caller should populate uuid/parentUuid via a ChainTracker.
func AppendTranscriptMessage(sessionPath string, msg TranscriptMessage) error {
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(sessionPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// AppendMetadataEntry appends a non-transcript metadata entry (SummaryMessage,
// TagMessage, etc.) to the session file.
func AppendMetadataEntry(sessionPath string, entry interface{}) error {
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(sessionPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// LoadSession reads all entries from a session file.
// Supports both legacy format (type + message) and new format (polymorphic entries).
// Each entry's Raw field holds the full JSON line for later typed parsing.
func LoadSession(sessionPath string) ([]Entry, error) {
	f, err := os.Open(sessionPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	// Increase buffer for large session files
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, err := ParseEntry([]byte(line))
		if err != nil {
			continue // skip malformed lines
		}
		// Also try to populate legacy Message field for backward compat
		var legacy struct {
			Message json.RawMessage `json:"message"`
		}
		if json.Unmarshal([]byte(line), &legacy) == nil && legacy.Message != nil {
			entry.Message = legacy.Message
		}
		entries = append(entries, *entry)
	}
	if err := scanner.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}

// LoadTranscript loads a session and returns only the TranscriptMessage entries,
// rebuilding the UUID chain. Legacy entries without UUIDs get new UUIDs assigned.
func LoadTranscript(sessionPath string) ([]TranscriptMessage, *ChainTracker, error) {
	entries, err := LoadSession(sessionPath)
	if err != nil {
		return nil, nil, err
	}

	ct := NewChainTracker()
	var messages []TranscriptMessage

	for _, e := range entries {
		if !IsTranscriptMessage(e.Type) {
			continue
		}

		if e.Raw != nil {
			tm := e.AsTranscriptMessage()
			if tm != nil && tm.UUID != "" {
				// New format: has UUID, use it
				ct.SetLastUUID(tm.UUID)
				messages = append(messages, *tm)
				continue
			}
		}

		// Legacy entry without UUID: assign new UUID from chain tracker
		uuid, parentUUID := ct.NextUUID()
		tm := TranscriptMessage{
			SerializedMessage: SerializedMessage{
				Type: e.Type,
			},
			UUID:       uuid,
			ParentUUID: parentUUID,
		}

		// Try to populate from the legacy message field
		if e.Message != nil {
			tm.Content = e.Message
			// Extract role from the message
			var msgRole struct {
				Role string `json:"role"`
			}
			if json.Unmarshal(e.Message, &msgRole) == nil {
				tm.Role = msgRole.Role
			}
		}

		messages = append(messages, tm)
	}

	return messages, ct, nil
}

// EntryFromUserMessage creates a session Entry from a user message string.
func EntryFromUserMessage(text string) Entry {
	msg := api.UserMessage(text)
	data, _ := json.Marshal(msg)
	return Entry{Type: "user", Message: data}
}

// TranscriptFromUserMessage creates a TranscriptMessage from a user message string,
// using the provided ChainTracker for UUID chain management.
func TranscriptFromUserMessage(text string, ct *ChainTracker, meta SerializedMessage) TranscriptMessage {
	msg := api.UserMessage(text)
	data, _ := json.Marshal(msg)

	uuid, parentUUID := ct.NextUUID()
	return TranscriptMessage{
		SerializedMessage: SerializedMessage{
			Type:       "user",
			Role:       "user",
			Content:    data,
			CWD:        meta.CWD,
			UserType:   meta.UserType,
			Entrypoint: meta.Entrypoint,
			SessionID:  meta.SessionID,
			Timestamp:  meta.Timestamp,
			Version:    meta.Version,
			GitBranch:  meta.GitBranch,
			Slug:       meta.Slug,
		},
		UUID:       uuid,
		ParentUUID: parentUUID,
	}
}

// TranscriptFromMessage creates a TranscriptMessage from an API message,
// using the provided ChainTracker for UUID chain management.
func TranscriptFromMessage(msg api.Message, ct *ChainTracker, meta SerializedMessage) TranscriptMessage {
	data, _ := json.Marshal(msg)

	entryType := msg.Role
	if entryType == "" {
		entryType = "assistant"
	}

	uuid, parentUUID := ct.NextUUID()
	return TranscriptMessage{
		SerializedMessage: SerializedMessage{
			Type:       entryType,
			Role:       msg.Role,
			Content:    data,
			CWD:        meta.CWD,
			UserType:   meta.UserType,
			Entrypoint: meta.Entrypoint,
			SessionID:  meta.SessionID,
			Timestamp:  meta.Timestamp,
			Version:    meta.Version,
			GitBranch:  meta.GitBranch,
			Slug:       meta.Slug,
		},
		UUID:       uuid,
		ParentUUID: parentUUID,
	}
}

// EntryFromMessage creates a session Entry from any API message.
func EntryFromMessage(msg api.Message) Entry {
	data, _ := json.Marshal(msg)
	return Entry{Type: msg.Role, Message: data}
}

// EntriesToMessages converts session entries back to API messages for resume.
// Handles both legacy entries (with Message field) and new entries (with Raw field).
func EntriesToMessages(entries []Entry) []api.Message {
	messages := make([]api.Message, 0, len(entries))
	for _, e := range entries {
		if !IsTranscriptMessage(e.Type) {
			continue
		}

		var msg api.Message

		// Try legacy Message field first
		if e.Message != nil {
			if err := json.Unmarshal(e.Message, &msg); err == nil {
				messages = append(messages, msg)
				continue
			}
		}

		// Try Raw field (new format TranscriptMessage has content inline)
		if e.Raw != nil {
			var tm TranscriptMessage
			if err := json.Unmarshal(e.Raw, &tm); err == nil && tm.Content != nil {
				if err := json.Unmarshal(tm.Content, &msg); err == nil {
					messages = append(messages, msg)
					continue
				}
			}
		}
	}
	return messages
}

func hashPath(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:8]) // 16-char hex prefix
}

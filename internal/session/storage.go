// Package session provides session persistence and command history for ClawGo.
// Sessions are stored as JSONL files in ~/.claude/projects/<hash>/<session-id>.jsonl,
// matching the TypeScript version's session storage format.
package session

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/config"
)

// Entry represents a single entry in the session transcript.
type Entry struct {
	Type    string          `json:"type"`    // "user", "assistant", "tool_use", "tool_result", "system"
	Message json.RawMessage `json:"message"` // Full message content
}

// NewSessionID generates a new session ID using crypto/rand.
func NewSessionID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: this should never happen
		return "fallback-session"
	}
	// Format as UUID-like string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
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

// LoadSession reads all entries from a session file.
func LoadSession(sessionPath string) ([]Entry, error) {
	f, err := os.Open(sessionPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}

// EntryFromUserMessage creates a session Entry from a user message string.
func EntryFromUserMessage(text string) Entry {
	msg := api.UserMessage(text)
	data, _ := json.Marshal(msg)
	return Entry{Type: "user", Message: data}
}

// EntryFromMessage creates a session Entry from any API message.
func EntryFromMessage(msg api.Message) Entry {
	data, _ := json.Marshal(msg)
	return Entry{Type: msg.Role, Message: data}
}

// EntriesToMessages converts session entries back to API messages for resume.
func EntriesToMessages(entries []Entry) []api.Message {
	messages := make([]api.Message, 0, len(entries))
	for _, e := range entries {
		var msg api.Message
		if err := json.Unmarshal(e.Message, &msg); err == nil {
			messages = append(messages, msg)
		}
	}
	return messages
}

func hashPath(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:8]) // 16-char hex prefix
}

package session

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/config"
)

const defaultHistoryMaxSize = 1000

// History manages command input history with arrow-up recall.
type History struct {
	entries []string
	pos     int // current position for navigation; -1 = before start, len(entries) = past end
	path    string
	maxSize int
}

// NewHistory creates a history instance, loading from disk.
func NewHistory() *History {
	h := &History{
		path:    filepath.Join(config.ConfigDir(), "command_history.jsonl"),
		maxSize: defaultHistoryMaxSize,
	}
	h.load()
	h.pos = len(h.entries) // start past end (no selection)
	return h
}

// NewHistoryWithPath creates a history instance with a custom path (for testing).
func NewHistoryWithPath(path string) *History {
	h := &History{
		path:    path,
		maxSize: defaultHistoryMaxSize,
	}
	h.load()
	h.pos = len(h.entries)
	return h
}

// Add appends a command to history and persists.
func (h *History) Add(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	// Deduplicate: don't add if same as last entry
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == text {
		return
	}
	h.entries = append(h.entries, text)
	if len(h.entries) > h.maxSize {
		h.entries = h.entries[len(h.entries)-h.maxSize:]
	}
	h.pos = len(h.entries) // reset position to end
	h.persist(text)
}

// Previous returns the previous history entry (arrow-up).
func (h *History) Previous() (string, bool) {
	if h.pos <= 0 {
		return "", false
	}
	h.pos--
	return h.entries[h.pos], true
}

// Next returns the next history entry (arrow-down).
func (h *History) Next() (string, bool) {
	if h.pos >= len(h.entries)-1 {
		h.pos = len(h.entries)
		return "", true // empty = clear input
	}
	h.pos++
	return h.entries[h.pos], true
}

// Len returns the number of history entries.
func (h *History) Len() int {
	return len(h.entries)
}

func (h *History) load() {
	f, err := os.Open(h.path)
	if err != nil {
		return // file doesn't exist yet
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			h.entries = append(h.entries, line)
		}
	}
}

func (h *History) persist(text string) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(h.path), 0755); err != nil {
		return
	}
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(text + "\n")
}

// Package attribution tracks file changes per session with content hashes
// and provides git trailer formatting for AI attribution.
package attribution

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// FileState represents the state of a tracked file at a point in time.
type FileState struct {
	Path       string    `json:"path"`
	Hash       string    `json:"hash"`
	ModifiedByAI bool   `json:"modified_by_ai"`
	Timestamp  time.Time `json:"timestamp"`
}

// Tracker records file changes within a session, computing content hashes
// for change detection and tracking AI vs human modifications.
type Tracker struct {
	mu        sync.Mutex
	states    map[string]*FileState
	sessionID string
}

// NewTracker creates a new attribution tracker for the given session.
func NewTracker(sessionID string) *Tracker {
	return &Tracker{
		states:    make(map[string]*FileState),
		sessionID: sessionID,
	}
}

// RecordChange records a file modification with a SHA256 content hash.
// If the file was already tracked, its state is replaced with the latest
// (full state, not delta -- only the most recent state per file is kept).
func (t *Tracker) RecordChange(path string, content []byte, byAI bool) {
	hash := fmt.Sprintf("%x", sha256.Sum256(content))

	t.mu.Lock()
	defer t.mu.Unlock()

	t.states[path] = &FileState{
		Path:       path,
		Hash:       hash,
		ModifiedByAI: byAI,
		Timestamp:  time.Now(),
	}
}

// GetAttribution returns a copy of all tracked file states.
func (t *Tracker) GetAttribution() map[string]*FileState {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string]*FileState, len(t.states))
	for k, v := range t.states {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetAIModifiedFiles returns the paths of all files modified by AI,
// sorted alphabetically for deterministic output.
func (t *Tracker) GetAIModifiedFiles() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var paths []string
	for _, fs := range t.states {
		if fs.ModifiedByAI {
			paths = append(paths, fs.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

// Snapshot returns a JSON representation of the current tracker state.
// It marshals the full state (not delta) per research pitfall 5.
func (t *Tracker) Snapshot() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(t.states)
	if err != nil {
		// Should never fail for simple map[string]*FileState
		return []byte("{}")
	}
	return data
}

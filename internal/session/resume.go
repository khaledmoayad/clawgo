package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// FindLatestSession returns the path of the most recently modified session file.
func FindLatestSession(projectPath string) (string, error) {
	dir := GetSessionDir(projectPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("cannot read session directory %s: %w", dir, err)
	}

	// Filter .jsonl files and get their info
	type fileEntry struct {
		path    string
		modTime int64
	}
	var files []fileEntry
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no session files found in %s", dir)
	}

	// Sort by modification time descending
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	return files[0].path, nil
}

// Resume loads a session by ID or latest.
// If sessionID is provided, loads that specific session.
// If sessionID is empty, loads the most recently modified session.
// Returns the session entries, the resolved session ID, and any error.
func Resume(projectPath, sessionID string) ([]Entry, string, error) {
	if sessionID != "" {
		path := GetSessionPath(projectPath, sessionID)
		entries, err := LoadSession(path)
		return entries, sessionID, err
	}

	// Find latest
	path, err := FindLatestSession(projectPath)
	if err != nil {
		return nil, "", err
	}

	// Extract session ID from filename
	sid := filepath.Base(path)
	sid = sid[:len(sid)-len(".jsonl")] // strip extension

	entries, err := LoadSession(path)
	return entries, sid, err
}

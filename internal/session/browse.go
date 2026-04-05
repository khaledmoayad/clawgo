package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// SessionInfo holds metadata about a past session for history browsing.
type SessionInfo struct {
	// ID is the session identifier (derived from the filename).
	ID string

	// Path is the full filesystem path to the session JSONL file.
	Path string

	// StartTime is the approximate start time of the session.
	// Derived from the first entry timestamp or file creation time.
	StartTime time.Time

	// Duration is the approximate session duration (last activity - first activity).
	Duration time.Duration

	// MessageCount is the number of JSONL lines (entries) in the session file.
	MessageCount int

	// FirstMessage is a preview of the first user message, truncated to 100 chars.
	FirstMessage string

	// EstimatedCost is a placeholder for per-session cost tracking (not yet wired).
	EstimatedCost float64
}

// ListSessions scans the session directory and returns metadata for past sessions.
// Results are sorted by modification time descending (newest first).
// The limit parameter controls the maximum number of sessions returned (0 = no limit).
func ListSessions(projectPath string, limit int) ([]SessionInfo, error) {
	dir := GetSessionDir(projectPath)
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read session directory %s: %w", dir, err)
	}

	// Collect .jsonl files with their mod times for sorting
	type fileInfo struct {
		name    string
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, de := range dirEntries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".jsonl" {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			name:    de.Name(),
			path:    filepath.Join(dir, de.Name()),
			modTime: info.ModTime(),
		})
	}

	// Sort by modification time descending (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	// Apply limit
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	sessions := make([]SessionInfo, 0, len(files))
	for _, f := range files {
		si := SessionInfo{
			ID:   strings.TrimSuffix(f.name, ".jsonl"),
			Path: f.path,
		}

		// Read the file to extract metadata
		entries, lineCount, firstEntryTime, err := readSessionMetadata(f.path)
		if err != nil {
			// Still include the session with basic info from the filesystem
			si.StartTime = f.modTime
			si.MessageCount = 0
			sessions = append(sessions, si)
			continue
		}

		si.MessageCount = lineCount

		// Start time: use first entry time if available, otherwise file mod time
		if !firstEntryTime.IsZero() {
			si.StartTime = firstEntryTime
		} else {
			si.StartTime = f.modTime
		}

		// Duration: mod time - start time (approximate)
		si.Duration = f.modTime.Sub(si.StartTime)
		if si.Duration < 0 {
			si.Duration = 0
		}

		// First user message preview
		si.FirstMessage = GetSessionPreview(entries)

		// EstimatedCost: placeholder, not yet wired
		si.EstimatedCost = 0

		sessions = append(sessions, si)
	}

	return sessions, nil
}

// readSessionMetadata reads a session file and returns:
// - the parsed entries (for preview extraction)
// - the line count
// - the time from the first entry (if parseable)
// This reads the full file but avoids parsing every entry completely.
func readSessionMetadata(sessionPath string) ([]Entry, int, time.Time, error) {
	f, err := os.Open(sessionPath)
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	defer f.Close()

	var entries []Entry
	lineCount := 0
	var firstEntryTime time.Time

	scanner := bufio.NewScanner(f)
	// Increase buffer for large session files
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineCount++

		entry, err := ParseEntry([]byte(line))
		if err != nil {
			continue
		}
		// Also populate legacy Message field if present
		var legacy struct {
			Message json.RawMessage `json:"message"`
		}
		if json.Unmarshal([]byte(line), &legacy) == nil && legacy.Message != nil {
			entry.Message = legacy.Message
		}
		entries = append(entries, *entry)

		// Try to extract timestamp from first entry
		if lineCount == 1 {
			var ts struct {
				Timestamp string `json:"timestamp"`
			}
			// Try the raw line first (new format has timestamp at top level)
			_ = json.Unmarshal([]byte(line), &ts)
			if ts.Timestamp == "" && entry.Message != nil {
				// Fallback: legacy format has timestamp inside the message
				_ = json.Unmarshal(entry.Message, &ts)
			}
			if ts.Timestamp != "" {
				if t, err := time.Parse(time.RFC3339, ts.Timestamp); err == nil {
					firstEntryTime = t
				}
			}
		}
	}

	return entries, lineCount, firstEntryTime, scanner.Err()
}

// GetSessionPreview finds the first user message entry and extracts a text
// preview, truncated to 100 characters with "..." suffix if longer.
func GetSessionPreview(entries []Entry) string {
	for _, e := range entries {
		if e.Type != "user" {
			continue
		}

		// Try legacy Message field first
		if e.Message != nil {
			var msg api.Message
			if err := json.Unmarshal(e.Message, &msg); err == nil {
				if text := extractPreviewText(msg); text != "" {
					return text
				}
			}
		}

		// Try Raw field (new format: TranscriptMessage with inline content)
		if e.Raw != nil {
			var tm TranscriptMessage
			if err := json.Unmarshal(e.Raw, &tm); err == nil && tm.Content != nil {
				var msg api.Message
				if err := json.Unmarshal(tm.Content, &msg); err == nil {
					if text := extractPreviewText(msg); text != "" {
						return text
					}
				}
			}
		}
	}
	return ""
}

// extractPreviewText extracts text from a message, truncating to 100 chars.
func extractPreviewText(msg api.Message) string {
	for _, block := range msg.Content {
		if block.Type == api.ContentText && block.Text != "" {
			text := strings.TrimSpace(block.Text)
			if len(text) > 100 {
				return text[:100] + "..."
			}
			return text
		}
	}
	return ""
}

// FormatSessionList formats sessions for display as a table-like string.
func FormatSessionList(sessions []SessionInfo) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}

	var sb strings.Builder
	// Header
	sb.WriteString(fmt.Sprintf("%-20s | %-10s | %-8s | %s\n", "ID", "Date", "Messages", "Preview"))
	sb.WriteString(strings.Repeat("-", 20))
	sb.WriteString("-+-")
	sb.WriteString(strings.Repeat("-", 10))
	sb.WriteString("-+-")
	sb.WriteString(strings.Repeat("-", 8))
	sb.WriteString("-+-")
	sb.WriteString(strings.Repeat("-", 40))
	sb.WriteString("\n")

	for _, s := range sessions {
		id := s.ID
		if len(id) > 17 {
			id = id[:17] + "..."
		}
		preview := s.FirstMessage
		if len(preview) > 40 {
			preview = preview[:37] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-20s | %-10s | %-8d | %s\n",
			id,
			s.StartTime.Format("2006-01-02"),
			s.MessageCount,
			preview,
		))
	}

	return sb.String()
}

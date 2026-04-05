// Package write implements the WriteTool for creating and overwriting files.
package write

import (
	"fmt"
	"os"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/filestate"
)

// checkStaleness verifies that the file at filePath has been read and
// has not been modified since the last read. This prevents silent data
// loss from concurrent edits. Returns nil if the write is safe to proceed.
func checkStaleness(filePath string, cache *filestate.FileStateCache) error {
	// Stat the file to get mtime
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// New file -- no staleness check needed
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if file was read
	state, ok := cache.Get(filePath)
	if !ok || state.IsPartialView {
		return fmt.Errorf("File has not been read yet. Read it first before writing to it.")
	}

	// Compare mtime: if file was modified after our last read, it's stale
	lastWriteTime := info.ModTime().UnixMilli()
	if lastWriteTime > state.Timestamp {
		return fmt.Errorf("File has been modified since read, either by the user or by a linter. Read it again before attempting to write it.")
	}

	return nil
}

// detectLineEnding detects the line ending style used in the given content.
// Returns "\r\n" if CRLF is found, otherwise "\n".
func detectLineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// preserveLineEnding converts newContent to use the originalLineEnding style.
// If the content already uses the correct style, it is returned unchanged.
func preserveLineEnding(newContent string, originalLineEnding string) string {
	if originalLineEnding == "\r\n" {
		// First normalize to LF, then convert to CRLF
		normalized := strings.ReplaceAll(newContent, "\r\n", "\n")
		return strings.ReplaceAll(normalized, "\n", "\r\n")
	}
	// Original is LF -- strip any CRLF to LF
	return strings.ReplaceAll(newContent, "\r\n", "\n")
}

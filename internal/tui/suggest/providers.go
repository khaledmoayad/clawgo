package suggest

import (
	"os"
	"path/filepath"
	"strings"
)

// SuggestionProvider is the interface for pluggable suggestion sources.
// Each provider decides whether it applies to the current input and returns
// matching suggestions.
type SuggestionProvider interface {
	// Match returns true if this provider should handle the given input.
	Match(input string, cursorPos int) bool

	// Suggest returns suggestions for the given input and cursor position.
	Suggest(input string, cursorPos int) []Suggestion
}

// Compile-time interface compliance checks.
var (
	_ SuggestionProvider = (*CommandProvider)(nil)
	_ SuggestionProvider = (*FilePathProvider)(nil)
	_ SuggestionProvider = (*ShellHistoryProvider)(nil)
)

// CommandProvider suggests slash commands when the input starts with "/".
type CommandProvider struct {
	Commands []string // Available command names (without leading /)
}

// Match returns true when the input starts with "/".
func (p *CommandProvider) Match(input string, cursorPos int) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "/")
}

// Suggest returns commands that prefix-match the text after "/".
func (p *CommandProvider) Suggest(input string, cursorPos int) []Suggestion {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	// Extract the partial command name after "/"
	partial := strings.ToLower(trimmed[1:])

	var results []Suggestion
	for _, cmd := range p.Commands {
		if partial == "" || strings.HasPrefix(strings.ToLower(cmd), partial) {
			results = append(results, Suggestion{
				Text:     "/" + cmd,
				Icon:     "/",
				Provider: "command",
			})
		}
	}
	return results
}

// FilePathProvider suggests file paths when the input contains "@".
type FilePathProvider struct {
	WorkingDir string // Base directory for relative path resolution
}

// Match returns true when the input contains "@" followed by a partial path.
func (p *FilePathProvider) Match(input string, cursorPos int) bool {
	// Look for @ symbol in the input up to cursor position
	relevant := input
	if cursorPos > 0 && cursorPos <= len(input) {
		relevant = input[:cursorPos]
	}
	return strings.Contains(relevant, "@")
}

// Suggest returns file/directory entries matching the partial path after "@".
func (p *FilePathProvider) Suggest(input string, cursorPos int) []Suggestion {
	relevant := input
	if cursorPos > 0 && cursorPos <= len(input) {
		relevant = input[:cursorPos]
	}

	// Find the last "@" and extract partial path after it
	idx := strings.LastIndex(relevant, "@")
	if idx < 0 {
		return nil
	}

	partial := relevant[idx+1:]
	// Stop at whitespace after the @
	if spaceIdx := strings.IndexByte(partial, ' '); spaceIdx >= 0 {
		partial = partial[:spaceIdx]
	}

	// Resolve the directory and prefix to match
	dir := p.WorkingDir
	prefix := partial

	if partial != "" {
		if filepath.IsAbs(partial) {
			dir = filepath.Dir(partial)
			prefix = filepath.Base(partial)
		} else {
			expanded := filepath.Join(p.WorkingDir, partial)
			// If partial ends with /, treat as directory
			if strings.HasSuffix(partial, "/") || strings.HasSuffix(partial, string(filepath.Separator)) {
				dir = expanded
				prefix = ""
			} else {
				dir = filepath.Dir(expanded)
				prefix = filepath.Base(partial)
			}
		}
	}

	// Read directory entries
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var results []Suggestion
	lowerPrefix := strings.ToLower(prefix)
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files unless the prefix starts with "."
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(lowerPrefix, ".") {
			continue
		}

		if lowerPrefix == "" || strings.HasPrefix(strings.ToLower(name), lowerPrefix) {
			// Build the relative path for display
			relPath := name
			if partial != "" && strings.Contains(partial, "/") {
				// Preserve the directory prefix the user already typed
				dirPart := partial[:strings.LastIndex(partial, "/")+1]
				relPath = dirPart + name
			}

			desc := "file"
			if entry.IsDir() {
				relPath += "/"
				desc = "dir"
			}

			results = append(results, Suggestion{
				Text:        relPath,
				Description: desc,
				Icon:        "@",
				Provider:    "filepath",
			})
		}

		// Limit results
		if len(results) >= 20 {
			break
		}
	}

	return results
}

// ShellHistoryProvider suggests shell commands from history when input is
// empty or starts with "!".
type ShellHistoryProvider struct {
	History []string // Previously executed shell commands
}

// Match returns true when the input is empty or starts with "!".
func (p *ShellHistoryProvider) Match(input string, cursorPos int) bool {
	trimmed := strings.TrimSpace(input)
	return trimmed == "" || strings.HasPrefix(trimmed, "!")
}

// Suggest returns history entries matching the text after "!",
// or all recent entries if input is empty.
func (p *ShellHistoryProvider) Suggest(input string, cursorPos int) []Suggestion {
	trimmed := strings.TrimSpace(input)

	var partial string
	if strings.HasPrefix(trimmed, "!") {
		partial = strings.ToLower(trimmed[1:])
	}

	var results []Suggestion
	seen := make(map[string]bool)

	for _, cmd := range p.History {
		if seen[cmd] {
			continue
		}
		seen[cmd] = true

		if partial == "" || strings.Contains(strings.ToLower(cmd), partial) {
			results = append(results, Suggestion{
				Text:     "!" + cmd,
				Icon:     "!",
				Provider: "history",
			})
		}

		// Limit to 15 recent matching entries
		if len(results) >= 15 {
			break
		}
	}

	return results
}

// Package claudemd implements discovery and loading of CLAUDE.md project
// instruction files. It walks the directory tree from CWD upward, collecting
// managed, user, project, and local memory files in priority order.
//
// Priority order (first loaded = lowest index):
//  1. Managed: /etc/claude-code/CLAUDE.md
//  2. User: ~/.claude/CLAUDE.md, ~/.claude/rules/*.md
//  3. Project: CLAUDE.md and .claude/CLAUDE.md from root down to CWD
//  4. Local: CLAUDE.local.md in each project directory
package claudemd

import (
	"os"
	"path/filepath"
	"sort"
)

// MemoryType categorizes the source of a CLAUDE.md file.
type MemoryType string

const (
	// MemoryManaged is a system-wide config from /etc/claude-code/.
	MemoryManaged MemoryType = "managed"
	// MemoryUser is a per-user config from ~/.claude/.
	MemoryUser MemoryType = "user"
	// MemoryProject is a project-level config from the directory tree.
	MemoryProject MemoryType = "project"
	// MemoryLocal is a local-only config (CLAUDE.local.md, not checked in).
	MemoryLocal MemoryType = "local"
)

// MemoryFile represents a single loaded CLAUDE.md instruction file.
type MemoryFile struct {
	Path    string     // Absolute path to the file
	Type    MemoryType // Classification of the file source
	Content string     // File contents
}

// LoadMemoryFiles discovers and loads CLAUDE.md files starting from cwd,
// walking upward through parent directories. Files are returned in priority
// order: managed -> user -> project (root-to-CWD) -> local.
func LoadMemoryFiles(cwd string, homeDir string) ([]MemoryFile, error) {
	var files []MemoryFile
	seen := make(map[string]bool)

	// Step 1: Managed config (/etc/claude-code/CLAUDE.md)
	tryLoad(&files, seen, "/etc/claude-code/CLAUDE.md", MemoryManaged)

	// Step 2: User config (~/.claude/CLAUDE.md)
	tryLoad(&files, seen, filepath.Join(homeDir, ".claude", "CLAUDE.md"), MemoryUser)

	// Step 2b: User rules (~/.claude/rules/*.md)
	loadRulesDir(&files, seen, filepath.Join(homeDir, ".claude", "rules"), MemoryUser)

	// Step 3: Collect directories from CWD upward
	dirs := collectDirsUpward(cwd)

	// Step 4: Process directories from root downward (reverse the collected list)
	// dirs is currently [cwd, parent, grandparent, ..., /]
	// We want [/, ..., grandparent, parent, cwd]
	reversed := make([]string, len(dirs))
	for i, d := range dirs {
		reversed[len(dirs)-1-i] = d
	}

	for _, dir := range reversed {
		// Try dir/CLAUDE.md as Project
		tryLoad(&files, seen, filepath.Join(dir, "CLAUDE.md"), MemoryProject)

		// Try dir/.claude/CLAUDE.md as Project
		tryLoad(&files, seen, filepath.Join(dir, ".claude", "CLAUDE.md"), MemoryProject)

		// Load dir/.claude/rules/*.md as Project
		loadRulesDir(&files, seen, filepath.Join(dir, ".claude", "rules"), MemoryProject)

		// Try dir/CLAUDE.local.md as Local
		tryLoad(&files, seen, filepath.Join(dir, "CLAUDE.local.md"), MemoryLocal)
	}

	return files, nil
}

// tryLoad attempts to read a file and append it to the files slice.
// If the file doesn't exist or is already seen, it's silently skipped.
func tryLoad(files *[]MemoryFile, seen map[string]bool, path string, typ MemoryType) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	if seen[absPath] {
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return // file doesn't exist or can't be read -- not an error
	}

	seen[absPath] = true
	*files = append(*files, MemoryFile{
		Path:    absPath,
		Type:    typ,
		Content: string(data),
	})
}

// loadRulesDir loads all *.md files from a directory, sorted alphabetically.
func loadRulesDir(files *[]MemoryFile, seen map[string]bool, dir string, typ MemoryType) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(matches) == 0 {
		return
	}

	sort.Strings(matches)
	for _, m := range matches {
		tryLoad(files, seen, m, typ)
	}
}

// collectDirsUpward collects directories from cwd upward to the root.
// Returns [cwd, parent, grandparent, ..., /].
func collectDirsUpward(cwd string) []string {
	var dirs []string
	current := filepath.Clean(cwd)

	for {
		dirs = append(dirs, current)
		parent := filepath.Dir(current)
		if parent == current {
			break // reached root
		}
		current = parent
	}

	return dirs
}

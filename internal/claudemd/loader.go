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
	Path         string       // Absolute path to the file
	Type         MemoryType   // Classification of the file source
	Content      string       // File contents
	Frontmatter  *Frontmatter // Parsed frontmatter data (nil if none)
	IncludedFrom string       // Path of the file that included this one (empty for directly loaded)
}

// LoadMemoryFiles discovers and loads CLAUDE.md files starting from cwd,
// walking upward through parent directories. Files are returned in priority
// order: managed -> user -> project (root-to-CWD) -> local.
//
// For each loaded file:
//   - Frontmatter (--- delimited YAML) is parsed and stripped from content
//   - @include directives are resolved, and included files appear BEFORE the
//     including file in the result slice
//   - Circular references across all files are prevented via a shared seen set
func LoadMemoryFiles(cwd string, homeDir string) ([]MemoryFile, error) {
	var files []MemoryFile
	seen := make(map[string]bool)

	// Step 1: Managed config (/etc/claude-code/CLAUDE.md)
	loadWithIncludes(&files, seen, "/etc/claude-code/CLAUDE.md", MemoryManaged, homeDir)

	// Step 2: User config (~/.claude/CLAUDE.md)
	loadWithIncludes(&files, seen, filepath.Join(homeDir, ".claude", "CLAUDE.md"), MemoryUser, homeDir)

	// Step 2b: User rules (~/.claude/rules/*.md)
	loadRulesDirWithIncludes(&files, seen, filepath.Join(homeDir, ".claude", "rules"), MemoryUser, homeDir)

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
		loadWithIncludes(&files, seen, filepath.Join(dir, "CLAUDE.md"), MemoryProject, homeDir)

		// Try dir/.claude/CLAUDE.md as Project
		loadWithIncludes(&files, seen, filepath.Join(dir, ".claude", "CLAUDE.md"), MemoryProject, homeDir)

		// Load dir/.claude/rules/*.md as Project
		loadRulesDirWithIncludes(&files, seen, filepath.Join(dir, ".claude", "rules"), MemoryProject, homeDir)

		// Try dir/CLAUDE.local.md as Local
		loadWithIncludes(&files, seen, filepath.Join(dir, "CLAUDE.local.md"), MemoryLocal, homeDir)
	}

	return files, nil
}

// loadWithIncludes reads a file, parses frontmatter, resolves @include
// directives, and appends all results to the files slice. Included files
// appear BEFORE the including file.
func loadWithIncludes(files *[]MemoryFile, seen map[string]bool, path string, typ MemoryType, homeDir string) {
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
	content := string(data)

	// Parse and strip frontmatter
	fm, strippedContent := ParseFrontmatter(content)

	// Resolve @include directives
	baseDir := filepath.Dir(absPath)
	included, cleanedContent, _ := resolveIncludes(strippedContent, baseDir, homeDir, seen, 0, typ)

	// Included files appear BEFORE the including file
	*files = append(*files, included...)

	// Append the main file with frontmatter stripped and includes cleaned
	*files = append(*files, MemoryFile{
		Path:        absPath,
		Type:        typ,
		Content:     cleanedContent,
		Frontmatter: fm,
	})
}

// tryLoad attempts to read a file and append it to the files slice.
// If the file doesn't exist or is already seen, it's silently skipped.
// Frontmatter is parsed and stripped from the content.
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

	content := string(data)
	fm, strippedContent := ParseFrontmatter(content)

	*files = append(*files, MemoryFile{
		Path:        absPath,
		Type:        typ,
		Content:     strippedContent,
		Frontmatter: fm,
	})
}

// loadRulesDirWithIncludes loads all *.md files from a directory with include resolution.
func loadRulesDirWithIncludes(files *[]MemoryFile, seen map[string]bool, dir string, typ MemoryType, homeDir string) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil || len(matches) == 0 {
		return
	}

	sort.Strings(matches)
	for _, m := range matches {
		loadWithIncludes(files, seen, m, typ, homeDir)
	}
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

// Package claudemd: @include directive resolution for CLAUDE.md files.
//
// Memory files can include other files using @ notation:
//   - @./relative/path  -- relative to the including file's directory
//   - @relative/path    -- same as @./relative/path
//   - @~/home/path      -- relative to the user's home directory
//   - @/absolute/path   -- absolute path
//
// Included files are resolved recursively with circular reference prevention.
// Only text file extensions are allowed (binary files are silently ignored).
// Non-existent @include targets are silently ignored.
package claudemd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxIncludeDepth limits recursive @include resolution to prevent runaway nesting.
const MaxIncludeDepth = 5

// textFileExtensions is the set of file extensions allowed for @include directives.
// This prevents binary files (images, PDFs, etc.) from being loaded into memory.
var textFileExtensions = map[string]bool{
	// Markdown and text
	".md": true, ".txt": true, ".text": true,
	// Data formats
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true, ".csv": true,
	// Web
	".html": true, ".htm": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
	// JavaScript/TypeScript
	".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
	// Python
	".py": true, ".pyi": true, ".pyw": true,
	// Ruby
	".rb": true, ".erb": true, ".rake": true,
	// Go
	".go": true,
	// Rust
	".rs": true,
	// Java/Kotlin/Scala
	".java": true, ".kt": true, ".kts": true, ".scala": true,
	// C/C++
	".c": true, ".cpp": true, ".cc": true, ".cxx": true, ".h": true, ".hpp": true, ".hxx": true,
	// C#
	".cs": true,
	// Swift
	".swift": true,
	// Shell
	".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true, ".bat": true, ".cmd": true,
	// Config
	".env": true, ".ini": true, ".cfg": true, ".conf": true, ".config": true, ".properties": true,
	// Database
	".sql": true, ".graphql": true, ".gql": true,
	// Protocol
	".proto": true,
	// Frontend frameworks
	".vue": true, ".svelte": true, ".astro": true,
	// Templating
	".ejs": true, ".hbs": true, ".pug": true, ".jade": true,
	// Other languages
	".php": true, ".pl": true, ".pm": true, ".lua": true, ".r": true,
	".dart": true, ".ex": true, ".exs": true, ".erl": true, ".hrl": true,
	".clj": true, ".cljs": true, ".cljc": true, ".edn": true,
	".hs": true, ".lhs": true, ".elm": true,
	".ml": true, ".mli": true,
	".f": true, ".f90": true, ".f95": true, ".for": true,
	// Build files
	".cmake": true, ".make": true, ".makefile": true, ".gradle": true, ".sbt": true,
	// Documentation
	".rst": true, ".adoc": true, ".asciidoc": true, ".org": true, ".tex": true, ".latex": true,
	// Lock files
	".lock": true,
	// Misc
	".log": true, ".diff": true, ".patch": true,
	// Additional from plan
	".liquid": true, ".jinja": true, ".j2": true, ".tf": true, ".hcl": true,
	".dockerfile": true, ".rmd": true, ".m": true, ".mm": true,
	".nim": true, ".zig": true, ".v": true, ".sv": true,
	".vhd": true, ".vhdl": true, ".ada": true, ".adb": true, ".ads": true,
	".fs": true, ".fsx": true,
}

// includePathRegex matches @include patterns in text lines.
// The path must contain a / or start with ./, ~/, or /.
// This avoids matching email addresses and @mentions.
var includePathRegex = regexp.MustCompile(`(?:^|\s)@((?:\./|~/|/)[^\s]+|[a-zA-Z0-9_.-]+/[^\s]+)`)

// isTextFileExtension checks if the file has a text file extension allowed for includes.
func isTextFileExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}
	return textFileExtensions[ext]
}

// resolveIncludes scans content for @include directives and resolves them.
// It returns:
//   - included: MemoryFile entries for all transitively included files
//   - cleanedContent: the original content with @include references removed
//   - err: any error (currently always nil)
//
// Parameters:
//   - content: the file content to scan for @include directives
//   - baseDir: directory of the file containing the content (for relative path resolution)
//   - homeDir: user's home directory (for ~/ path resolution)
//   - seen: set of already-processed absolute paths (prevents circular references)
//   - depth: current include depth (stops at MaxIncludeDepth)
//   - parentType: the MemoryType of the including file
func resolveIncludes(content string, baseDir string, homeDir string, seen map[string]bool, depth int, parentType MemoryType) ([]MemoryFile, string, error) {
	if depth >= MaxIncludeDepth {
		return nil, content, nil
	}

	var included []MemoryFile
	var cleanedLines []string
	inCodeBlock := false
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track code block fences (``` or ~~~)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// Skip @include resolution inside code blocks
		if inCodeBlock {
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// Find @include references in this line
		matches := includePathRegex.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// Process matches in reverse order so we can modify the line without
		// invalidating earlier indices
		cleanedLine := line
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			// m[2] and m[3] are the submatch (the path portion after @)
			rawPath := line[m[2]:m[3]]

			// Strip fragment identifiers (#heading)
			if hashIdx := strings.Index(rawPath, "#"); hashIdx != -1 {
				rawPath = rawPath[:hashIdx]
			}
			if rawPath == "" {
				continue
			}

			// Resolve the path
			resolvedPath := resolvePath(rawPath, baseDir, homeDir)
			if resolvedPath == "" {
				continue
			}

			absPath, err := filepath.Abs(resolvedPath)
			if err != nil {
				continue
			}

			// Check if extension is allowed
			if !isTextFileExtension(absPath) {
				continue
			}

			// Check for circular reference
			if seen[absPath] {
				continue
			}

			// Try to read the file
			data, err := os.ReadFile(absPath)
			if err != nil {
				// Non-existent files are silently ignored
				continue
			}

			// Mark as seen before recursing
			seen[absPath] = true

			fileContent := string(data)

			// Recursively resolve includes in the included file
			childDir := filepath.Dir(absPath)
			childIncludes, childContent, _ := resolveIncludes(fileContent, childDir, homeDir, seen, depth+1, parentType)

			// Add child's transitive includes first (they appear before the child)
			included = append(included, childIncludes...)

			// Add the included file itself
			included = append(included, MemoryFile{
				Path:         absPath,
				Type:         parentType,
				Content:      childContent,
				IncludedFrom: filepath.Join(baseDir, filepath.Base(baseDir)),
			})

			// Remove the @reference from the cleaned line.
			// m[0] is the start of the full match (including optional leading whitespace)
			// m[1] is the end of the full match
			// We replace from the @ sign position to the end of the path.
			// Find the @ position within the match.
			atPos := strings.Index(line[m[0]:m[1]], "@")
			if atPos >= 0 {
				removeStart := m[0] + atPos
				removeEnd := m[1]
				cleanedLine = cleanedLine[:removeStart] + cleanedLine[removeEnd:]
			}
		}

		cleanedLines = append(cleanedLines, cleanedLine)
	}

	return included, strings.Join(cleanedLines, "\n"), nil
}

// resolvePath resolves an @include path to a filesystem path.
//   - ./relative or relative/path -- relative to baseDir
//   - ~/path -- relative to homeDir
//   - /absolute -- absolute path
func resolvePath(rawPath string, baseDir string, homeDir string) string {
	if strings.HasPrefix(rawPath, "~/") {
		return filepath.Join(homeDir, rawPath[2:])
	}
	if strings.HasPrefix(rawPath, "/") {
		return rawPath
	}
	if strings.HasPrefix(rawPath, "./") {
		return filepath.Join(baseDir, rawPath[2:])
	}
	// Bare relative path (contains /)
	return filepath.Join(baseDir, rawPath)
}

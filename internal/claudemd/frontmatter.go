// Package claudemd: frontmatter parsing for CLAUDE.md files.
//
// CLAUDE.md files may contain YAML frontmatter between --- delimiters.
// Frontmatter is parsed and stripped before the content is shown to the model.
// The globs field allows path-scoped rules (only activated when matching files
// are touched).
package claudemd

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Frontmatter represents parsed YAML frontmatter from a CLAUDE.md file.
type Frontmatter struct {
	Globs  []string          // File path patterns this rule applies to
	Fields map[string]string // Other frontmatter key-value pairs
}

// frontmatterRegex matches YAML frontmatter between --- delimiters at the start of content.
var frontmatterRegex = regexp.MustCompile(`(?s)\A\s*---\s*\n(.*?)---\s*\n?`)

// ParseFrontmatter strips YAML frontmatter (between --- delimiters) from content.
// Returns the frontmatter data and the content without frontmatter.
// If no frontmatter is detected, returns nil frontmatter and the original content.
func ParseFrontmatter(content string) (*Frontmatter, string) {
	match := frontmatterRegex.FindStringSubmatchIndex(content)
	if match == nil {
		return nil, content
	}

	// match[2]:match[3] is the captured YAML block
	yamlBlock := content[match[2]:match[3]]
	remaining := content[match[1]:]

	fm := parseSimpleYAML(yamlBlock)
	if fm == nil {
		return nil, content
	}

	return fm, remaining
}

// parseSimpleYAML does lightweight key: value YAML parsing.
// Supports simple string values and list values (both inline [a, b] and multi-line - item).
func parseSimpleYAML(yaml string) *Frontmatter {
	fm := &Frontmatter{
		Fields: make(map[string]string),
	}

	lines := strings.Split(yaml, "\n")
	var currentKey string
	var collectingList []string
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if this is a list item continuation (  - value)
		if inList && strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(trimmed[2:])
			item = unquoteYAML(item)
			collectingList = append(collectingList, item)
			continue
		}

		// If we were collecting a list, finalize it
		if inList {
			if currentKey == "globs" || currentKey == "paths" {
				fm.Globs = collectingList
			}
			inList = false
			collectingList = nil
		}

		// Parse key: value
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:colonIdx])
		value := strings.TrimSpace(trimmed[colonIdx+1:])

		currentKey = key

		if value == "" {
			// Could be start of a multi-line list
			inList = true
			collectingList = nil
			continue
		}

		// Check for inline list: [a, b, c]
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			inner := value[1 : len(value)-1]
			items := splitRespectingQuotes(inner)
			if key == "globs" || key == "paths" {
				fm.Globs = items
			} else {
				fm.Fields[key] = value
			}
			continue
		}

		// Simple string value
		fm.Fields[key] = unquoteYAML(value)
	}

	// Finalize any trailing list
	if inList && len(collectingList) > 0 {
		if currentKey == "globs" || currentKey == "paths" {
			fm.Globs = collectingList
		}
	}

	return fm
}

// splitRespectingQuotes splits a comma-separated string, handling quoted values.
func splitRespectingQuotes(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !inQuote && (ch == '"' || ch == '\'') {
			inQuote = true
			quoteChar = ch
			continue
		}
		if inQuote && ch == quoteChar {
			inQuote = false
			continue
		}
		if !inQuote && ch == ',' {
			item := strings.TrimSpace(current.String())
			if item != "" {
				result = append(result, item)
			}
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}

	item := strings.TrimSpace(current.String())
	if item != "" {
		result = append(result, item)
	}

	return result
}

// unquoteYAML removes surrounding quotes from a YAML value.
func unquoteYAML(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// MatchesFrontmatterGlobs checks if any of the frontmatter glob patterns match the given file path.
// Returns true if the file path matches at least one glob pattern.
func MatchesFrontmatterGlobs(globs []string, filePath string) bool {
	for _, pattern := range globs {
		matched, err := filepath.Match(pattern, filePath)
		if err == nil && matched {
			return true
		}
		// Also try matching just the base name
		matched, err = filepath.Match(pattern, filepath.Base(filePath))
		if err == nil && matched {
			return true
		}
	}
	return false
}

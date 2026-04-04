package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a loaded skill file with its parsed metadata.
type Skill struct {
	Name        string       // Skill name (from filename or frontmatter)
	Description string       // Short description
	FilePath    string       // Absolute path to the skill file
	Content     string       // Markdown body (after frontmatter extraction)
	Frontmatter *Frontmatter // Parsed frontmatter (nil if none)
	Source      string       // Origin: "skills", "plugin", "managed", "bundled"
	LoadedFrom  string       // Directory path the skill was loaded from
}

// LoadSkills scans standard skill directories and returns all discovered skills.
// Directories are scanned in order:
//  1. {projectRoot}/.claude/skills/
//  2. {configDir}/skills/
//  3. {projectRoot}/.claude/commands/ (legacy)
//
// Deduplication is by name (first found wins), matching the TS behavior.
func LoadSkills(projectRoot, configDir string) ([]*Skill, error) {
	seen := make(map[string]bool)
	var all []*Skill

	dirs := []struct {
		path   string
		source string
	}{
		{filepath.Join(projectRoot, ".claude", "skills"), "skills"},
		{filepath.Join(configDir, "skills"), "skills"},
		{filepath.Join(projectRoot, ".claude", "commands"), "skills"},
	}

	for _, d := range dirs {
		skills, err := LoadSkillsFromDir(d.path, d.source)
		if err != nil {
			// Directory may not exist -- that's fine
			continue
		}
		for _, s := range skills {
			if !seen[s.Name] {
				seen[s.Name] = true
				all = append(all, s)
			}
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Name < all[j].Name
	})
	return all, nil
}

// LoadSkillsFromDir loads skills from a single directory (recursive).
// Used by the plugin system for loading skills from plugin directories.
func LoadSkillsFromDir(dir, source string) ([]*Skill, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrNotExist
	}

	var skills []*Skill

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files
		}

		fm, body, parseErr := ParseFrontmatter(data)
		if parseErr != nil {
			// Parse error on frontmatter -- use file with full content
			fm = nil
			body = string(data)
		}

		name := deriveName(path, dir, fm)
		desc := deriveDescription(fm, body)

		skills = append(skills, &Skill{
			Name:        name,
			Description: desc,
			FilePath:    path,
			Content:     body,
			Frontmatter: fm,
			Source:      source,
			LoadedFrom:  dir,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return skills, nil
}

// SkillNames returns the names of the given skills.
func SkillNames(skills []*Skill) []string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return names
}

// deriveName determines the skill name from frontmatter or filename.
func deriveName(path, baseDir string, fm *Frontmatter) string {
	if fm != nil && fm.Name != "" {
		return fm.Name
	}
	// Use relative path from base dir, strip .md extension
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	// Strip .md extension
	name := strings.TrimSuffix(rel, ".md")
	// Replace path separators with / for consistency
	name = filepath.ToSlash(name)
	return name
}

// deriveDescription extracts a description from frontmatter or the first
// non-header, non-empty paragraph in the markdown body.
func deriveDescription(fm *Frontmatter, body string) string {
	if fm != nil && fm.Description != "" {
		return fm.Description
	}
	return extractDescriptionFromMarkdown(body)
}

// extractDescriptionFromMarkdown finds the first non-header, non-empty line.
func extractDescriptionFromMarkdown(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Return first content line, truncated if too long
		if len(trimmed) > 200 {
			return trimmed[:200] + "..."
		}
		return trimmed
	}
	return ""
}

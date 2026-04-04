// Package skills implements skill loading, frontmatter parsing, and change detection.
// Skills are markdown files in .claude/skills/ that provide specialized instructions
// for specific tasks, matching the TypeScript skills system behavior.
package skills

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds parsed YAML frontmatter from a skill markdown file.
type Frontmatter struct {
	Name                   string          `yaml:"name"`
	Description            string          `yaml:"description"`
	Aliases                []string        `yaml:"aliases"`
	AllowedTools           []string        `yaml:"allowed_tools"`
	Model                  string          `yaml:"model"`
	WhenToUse              string          `yaml:"when_to_use"`
	ArgumentHint           string          `yaml:"argument_hint"`
	DisableModelInvocation bool            `yaml:"disable_model_invocation"`
	UserInvocable          *bool           `yaml:"user_invocable"` // pointer for unset detection, default true
	Hooks                  json.RawMessage `yaml:"hooks"`          // raw JSON for hook config
	Context                string          `yaml:"context"`        // "inline" or "fork"
	Agent                  string          `yaml:"agent"`
}

// ParseFrontmatter splits YAML frontmatter from the markdown body.
// If the content does not start with "---", it returns nil Frontmatter and the
// full content as the body (graceful fallback).
func ParseFrontmatter(content []byte) (*Frontmatter, string, error) {
	s := string(content)
	trimmed := strings.TrimSpace(s)

	// Must start with ---
	if !strings.HasPrefix(trimmed, "---") {
		return nil, s, nil
	}

	// Split into first line and remainder
	lines := strings.SplitN(trimmed, "\n", 2)
	if len(lines) < 2 {
		// Only "---" and nothing else
		return nil, s, nil
	}

	// First line must be exactly "---" (possibly with trailing \r)
	if strings.TrimRight(lines[0], "\r") != "---" {
		return nil, s, nil
	}

	// Find the closing "---" in the remaining content
	remaining := lines[1]
	closingIdx := -1
	pos := 0
	for _, line := range strings.Split(remaining, "\n") {
		trimLine := strings.TrimRight(line, "\r")
		if trimLine == "---" {
			closingIdx = pos
			break
		}
		pos += len(line) + 1 // +1 for the \n
	}

	if closingIdx < 0 {
		// No closing --- found, treat entire content as body
		return nil, s, nil
	}

	yamlData := remaining[:closingIdx]
	body := remaining[closingIdx+3:] // skip "---"
	// Trim leading newline from body
	body = strings.TrimPrefix(body, "\r\n")
	body = strings.TrimPrefix(body, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlData), &fm); err != nil {
		return nil, s, err
	}

	return &fm, body, nil
}

package skills

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrontmatter_ValidYAML(t *testing.T) {
	content := []byte(`---
name: my-skill
description: A test skill
aliases:
  - ms
  - skill1
allowed_tools:
  - BashTool
  - FileReadTool
model: claude-sonnet-4-20250514
when_to_use: Use when testing
context: inline
---
# My Skill

This is the body.
`)

	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	require.NotNil(t, fm)

	assert.Equal(t, "my-skill", fm.Name)
	assert.Equal(t, "A test skill", fm.Description)
	assert.Equal(t, []string{"ms", "skill1"}, fm.Aliases)
	assert.Equal(t, []string{"BashTool", "FileReadTool"}, fm.AllowedTools)
	assert.Equal(t, "claude-sonnet-4-20250514", fm.Model)
	assert.Equal(t, "Use when testing", fm.WhenToUse)
	assert.Equal(t, "inline", fm.Context)
	assert.Contains(t, body, "# My Skill")
	assert.Contains(t, body, "This is the body.")
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte("# Just Markdown\n\nNo frontmatter here.")

	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	assert.Nil(t, fm)
	assert.Equal(t, string(content), body)
}

func TestParseFrontmatter_UserInvocablePointer(t *testing.T) {
	content := []byte(`---
name: test
user_invocable: false
---
Body`)

	fm, _, err := ParseFrontmatter(content)
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.UserInvocable)
	assert.False(t, *fm.UserInvocable)
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	content := []byte(`---
---
Body content here`)

	fm, body, err := ParseFrontmatter(content)
	require.NoError(t, err)
	// Empty frontmatter is valid YAML (nil fields)
	require.NotNil(t, fm)
	assert.Equal(t, "", fm.Name)
	assert.Contains(t, body, "Body content here")
}

func TestLoadSkills_FindsMdFiles(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, ".claude", "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))

	// Create skill files
	require.NoError(t, os.WriteFile(
		filepath.Join(skillsDir, "coding.md"),
		[]byte("---\nname: coding\ndescription: Coding skill\n---\n# Coding\nWrite clean code."),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillsDir, "review.md"),
		[]byte("# Review\nReview code carefully."),
		0o644,
	))

	skills, err := LoadSkills(tmpDir, filepath.Join(tmpDir, "config"))
	require.NoError(t, err)
	assert.Len(t, skills, 2)

	// Should be sorted by name
	assert.Equal(t, "coding", skills[0].Name)
	assert.Equal(t, "review", skills[1].Name)
}

func TestLoadSkillsFromDir_ScansDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a nested skill
	subDir := filepath.Join(tmpDir, "advanced")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "basic.md"),
		[]byte("Basic skill content"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(subDir, "expert.md"),
		[]byte("---\nname: expert-mode\n---\nExpert skill content"),
		0o644,
	))

	skills, err := LoadSkillsFromDir(tmpDir, "skills")
	require.NoError(t, err)
	assert.Len(t, skills, 2)

	// Check that names are derived correctly
	names := SkillNames(skills)
	assert.Contains(t, names, "basic")
	assert.Contains(t, names, "expert-mode") // from frontmatter, not path
}

func TestLoadSkills_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()

	// Create same-named skill in two directories
	dir1 := filepath.Join(tmpDir, ".claude", "skills")
	dir2 := filepath.Join(tmpDir, "config", "skills")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(dir1, "dupe.md"),
		[]byte("First version"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir2, "dupe.md"),
		[]byte("Second version"),
		0o644,
	))

	skills, err := LoadSkills(tmpDir, filepath.Join(tmpDir, "config"))
	require.NoError(t, err)

	// First found wins (project skills before config skills)
	assert.Len(t, skills, 1)
	assert.Equal(t, "dupe", skills[0].Name)
	assert.Equal(t, "First version", skills[0].Content)
}

func TestLoadSkillsFromDir_NonExistentDir(t *testing.T) {
	skills, err := LoadSkillsFromDir("/nonexistent/path", "skills")
	assert.Error(t, err)
	assert.Nil(t, skills)
}

func TestSkillNames(t *testing.T) {
	skills := []*Skill{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}
	names := SkillNames(skills)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, names)
}

func TestExtractDescriptionFromMarkdown(t *testing.T) {
	body := "# Title\n\nThis is the first paragraph.\n\nMore content."
	desc := extractDescriptionFromMarkdown(body)
	assert.Equal(t, "This is the first paragraph.", desc)
}

func TestExtractDescriptionFromMarkdown_NoContent(t *testing.T) {
	body := "# Only Headers\n## Sub Header"
	desc := extractDescriptionFromMarkdown(body)
	assert.Equal(t, "", desc)
}

func TestWatcher_DetectsChanges(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "initial.md"),
		[]byte("initial content"),
		0o644,
	))

	var changeCount atomic.Int32
	w := NewWatcher([]string{tmpDir}, func() {
		changeCount.Add(1)
	})

	require.NoError(t, w.Start())
	defer w.Stop()

	// Wait a moment, then modify a file
	time.Sleep(100 * time.Millisecond)

	// Advance mtime sufficiently for detection
	future := time.Now().Add(5 * time.Second)
	require.NoError(t, os.Chtimes(filepath.Join(tmpDir, "initial.md"), future, future))

	// Wait for at least one poll cycle
	time.Sleep(PollInterval + 500*time.Millisecond)

	assert.GreaterOrEqual(t, changeCount.Load(), int32(1), "onChange should have been called at least once")
}

func TestWatcher_DetectsNewFile(t *testing.T) {
	tmpDir := t.TempDir()

	var changeCount atomic.Int32
	w := NewWatcher([]string{tmpDir}, func() {
		changeCount.Add(1)
	})

	require.NoError(t, w.Start())
	defer w.Stop()

	// Wait a moment, then create a new file
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "new-skill.md"),
		[]byte("new skill"),
		0o644,
	))

	// Wait for poll
	time.Sleep(PollInterval + 500*time.Millisecond)

	assert.GreaterOrEqual(t, changeCount.Load(), int32(1), "onChange should fire for new file")
}

func TestWatcher_StopTerminates(t *testing.T) {
	w := NewWatcher([]string{t.TempDir()}, func() {})
	require.NoError(t, w.Start())
	w.Stop()
	w.Stop() // Double stop should not panic
}

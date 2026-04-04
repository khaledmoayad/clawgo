package claudemd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMemoryFiles_FindsInCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create CLAUDE.md in current directory
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Project"), 0644)
	require.NoError(t, err)

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	found := false
	for _, f := range files {
		if f.Type == MemoryProject && filepath.Base(f.Path) == "CLAUDE.md" {
			found = true
			assert.Equal(t, "# Project", f.Content)
		}
	}
	assert.True(t, found, "should find CLAUDE.md in current directory")
}

func TestLoadMemoryFiles_FindsUserClaudeMD(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create ~/.claude/CLAUDE.md
	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("# User"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	found := false
	for _, f := range files {
		if f.Type == MemoryUser && filepath.Base(f.Path) == "CLAUDE.md" {
			found = true
			assert.Equal(t, "# User", f.Content)
		}
	}
	assert.True(t, found, "should find ~/.claude/CLAUDE.md as User type")
}

func TestLoadMemoryFiles_WalksUpward(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create directory structure: tmpDir/a/b/c
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(deepDir, 0755))

	// Create CLAUDE.md at root and at "a" level
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Root"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a", "CLAUDE.md"), []byte("# Level A"), 0644))

	files, err := LoadMemoryFiles(deepDir, homeDir)
	require.NoError(t, err)

	// Should find both, root first (root-to-CWD order)
	var projectFiles []MemoryFile
	for _, f := range files {
		if f.Type == MemoryProject {
			projectFiles = append(projectFiles, f)
		}
	}
	require.GreaterOrEqual(t, len(projectFiles), 2)
	assert.Equal(t, "# Root", projectFiles[0].Content)
	assert.Equal(t, "# Level A", projectFiles[1].Content)
}

func TestLoadMemoryFiles_PriorityOrder(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create managed, user, project, and local files
	etcDir := filepath.Join(tmpDir, "etc", "claude-code")
	require.NoError(t, os.MkdirAll(etcDir, 0755))

	claudeDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("# User"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("# Project"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "CLAUDE.local.md"), []byte("# Local"), 0644))

	files, err := LoadMemoryFiles(projectDir, homeDir)
	require.NoError(t, err)

	// Verify order: user comes before project, project before local
	userIdx := -1
	projectIdx := -1
	localIdx := -1
	for i, f := range files {
		if f.Type == MemoryUser {
			userIdx = i
		}
		if f.Type == MemoryProject {
			projectIdx = i
		}
		if f.Type == MemoryLocal {
			localIdx = i
		}
	}
	assert.True(t, userIdx < projectIdx, "user should come before project")
	assert.True(t, projectIdx < localIdx, "project should come before local")
}

func TestLoadMemoryFiles_FindsDotClaudeDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create .claude/CLAUDE.md in project dir
	dotClaudeDir := filepath.Join(tmpDir, ".claude")
	require.NoError(t, os.MkdirAll(dotClaudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dotClaudeDir, "CLAUDE.md"), []byte("# Dot Claude"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	found := false
	for _, f := range files {
		if f.Content == "# Dot Claude" {
			found = true
		}
	}
	assert.True(t, found, "should find .claude/CLAUDE.md in project dir")
}

func TestLoadMemoryFiles_FindsRulesDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create .claude/rules/*.md
	rulesDir := filepath.Join(tmpDir, ".claude", "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "rule1.md"), []byte("# Rule 1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "rule2.md"), []byte("# Rule 2"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	ruleCount := 0
	for _, f := range files {
		if f.Content == "# Rule 1" || f.Content == "# Rule 2" {
			ruleCount++
		}
	}
	assert.Equal(t, 2, ruleCount, "should find both rule files")
}

func TestLoadMemoryFiles_FindsLocalMD(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.local.md"), []byte("# Local"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	found := false
	for _, f := range files {
		if f.Type == MemoryLocal && f.Content == "# Local" {
			found = true
		}
	}
	assert.True(t, found, "should find CLAUDE.local.md")
}

func TestLoadMemoryFiles_DeduplicatesSamePath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create CLAUDE.md -- the CWD and directory walk will both find it
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Test"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	// Count how many times this specific file appears
	count := 0
	absPath, _ := filepath.Abs(filepath.Join(tmpDir, "CLAUDE.md"))
	for _, f := range files {
		if f.Path == absPath {
			count++
		}
	}
	assert.Equal(t, 1, count, "should not duplicate same file path")
}

func TestLoadMemoryFiles_HandlesMissingGracefully(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// No files created -- should return empty without error
	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestLoadMemoryFiles_UserRulesDir(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create ~/.claude/rules/*.md
	rulesDir := filepath.Join(homeDir, ".claude", "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "global-rule.md"), []byte("# Global Rule"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	found := false
	for _, f := range files {
		if f.Type == MemoryUser && f.Content == "# Global Rule" {
			found = true
		}
	}
	assert.True(t, found, "should find user rules from ~/.claude/rules/")
}

func TestCollectDirsUpward(t *testing.T) {
	dirs := collectDirsUpward("/a/b/c")
	assert.Contains(t, dirs, "/a/b/c")
	assert.Contains(t, dirs, "/a/b")
	assert.Contains(t, dirs, "/a")
	assert.Contains(t, dirs, "/")
}

package claudemd

import (
	"os"
	"path/filepath"
	"strings"
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

// --- @include directive tests ---

func TestInclude_BasicRelativeInclude(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the included file
	err := os.WriteFile(filepath.Join(tmpDir, "other.md"), []byte("# Other Content"), 0644)
	require.NoError(t, err)

	content := "Some text\n@./other.md\nMore text"
	seen := make(map[string]bool)
	included, cleaned, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 1)
	assert.Equal(t, "# Other Content", included[0].Content)
	assert.NotContains(t, cleaned, "@./other.md")
	assert.Contains(t, cleaned, "Some text")
	assert.Contains(t, cleaned, "More text")
}

func TestInclude_CircularReference(t *testing.T) {
	tmpDir := t.TempDir()

	// A includes B, B includes A
	err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte("File A\n@./b.md"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte("File B\n@./a.md"), 0644)
	require.NoError(t, err)

	seen := make(map[string]bool)
	absA, _ := filepath.Abs(filepath.Join(tmpDir, "a.md"))
	seen[absA] = true // mark A as already seen (it's the file being processed)

	contentA := "File A\n@./b.md"
	included, _, err := resolveIncludes(contentA, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	// Should include B but B should NOT re-include A (circular)
	assert.Len(t, included, 1)
	assert.Contains(t, included[0].Content, "File B")
}

func TestInclude_NestedInclude(t *testing.T) {
	tmpDir := t.TempDir()

	// A includes B, B includes C
	subDir := filepath.Join(tmpDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	err := os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte("File B\n@./sub/c.md"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "c.md"), []byte("File C"), 0644)
	require.NoError(t, err)

	content := "File A\n@./b.md"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	// Should include C first (transitive), then B
	assert.Len(t, included, 2)
	assert.Equal(t, "File C", included[0].Content)
	assert.Contains(t, included[1].Content, "File B")
}

func TestInclude_CodeBlockSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "should-not-include.md"), []byte("# Should Not Include"), 0644)
	require.NoError(t, err)

	content := "Before\n```\n@./should-not-include.md\n```\nAfter"
	seen := make(map[string]bool)
	included, cleaned, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 0, "should not resolve @includes inside code blocks")
	assert.Contains(t, cleaned, "@./should-not-include.md", "code block content should be preserved")
}

func TestInclude_TildeFenceCodeBlockSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "nope.md"), []byte("# Nope"), 0644)
	require.NoError(t, err)

	content := "Before\n~~~\n@./nope.md\n~~~\nAfter"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 0, "should not resolve @includes inside ~~~ code blocks")
}

func TestInclude_NonTextExtensionIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "image.png"), []byte("fake image"), 0644)
	require.NoError(t, err)

	content := "Look at @./image.png"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 0, "should not include non-text file extensions")
}

func TestInclude_HomeDirExpansion(t *testing.T) {
	homeDir := t.TempDir()

	err := os.WriteFile(filepath.Join(homeDir, "notes.md"), []byte("# Home Notes"), 0644)
	require.NoError(t, err)

	content := "Check @~/notes.md"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, "/some/other/dir", homeDir, seen, 0, MemoryUser)
	require.NoError(t, err)

	assert.Len(t, included, 1)
	assert.Equal(t, "# Home Notes", included[0].Content)
}

func TestInclude_NonExistentFileSilentlyIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	content := "Reference @./missing.md here"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 0, "non-existent files should be silently ignored")
}

func TestInclude_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	absFile := filepath.Join(tmpDir, "abs.md")

	err := os.WriteFile(absFile, []byte("# Absolute"), 0644)
	require.NoError(t, err)

	content := "See @" + absFile
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, "/irrelevant", "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 1)
	assert.Equal(t, "# Absolute", included[0].Content)
}

func TestInclude_BareRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	err := os.WriteFile(filepath.Join(subDir, "guide.md"), []byte("# Guide"), 0644)
	require.NoError(t, err)

	content := "See @docs/guide.md"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	assert.Len(t, included, 1)
	assert.Equal(t, "# Guide", included[0].Content)
}

func TestInclude_DepthLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a chain of includes deeper than MaxIncludeDepth
	for i := 0; i < MaxIncludeDepth+2; i++ {
		name := filepath.Join(tmpDir, strings.ReplaceAll(strings.Repeat("d/", i)+"file.md", "//", "/"))
		dir := filepath.Dir(name)
		require.NoError(t, os.MkdirAll(dir, 0755))

		var fileContent string
		if i < MaxIncludeDepth+1 {
			nextName := strings.Repeat("d/", 1) + "file.md"
			fileContent = "Level " + string(rune('0'+i)) + "\n@./" + nextName
		} else {
			fileContent = "Deepest level"
		}
		require.NoError(t, os.WriteFile(name, []byte(fileContent), 0644))
	}

	content := "Root\n@./file.md"
	seen := make(map[string]bool)
	included, _, err := resolveIncludes(content, tmpDir, "/home/user", seen, 0, MemoryProject)
	require.NoError(t, err)

	// Should not exceed MaxIncludeDepth levels of nesting
	assert.LessOrEqual(t, len(included), MaxIncludeDepth+1)
}

func TestIsTextFileExtension(t *testing.T) {
	assert.True(t, isTextFileExtension("file.md"))
	assert.True(t, isTextFileExtension("file.go"))
	assert.True(t, isTextFileExtension("file.ts"))
	assert.True(t, isTextFileExtension("file.py"))
	assert.True(t, isTextFileExtension("FILE.MD")) // case-insensitive
	assert.False(t, isTextFileExtension("image.png"))
	assert.False(t, isTextFileExtension("photo.jpg"))
	assert.False(t, isTextFileExtension("doc.pdf"))
	assert.False(t, isTextFileExtension("noextension"))
}

func TestResolvePath(t *testing.T) {
	assert.Equal(t, "/base/dir/file.md", resolvePath("./file.md", "/base/dir", "/home/user"))
	assert.Equal(t, "/home/user/notes.md", resolvePath("~/notes.md", "/base/dir", "/home/user"))
	assert.Equal(t, "/absolute/path.md", resolvePath("/absolute/path.md", "/base/dir", "/home/user"))
	assert.Equal(t, "/base/dir/sub/file.md", resolvePath("sub/file.md", "/base/dir", "/home/user"))
}

// --- Frontmatter tests ---

func TestParseFrontmatter_WithGlobs(t *testing.T) {
	content := "---\nglobs: [\"*.go\", \"*.ts\"]\n---\n# Content here"
	fm, remaining := ParseFrontmatter(content)

	require.NotNil(t, fm)
	assert.Equal(t, []string{"*.go", "*.ts"}, fm.Globs)
	assert.Equal(t, "# Content here", remaining)
}

func TestParseFrontmatter_WithSingleGlob(t *testing.T) {
	content := "---\nglobs: \"*.go\"\n---\nContent"
	fm, remaining := ParseFrontmatter(content)

	require.NotNil(t, fm)
	// Single string value stored in Fields, not Globs
	assert.Equal(t, "Content", remaining)
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Just content\nNo frontmatter here"
	fm, remaining := ParseFrontmatter(content)

	assert.Nil(t, fm)
	assert.Equal(t, content, remaining)
}

func TestParseFrontmatter_StripsFromContent(t *testing.T) {
	content := "---\ndescription: A test rule\npaths: [\"src/**\"]\n---\n# Rule Title\nRule body text"
	fm, remaining := ParseFrontmatter(content)

	require.NotNil(t, fm)
	assert.NotContains(t, remaining, "---")
	assert.NotContains(t, remaining, "description:")
	assert.Contains(t, remaining, "# Rule Title")
	assert.Contains(t, remaining, "Rule body text")
}

func TestParseFrontmatter_MultiLineList(t *testing.T) {
	content := "---\nglobs:\n  - \"*.go\"\n  - \"*.rs\"\n---\nContent"
	fm, remaining := ParseFrontmatter(content)

	require.NotNil(t, fm)
	assert.Equal(t, []string{"*.go", "*.rs"}, fm.Globs)
	assert.Equal(t, "Content", remaining)
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	content := "---\n---\nContent"
	fm, remaining := ParseFrontmatter(content)

	require.NotNil(t, fm)
	assert.Equal(t, "Content", remaining)
}

func TestParseFrontmatter_FieldValues(t *testing.T) {
	content := "---\ndescription: My rule\nmodel: haiku\n---\nContent"
	fm, remaining := ParseFrontmatter(content)

	require.NotNil(t, fm)
	assert.Equal(t, "My rule", fm.Fields["description"])
	assert.Equal(t, "haiku", fm.Fields["model"])
	assert.Equal(t, "Content", remaining)
}

func TestMatchesFrontmatterGlobs(t *testing.T) {
	assert.True(t, MatchesFrontmatterGlobs([]string{"*.go"}, "main.go"))
	assert.True(t, MatchesFrontmatterGlobs([]string{"*.ts", "*.go"}, "app.ts"))
	assert.False(t, MatchesFrontmatterGlobs([]string{"*.go"}, "main.py"))
	assert.False(t, MatchesFrontmatterGlobs([]string{}, "main.go"))
}

// --- Integration tests: LoadMemoryFiles with includes ---

func TestLoadMemoryFiles_IncludesResolvedDuringLoad(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create CLAUDE.md with an @include
	includeFile := filepath.Join(tmpDir, "extra.md")
	require.NoError(t, os.WriteFile(includeFile, []byte("# Extra Content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Main\n@./extra.md"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	// Should have at least 2 files: the included extra.md and the main CLAUDE.md
	require.GreaterOrEqual(t, len(files), 2)

	// Find the included file and the main file
	var foundExtra, foundMain bool
	var extraIdx, mainIdx int
	for i, f := range files {
		if filepath.Base(f.Path) == "extra.md" {
			foundExtra = true
			extraIdx = i
			assert.Equal(t, "# Extra Content", f.Content)
		}
		if filepath.Base(f.Path) == "CLAUDE.md" && f.Type == MemoryProject {
			foundMain = true
			mainIdx = i
		}
	}

	assert.True(t, foundExtra, "should find included extra.md")
	assert.True(t, foundMain, "should find main CLAUDE.md")
	assert.Less(t, extraIdx, mainIdx, "included files should appear before the including file")
}

func TestLoadMemoryFiles_FrontmatterStrippedFromContent(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	content := "---\nglobs: [\"*.go\"]\ndescription: Go rules\n---\n# Go Rules\nAlways use gofmt"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(content), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	var found bool
	for _, f := range files {
		if f.Type == MemoryProject && filepath.Base(f.Path) == "CLAUDE.md" {
			found = true
			// Content should NOT contain frontmatter
			assert.NotContains(t, f.Content, "---")
			assert.NotContains(t, f.Content, "globs:")
			assert.Contains(t, f.Content, "# Go Rules")
			assert.Contains(t, f.Content, "Always use gofmt")
			// Frontmatter should be parsed
			require.NotNil(t, f.Frontmatter)
			assert.Equal(t, []string{"*.go"}, f.Frontmatter.Globs)
		}
	}
	assert.True(t, found)
}

func TestLoadMemoryFiles_IncludesOrderCorrect(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create a chain: CLAUDE.md -> inc1.md -> inc2.md
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "inc2.md"), []byte("# Inc2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "inc1.md"), []byte("# Inc1\n@./inc2.md"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Main\n@./inc1.md"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	// Find project files in order
	var projectContents []string
	for _, f := range files {
		if f.Type == MemoryProject {
			projectContents = append(projectContents, f.Content)
		}
	}

	// inc2 (deepest include) should come first, then inc1, then CLAUDE.md
	require.GreaterOrEqual(t, len(projectContents), 3)
	assert.Equal(t, "# Inc2", projectContents[0])
	assert.Contains(t, projectContents[1], "# Inc1")
	assert.Contains(t, projectContents[2], "# Main")
}

func TestLoadMemoryFiles_CrossFileCircularPrevention(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Create two files that include each other
	// CLAUDE.md -> shared.md, CLAUDE.local.md -> shared.md
	// shared.md should only appear once
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "shared.md"), []byte("# Shared"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Project\n@./shared.md"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.local.md"), []byte("# Local\n@./shared.md"), 0644))

	files, err := LoadMemoryFiles(tmpDir, homeDir)
	require.NoError(t, err)

	// Count how many times shared.md appears
	sharedCount := 0
	for _, f := range files {
		if filepath.Base(f.Path) == "shared.md" {
			sharedCount++
		}
	}
	assert.Equal(t, 1, sharedCount, "shared.md should only appear once due to cross-file dedup")
}

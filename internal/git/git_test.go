package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a temporary git repo for testing.
// It runs git init, configures user email/name, and returns the repo path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	run("init", "--initial-branch=main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	return dir
}

// commitFile creates, adds, and commits a file in the test repo.
func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	run("add", name)
	run("commit", "-m", msg)
}

// --- FindRoot Tests ---

func TestFindRoot_InRepo(t *testing.T) {
	dir := initTestRepo(t)
	// Need at least one commit for some git operations
	commitFile(t, dir, "init.txt", "init", "initial commit")

	root, err := FindRoot(dir)
	assert.NoError(t, err)
	// Resolve symlinks for temp dir comparison
	expected, _ := filepath.EvalSymlinks(dir)
	actual, _ := filepath.EvalSymlinks(root)
	assert.Equal(t, expected, actual)
}

func TestFindRoot_NotRepo(t *testing.T) {
	dir := t.TempDir() // Not a git repo
	_, err := FindRoot(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestIsGitRepo(t *testing.T) {
	gitDir := initTestRepo(t)
	nonGitDir := t.TempDir()

	assert.True(t, IsGitRepo(gitDir))
	assert.False(t, IsGitRepo(nonGitDir))
}

// --- Branch Tests ---

func TestBranch(t *testing.T) {
	dir := initTestRepo(t)
	commitFile(t, dir, "init.txt", "init", "initial commit")

	ctx := context.Background()
	branch, err := Branch(ctx, dir)
	assert.NoError(t, err)
	assert.Equal(t, "main", branch)
}

// --- Status Tests ---

func TestStatus_Clean(t *testing.T) {
	dir := initTestRepo(t)
	commitFile(t, dir, "file.txt", "content", "add file")

	ctx := context.Background()
	result, err := Status(ctx, dir)
	assert.NoError(t, err)
	assert.True(t, result.IsClean)
	assert.Empty(t, result.Files)
}

func TestStatus_Modified(t *testing.T) {
	dir := initTestRepo(t)
	commitFile(t, dir, "file.txt", "content", "add file")

	// Modify the file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644))

	ctx := context.Background()
	result, err := Status(ctx, dir)
	assert.NoError(t, err)
	assert.False(t, result.IsClean)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "file.txt", result.Files[0].Path)
	assert.Equal(t, "M", result.Files[0].Status)
}

func TestStatus_Untracked(t *testing.T) {
	dir := initTestRepo(t)
	commitFile(t, dir, "tracked.txt", "content", "add tracked")

	// Create untracked file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new"), 0o644))

	ctx := context.Background()
	result, err := Status(ctx, dir)
	assert.NoError(t, err)
	assert.False(t, result.IsClean)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "untracked.txt", result.Files[0].Path)
	assert.Equal(t, "?", result.Files[0].Status)
}

// --- IsIgnored Tests ---

func TestIsIgnored(t *testing.T) {
	dir := initTestRepo(t)

	// Create .gitignore
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\nbuild/\n"), 0o644))
	commitFile(t, dir, ".gitignore", "*.log\nbuild/\n", "add gitignore")

	ctx := context.Background()

	// Ignored files
	assert.True(t, IsIgnored(ctx, dir, "test.log"))
	assert.True(t, IsIgnored(ctx, dir, "debug.log"))
	assert.True(t, IsIgnored(ctx, dir, "build/output"))

	// Non-ignored files
	assert.False(t, IsIgnored(ctx, dir, "main.go"))
	assert.False(t, IsIgnored(ctx, dir, "test.txt"))
}

// --- FilterIgnored Tests ---

func TestFilterIgnored(t *testing.T) {
	dir := initTestRepo(t)

	// Create .gitignore
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n*.tmp\n"), 0o644))
	commitFile(t, dir, ".gitignore", "*.log\n*.tmp\n", "add gitignore")

	ctx := context.Background()
	paths := []string{"main.go", "debug.log", "data.tmp", "readme.md"}
	filtered := FilterIgnored(ctx, dir, paths)

	assert.Contains(t, filtered, "main.go")
	assert.Contains(t, filtered, "readme.md")
	assert.NotContains(t, filtered, "debug.log")
	assert.NotContains(t, filtered, "data.tmp")
}

// --- Diff Tests ---

func TestDiff(t *testing.T) {
	dir := initTestRepo(t)
	commitFile(t, dir, "file.txt", "line1\nline2\n", "add file")

	// Modify the file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\nline2\nline3\n"), 0o644))

	ctx := context.Background()
	diff, err := Diff(ctx, dir)
	assert.NoError(t, err)
	assert.Contains(t, diff, "+line3")
}

// --- Log Tests ---

func TestLog(t *testing.T) {
	dir := initTestRepo(t)
	commitFile(t, dir, "file1.txt", "a", "first commit")
	commitFile(t, dir, "file2.txt", "b", "second commit")

	ctx := context.Background()
	log, err := Log(ctx, dir, 5)
	assert.NoError(t, err)
	assert.Contains(t, log, "first commit")
	assert.Contains(t, log, "second commit")
}

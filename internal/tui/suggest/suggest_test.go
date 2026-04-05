package suggest

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandProviderMatch(t *testing.T) {
	p := &CommandProvider{Commands: []string{"help", "model", "compact"}}

	assert.True(t, p.Match("/he", 3), "should match input starting with /")
	assert.True(t, p.Match("/", 1), "should match bare /")
	assert.False(t, p.Match("hello", 5), "should not match plain text")
	assert.False(t, p.Match("", 0), "should not match empty input")
	assert.False(t, p.Match("@file", 5), "should not match @-prefixed input")
}

func TestCommandProviderSuggest(t *testing.T) {
	p := &CommandProvider{Commands: []string{"help", "model", "compact", "cost", "clear"}}

	// Full prefix match
	results := p.Suggest("/he", 3)
	require.Equal(t, 1, len(results))
	assert.Equal(t, "/help", results[0].Text)
	assert.Equal(t, "/", results[0].Icon)

	// Partial match returns multiple
	results = p.Suggest("/c", 2)
	assert.Equal(t, 3, len(results)) // compact, cost, clear

	// No match
	results = p.Suggest("/xyz", 4)
	assert.Equal(t, 0, len(results))

	// Bare / returns all
	results = p.Suggest("/", 1)
	assert.Equal(t, 5, len(results))
}

func TestFilePathProviderMatch(t *testing.T) {
	p := &FilePathProvider{WorkingDir: "/tmp"}

	assert.True(t, p.Match("@src/", 5), "should match @ in input")
	assert.True(t, p.Match("look at @file.go", 17), "should match @ anywhere")
	assert.False(t, p.Match("hello world", 11), "should not match without @")
	assert.False(t, p.Match("/help", 5), "should not match slash commands")
}

func TestFilePathProviderSuggest(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "internal"), 0755)

	p := &FilePathProvider{WorkingDir: tmpDir}

	// @ with empty path returns all entries
	results := p.Suggest("@", 1)
	require.GreaterOrEqual(t, len(results), 3, "should find main.go, util.go, and internal/")

	// Filter by prefix
	results = p.Suggest("@main", 5)
	require.Equal(t, 1, len(results))
	assert.Equal(t, "main.go", results[0].Text)
	assert.Equal(t, "file", results[0].Description)

	// Directory entries have trailing slash
	results = p.Suggest("@int", 4)
	require.Equal(t, 1, len(results))
	assert.Equal(t, "internal/", results[0].Text)
	assert.Equal(t, "dir", results[0].Description)
}

func TestShellHistoryProviderMatch(t *testing.T) {
	p := &ShellHistoryProvider{History: []string{"ls -la", "git status"}}

	assert.True(t, p.Match("!", 1), "should match ! prefix")
	assert.True(t, p.Match("!git", 4), "should match ! with partial")
	assert.True(t, p.Match("", 0), "should match empty input")
	assert.False(t, p.Match("/help", 5), "should not match commands")
	assert.False(t, p.Match("hello", 5), "should not match regular text")
}

func TestShellHistoryProviderSuggest(t *testing.T) {
	p := &ShellHistoryProvider{
		History: []string{"ls -la", "git status", "git log --oneline", "go test ./..."},
	}

	// Empty prefix returns all
	results := p.Suggest("", 0)
	assert.Equal(t, 4, len(results))

	// Filter by partial after !
	results = p.Suggest("!git", 4)
	assert.Equal(t, 2, len(results))
	assert.Equal(t, "!git status", results[0].Text)
	assert.Equal(t, "!", results[0].Icon)
}

func TestSuggestionSelection(t *testing.T) {
	m := NewSuggestModel()

	// Simulate receiving suggestions
	items := []Suggestion{
		{Text: "/help", Icon: "/", Provider: "command"},
		{Text: "/model", Icon: "/", Provider: "command"},
		{Text: "/compact", Icon: "/", Provider: "command"},
	}
	m, _ = m.Update(SuggestionsReadyMsg{Items: items})
	assert.True(t, m.active)
	assert.Equal(t, 0, m.cursor)

	// Tab cycles forward
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, 1, m.cursor)

	// Down arrow also cycles forward
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	assert.Equal(t, 2, m.cursor)

	// Tab wraps around
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, 0, m.cursor)

	// Up arrow cycles backward (wraps)
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	assert.Equal(t, 2, m.cursor)
}

func TestSuggestionAccept(t *testing.T) {
	m := NewSuggestModel()

	items := []Suggestion{
		{Text: "/help", Icon: "/", Provider: "command"},
		{Text: "/model", Icon: "/", Provider: "command"},
	}
	m, _ = m.Update(SuggestionsReadyMsg{Items: items})

	// Move to second item
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, 1, m.cursor)

	// Press Enter to accept
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.False(t, m.active, "should dismiss after accept")

	// Verify the SuggestionAcceptMsg
	require.NotNil(t, cmd)
	msg := cmd()
	acceptMsg, ok := msg.(SuggestionAcceptMsg)
	require.True(t, ok, "expected SuggestionAcceptMsg, got %T", msg)
	assert.Equal(t, "/model", acceptMsg.Text)
}

func TestSuggestionDismiss(t *testing.T) {
	m := NewSuggestModel()

	items := []Suggestion{
		{Text: "/help", Icon: "/", Provider: "command"},
	}
	m, _ = m.Update(SuggestionsReadyMsg{Items: items})
	assert.True(t, m.active)

	// Escape dismisses
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	assert.False(t, m.active)
	assert.Nil(t, m.items)
}

func TestSuggestModelEmptySuggestions(t *testing.T) {
	m := NewSuggestModel()

	// Empty suggestions should not activate
	m, _ = m.Update(SuggestionsReadyMsg{Items: nil})
	assert.False(t, m.active)

	m, _ = m.Update(SuggestionsReadyMsg{Items: []Suggestion{}})
	assert.False(t, m.active)
}

func TestSuggestModelView(t *testing.T) {
	m := NewSuggestModel()
	m.SetWidth(50)

	items := []Suggestion{
		{Text: "/help", Description: "Show help", Icon: "/", Provider: "command"},
		{Text: "/model", Description: "Switch model", Icon: "/", Provider: "command"},
	}
	m, _ = m.Update(SuggestionsReadyMsg{Items: items})

	view := m.View()
	assert.NotEmpty(t, view, "active suggestions should render a view")

	// When not active, view should be empty
	m.active = false
	assert.Empty(t, m.View())
}

func TestSuggestionProviderInterface(t *testing.T) {
	// Verify all providers implement the interface
	var _ SuggestionProvider = &CommandProvider{}
	var _ SuggestionProvider = &FilePathProvider{}
	var _ SuggestionProvider = &ShellHistoryProvider{}
}

func TestOnInputChangeWithProvider(t *testing.T) {
	cp := &CommandProvider{Commands: []string{"help", "model"}}
	m := NewSuggestModel(cp)

	// Input starting with "/" should produce a command
	cmd := m.OnInputChange("/h", 2)
	require.NotNil(t, cmd, "should return a command for matching provider")

	// Input not matching any provider should dismiss
	cmd = m.OnInputChange("hello world", 11)
	assert.Nil(t, cmd, "should return nil when no provider matches")
	assert.False(t, m.active, "should dismiss when no provider matches")
}

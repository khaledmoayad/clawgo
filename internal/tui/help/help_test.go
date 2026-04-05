package help

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleCommands() []HelpEntry {
	return []HelpEntry{
		{Name: "/help", Description: "Show all available commands", Category: "info"},
		{Name: "/compact", Description: "Compress conversation context", Category: "context"},
		{Name: "/model", Description: "Switch AI model", Category: "config"},
		{Name: "/cost", Description: "Show session costs", Category: "info"},
		{Name: "/clear", Description: "Clear the conversation", Category: "context"},
	}
}

func sampleKeybindings() []HelpEntry {
	return []HelpEntry{
		{Name: "Ctrl+C", Description: "Quit the application", Category: "general"},
		{Name: "Ctrl+K", Description: "Jump to message", Category: "navigation"},
		{Name: "Escape", Description: "Cancel current action", Category: "general"},
	}
}

func TestHelpTabCycling(t *testing.T) {
	m := NewHelpModel(sampleCommands(), sampleKeybindings())
	_ = m.Show()

	// Starts on Commands tab
	assert.Equal(t, TabCommands, m.activeTab)

	// Tab -> Keybindings
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, TabKeybindings, m.activeTab)

	// Tab -> Tips
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, TabTips, m.activeTab)

	// Tab -> wraps to Commands
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, TabCommands, m.activeTab)

	// Shift+Tab -> wraps to Tips
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	assert.Equal(t, TabTips, m.activeTab)

	// Shift+Tab -> Keybindings
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	assert.Equal(t, TabKeybindings, m.activeTab)
}

func TestHelpSearchFilter(t *testing.T) {
	m := NewHelpModel(sampleCommands(), sampleKeybindings())
	_ = m.Show()

	// Initially all commands visible
	assert.Equal(t, 5, m.FilteredCount())

	// Use SetFilter to test the filtering logic directly.
	m.SetFilter("cost")
	require.Equal(t, 1, m.FilteredCount())
	assert.Equal(t, "/cost", m.filteredItems[0].Name)

	// Switch to Keybindings tab -- filter reapplies on tab change
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Equal(t, TabKeybindings, m.activeTab)
	// "cost" should not match any keybindings
	assert.Equal(t, 0, m.FilteredCount())

	// Clear filter and switch back -- all keybindings visible
	m.SetFilter("")
	assert.Equal(t, 3, m.FilteredCount())
}

func TestHelpDismiss(t *testing.T) {
	m := NewHelpModel(sampleCommands(), sampleKeybindings())
	_ = m.Show()
	assert.True(t, m.IsActive())

	// Press Escape
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	assert.False(t, m.IsActive())

	// Verify DismissHelpMsg is sent
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(DismissHelpMsg)
	assert.True(t, ok, "expected DismissHelpMsg, got %T", msg)
}

func TestHelpViewRendersTabBar(t *testing.T) {
	m := NewHelpModel(sampleCommands(), sampleKeybindings())
	_ = m.Show()

	view := m.View(80, 30)
	// Strip ANSI escape sequences for content assertions
	plain := stripAnsi(view)
	assert.True(t, strings.Contains(plain, "Commands"), "view should contain Commands tab")
	assert.True(t, strings.Contains(plain, "Keybindings"), "view should contain Keybindings tab")
	assert.True(t, strings.Contains(plain, "Tips"), "view should contain Tips tab")
	assert.True(t, strings.Contains(plain, "Help"), "view should contain Help title")
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func TestHelpScrolling(t *testing.T) {
	m := NewHelpModel(sampleCommands(), sampleKeybindings())
	_ = m.Show()

	// Start at offset 0
	assert.Equal(t, 0, m.scrollOffset)

	// Scroll down
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	assert.Equal(t, 1, m.scrollOffset)

	// Scroll up
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	assert.Equal(t, 0, m.scrollOffset)

	// Can't scroll past 0
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	assert.Equal(t, 0, m.scrollOffset)
}

func TestHelpInactiveNoOp(t *testing.T) {
	m := NewHelpModel(sampleCommands(), sampleKeybindings())
	// Not shown -- should be inactive
	assert.False(t, m.IsActive())

	// Update should be a no-op
	m2, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	assert.Nil(t, cmd)
	assert.Equal(t, m.activeTab, m2.activeTab)
}

func TestHelpTipsPopulated(t *testing.T) {
	m := NewHelpModel(nil, nil)
	_ = m.Show()

	// Switch to tips tab
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})) // keybindings
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})) // tips

	assert.Equal(t, TabTips, m.activeTab)
	assert.GreaterOrEqual(t, len(m.filteredItems), 10, "should have at least 10 built-in tips")
}

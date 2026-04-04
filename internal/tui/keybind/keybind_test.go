package keybind

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKeyCombo_CtrlC(t *testing.T) {
	combo, err := ParseKeyCombo("ctrl+c")
	require.NoError(t, err)
	assert.Equal(t, rune('c'), combo.Code)
	assert.Equal(t, tea.ModCtrl, combo.Mod)
}

func TestParseKeyCombo_ShiftEnter(t *testing.T) {
	combo, err := ParseKeyCombo("shift+enter")
	require.NoError(t, err)
	assert.Equal(t, tea.KeyEnter, combo.Code)
	assert.Equal(t, tea.ModShift, combo.Mod)
}

func TestParseKeyCombo_Escape(t *testing.T) {
	combo, err := ParseKeyCombo("escape")
	require.NoError(t, err)
	assert.Equal(t, tea.KeyEscape, combo.Code)
	assert.Equal(t, tea.KeyMod(0), combo.Mod)
}

func TestParseKeyCombo_SingleChar(t *testing.T) {
	combo, err := ParseKeyCombo("a")
	require.NoError(t, err)
	assert.Equal(t, rune('a'), combo.Code)
	assert.Equal(t, tea.KeyMod(0), combo.Mod)
}

func TestParseKeyCombo_CtrlShiftA(t *testing.T) {
	combo, err := ParseKeyCombo("ctrl+shift+a")
	require.NoError(t, err)
	assert.Equal(t, rune('a'), combo.Code)
	assert.Equal(t, tea.ModCtrl|tea.ModShift, combo.Mod)
}

func TestParseKeyCombo_Enter(t *testing.T) {
	combo, err := ParseKeyCombo("enter")
	require.NoError(t, err)
	assert.Equal(t, tea.KeyEnter, combo.Code)
	assert.Equal(t, tea.KeyMod(0), combo.Mod)
}

func TestParseKeyCombo_Invalid(t *testing.T) {
	_, err := ParseKeyCombo("invalid")
	assert.Error(t, err)
}

func TestParseKeyCombo_EmptyString(t *testing.T) {
	_, err := ParseKeyCombo("")
	assert.Error(t, err)
}

func TestDefaultBindings(t *testing.T) {
	cfg := DefaultBindings()

	// Check submit is enter
	combo, ok := cfg.ComboFor(ActionSubmit)
	assert.True(t, ok)
	assert.Equal(t, tea.KeyEnter, combo.Code)
	assert.Equal(t, tea.KeyMod(0), combo.Mod)

	// Check quit is ctrl+c
	combo, ok = cfg.ComboFor(ActionQuit)
	assert.True(t, ok)
	assert.Equal(t, rune('c'), combo.Code)
	assert.Equal(t, tea.ModCtrl, combo.Mod)

	// Check escape
	combo, ok = cfg.ComboFor(ActionEscape)
	assert.True(t, ok)
	assert.Equal(t, tea.KeyEscape, combo.Code)
}

func TestLoadKeyBindings_NilMap(t *testing.T) {
	cfg, err := LoadKeyBindings(nil)
	require.NoError(t, err)
	// Should return defaults
	combo, ok := cfg.ComboFor(ActionSubmit)
	assert.True(t, ok)
	assert.Equal(t, tea.KeyEnter, combo.Code)
}

func TestLoadKeyBindings_EmptyMap(t *testing.T) {
	cfg, err := LoadKeyBindings(map[string]string{})
	require.NoError(t, err)
	combo, ok := cfg.ComboFor(ActionSubmit)
	assert.True(t, ok)
	assert.Equal(t, tea.KeyEnter, combo.Code)
}

func TestLoadKeyBindings_SubmitOverride(t *testing.T) {
	cfg, err := LoadKeyBindings(map[string]string{
		"submit": "ctrl+enter",
	})
	require.NoError(t, err)
	combo, ok := cfg.ComboFor(ActionSubmit)
	assert.True(t, ok)
	assert.Equal(t, tea.KeyEnter, combo.Code)
	assert.Equal(t, tea.ModCtrl, combo.Mod)
}

func TestLoadKeyBindings_QuitOverride(t *testing.T) {
	cfg, err := LoadKeyBindings(map[string]string{
		"quit": "ctrl+q",
	})
	require.NoError(t, err)
	combo, ok := cfg.ComboFor(ActionQuit)
	assert.True(t, ok)
	assert.Equal(t, rune('q'), combo.Code)
	assert.Equal(t, tea.ModCtrl, combo.Mod)
}

func TestLoadKeyBindings_InvalidCombo(t *testing.T) {
	_, err := LoadKeyBindings(map[string]string{
		"submit": "invalid",
	})
	assert.Error(t, err)
}

func TestResolveAction_Submit(t *testing.T) {
	cfg := DefaultBindings()
	k := tea.Key{Code: tea.KeyEnter, Mod: 0}
	action := cfg.ResolveAction(k)
	assert.Equal(t, ActionSubmit, action)
}

func TestResolveAction_Quit(t *testing.T) {
	cfg := DefaultBindings()
	k := tea.Key{Code: 'c', Mod: tea.ModCtrl}
	action := cfg.ResolveAction(k)
	assert.Equal(t, ActionQuit, action)
}

func TestResolveAction_Unknown(t *testing.T) {
	cfg := DefaultBindings()
	k := tea.Key{Code: 'z', Mod: 0}
	action := cfg.ResolveAction(k)
	assert.Equal(t, ActionNone, action)
}

func TestResolveAction_NewLine(t *testing.T) {
	cfg := DefaultBindings()
	k := tea.Key{Code: tea.KeyEnter, Mod: tea.ModShift}
	action := cfg.ResolveAction(k)
	assert.Equal(t, ActionNewLine, action)
}

func TestComboFor_NotFound(t *testing.T) {
	cfg := DefaultBindings()
	_, ok := cfg.ComboFor(Action("nonexistent"))
	assert.False(t, ok)
}

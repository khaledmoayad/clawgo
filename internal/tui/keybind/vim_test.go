package keybind

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewVimModel_StartsDisabled(t *testing.T) {
	vm := NewVimModel()
	assert.False(t, vm.IsEnabled())
	assert.True(t, vm.IsInsert()) // default is insert mode
}

func TestVimModel_Toggle(t *testing.T) {
	vm := NewVimModel()
	assert.False(t, vm.IsEnabled())

	vm.Toggle()
	assert.True(t, vm.IsEnabled())
	assert.True(t, vm.IsNormal()) // enabling starts in normal mode

	vm.Toggle()
	assert.False(t, vm.IsEnabled())
	assert.True(t, vm.IsInsert()) // disabling resets to insert
}

func TestVimModel_SetEnabled(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)
	assert.True(t, vm.IsEnabled())
	assert.True(t, vm.IsNormal())

	vm.SetEnabled(false)
	assert.False(t, vm.IsEnabled())
	assert.True(t, vm.IsInsert())
}

func TestVimModel_Disabled_PassThrough(t *testing.T) {
	vm := NewVimModel()
	k := tea.Key{Code: 'h'}
	action, consumed := vm.HandleKey(k)
	assert.Equal(t, ActionNone, action)
	assert.False(t, consumed)
}

func TestVimModel_Normal_HJKL(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// h = left (consumed, no scroll action)
	action, consumed := vm.HandleKey(tea.Key{Code: 'h'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)

	// j = scroll down
	action, consumed = vm.HandleKey(tea.Key{Code: 'j'})
	assert.Equal(t, ActionScrollDown, action)
	assert.True(t, consumed)

	// k = scroll up
	action, consumed = vm.HandleKey(tea.Key{Code: 'k'})
	assert.Equal(t, ActionScrollUp, action)
	assert.True(t, consumed)

	// l = right (consumed, no scroll action)
	action, consumed = vm.HandleKey(tea.Key{Code: 'l'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
}

func TestVimModel_Normal_InsertSwitch(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)
	assert.True(t, vm.IsNormal())

	// i switches to insert
	action, consumed := vm.HandleKey(tea.Key{Code: 'i'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsInsert())
}

func TestVimModel_Insert_EscapeToNormal(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// Switch to insert first
	vm.HandleKey(tea.Key{Code: 'i'})
	assert.True(t, vm.IsInsert())

	// Escape goes back to normal
	action, consumed := vm.HandleKey(tea.Key{Code: tea.KeyEscape})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsNormal())
}

func TestVimModel_Normal_LineStartEnd(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// 0 = line start
	action, consumed := vm.HandleKey(tea.Key{Code: '0'})
	assert.Equal(t, ActionHome, action)
	assert.True(t, consumed)

	// $ = line end
	action, consumed = vm.HandleKey(tea.Key{Code: '$'})
	assert.Equal(t, ActionEnd, action)
	assert.True(t, consumed)
}

func TestVimModel_Normal_GG(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// First g sets pending
	action, consumed := vm.HandleKey(tea.Key{Code: 'g'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)

	// Second g = home
	action, consumed = vm.HandleKey(tea.Key{Code: 'g'})
	assert.Equal(t, ActionHome, action)
	assert.True(t, consumed)
}

func TestVimModel_Normal_ShiftG(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	action, consumed := vm.HandleKey(tea.Key{Code: 'G'})
	assert.Equal(t, ActionEnd, action)
	assert.True(t, consumed)
}

func TestVimModel_Normal_DD(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// First d sets pending
	action, consumed := vm.HandleKey(tea.Key{Code: 'd'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)

	// Second d = delete line
	action, consumed = vm.HandleKey(tea.Key{Code: 'd'})
	assert.Equal(t, ActionDeleteLine, action)
	assert.True(t, consumed)
}

func TestVimModel_Normal_Slash(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	action, consumed := vm.HandleKey(tea.Key{Code: '/'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed) // consumed but no action yet (future stub)
}

func TestVimModel_Normal_EscapeStaysNormal(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)
	assert.True(t, vm.IsNormal())

	action, consumed := vm.HandleKey(tea.Key{Code: tea.KeyEscape})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsNormal())
}

func TestVimModel_Normal_UnknownKeyPassThrough(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// Unknown key in normal mode passes through
	action, consumed := vm.HandleKey(tea.Key{Code: 'z'})
	assert.Equal(t, ActionNone, action)
	assert.False(t, consumed)
}

func TestVimModel_ModeString(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)
	assert.Equal(t, "NORMAL", vm.ModeString())

	vm.HandleKey(tea.Key{Code: 'i'})
	assert.Equal(t, "INSERT", vm.ModeString())
}

func TestVimModel_Mode(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)
	assert.Equal(t, ModeNormal, vm.Mode())

	vm.HandleKey(tea.Key{Code: 'i'})
	assert.Equal(t, ModeInsert, vm.Mode())
}

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

	// Second g = scroll to top
	action, consumed = vm.HandleKey(tea.Key{Code: 'g'})
	assert.Equal(t, ActionScrollToTop, action)
	assert.True(t, consumed)
}

func TestVimModel_Normal_ShiftG(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// G = scroll to bottom
	action, consumed := vm.HandleKey(tea.Key{Code: 'G'})
	assert.Equal(t, ActionScrollToBottom, action)
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

	// / enters search mode
	action, consumed := vm.HandleKey(tea.Key{Code: '/'})
	assert.Equal(t, ActionSearchStart, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsSearch())
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

// --- New tests for full vim mode ---

func TestVimOperatorMotion_DW(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// d enters operator pending
	action, consumed := vm.HandleKey(tea.Key{Code: 'd'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)

	// w completes the operator with delete range
	action, consumed = vm.HandleKey(tea.Key{Code: 'w'})
	assert.Equal(t, ActionDeleteRange, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsNormal()) // back to normal after completing
}

func TestVimOperatorMotion_CW(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// c + w = change word
	vm.HandleKey(tea.Key{Code: 'c'})
	action, consumed := vm.HandleKey(tea.Key{Code: 'w'})
	assert.Equal(t, ActionChangeRange, action)
	assert.True(t, consumed)
}

func TestVimOperatorMotion_YY(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// y + y = yank line
	vm.HandleKey(tea.Key{Code: 'y'})
	action, consumed := vm.HandleKey(tea.Key{Code: 'y'})
	assert.Equal(t, ActionYankRange, action)
	assert.True(t, consumed)
}

func TestVimCount(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// 3 accumulates count
	action, consumed := vm.HandleKey(tea.Key{Code: '3'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
	assert.Equal(t, 3, vm.effectiveCount())

	// w uses the count
	action, consumed = vm.HandleKey(tea.Key{Code: 'w'})
	assert.Equal(t, ActionNone, action) // word motion consumed
	assert.True(t, consumed)
}

func TestVimTextObject_CIW(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// c enters operator pending
	vm.HandleKey(tea.Key{Code: 'c'})

	// i starts text object scope
	action, consumed := vm.HandleKey(tea.Key{Code: 'i'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)

	// w completes text object, yielding change range
	action, consumed = vm.HandleKey(tea.Key{Code: 'w'})
	assert.Equal(t, ActionChangeRange, action)
	assert.True(t, consumed)
}

func TestVimTextObject_DAP(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// d + a + ( = delete around paren
	vm.HandleKey(tea.Key{Code: 'd'})
	vm.HandleKey(tea.Key{Code: 'a'})
	action, consumed := vm.HandleKey(tea.Key{Code: '('})
	assert.Equal(t, ActionDeleteRange, action)
	assert.True(t, consumed)
}

func TestVimSearch(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// / enters search mode
	action, _ := vm.HandleKey(tea.Key{Code: '/'})
	assert.Equal(t, ActionSearchStart, action)
	assert.True(t, vm.IsSearch())
	assert.Equal(t, "", vm.SearchQuery())

	// Type search query "hello"
	vm.HandleKey(tea.Key{Code: 'h'})
	vm.HandleKey(tea.Key{Code: 'e'})
	vm.HandleKey(tea.Key{Code: 'l'})
	vm.HandleKey(tea.Key{Code: 'l'})
	vm.HandleKey(tea.Key{Code: 'o'})
	assert.Equal(t, "hello", vm.SearchQuery())

	// Enter executes search and returns to normal
	action, consumed := vm.HandleKey(tea.Key{Code: tea.KeyEnter})
	assert.Equal(t, ActionSearchNext, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsNormal())
}

func TestVimSearchCancel(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// / then escape cancels
	vm.HandleKey(tea.Key{Code: '/'})
	vm.HandleKey(tea.Key{Code: 'h'})
	assert.Equal(t, "h", vm.SearchQuery())

	action, consumed := vm.HandleKey(tea.Key{Code: tea.KeyEscape})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsNormal())
	assert.Equal(t, "", vm.SearchQuery())
}

func TestVimSearchBackspace(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	vm.HandleKey(tea.Key{Code: '/'})
	vm.HandleKey(tea.Key{Code: 'a'})
	vm.HandleKey(tea.Key{Code: 'b'})
	assert.Equal(t, "ab", vm.SearchQuery())

	vm.HandleKey(tea.Key{Code: tea.KeyBackspace})
	assert.Equal(t, "a", vm.SearchQuery())
}

func TestVimUndo(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// Push an undo entry
	vm.PushUndo("hello world", 5)

	// u triggers undo action
	action, consumed := vm.HandleKey(tea.Key{Code: 'u'})
	assert.Equal(t, ActionUndo, action)
	assert.True(t, consumed)

	// Verify undo stack works
	entry, ok := vm.Undo()
	assert.True(t, ok)
	assert.Equal(t, "hello world", entry.Text)
	assert.Equal(t, 5, entry.Cursor)

	// Empty stack returns false
	_, ok = vm.Undo()
	assert.False(t, ok)
}

func TestVimRedo(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// Push redo entry
	vm.PushRedo("modified text", 3)

	// Ctrl+R triggers redo
	action, consumed := vm.HandleKey(tea.Key{Code: 'r', Mod: tea.ModCtrl})
	assert.Equal(t, ActionRedo, action)
	assert.True(t, consumed)

	// Verify redo stack works
	entry, ok := vm.Redo()
	assert.True(t, ok)
	assert.Equal(t, "modified text", entry.Text)
}

func TestVimScrollKeys(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	tests := []struct {
		key    tea.Key
		action Action
	}{
		{tea.Key{Code: 'j'}, ActionScrollDown},
		{tea.Key{Code: 'k'}, ActionScrollUp},
		{tea.Key{Code: 'd', Mod: tea.ModCtrl}, ActionHalfPageDown},
		{tea.Key{Code: 'u', Mod: tea.ModCtrl}, ActionHalfPageUp},
		{tea.Key{Code: 'G'}, ActionScrollToBottom},
		{tea.Key{Code: 'f', Mod: tea.ModCtrl}, ActionPageDown},
		{tea.Key{Code: 'b', Mod: tea.ModCtrl}, ActionPageUp},
	}

	for _, tt := range tests {
		action, consumed := vm.HandleKey(tt.key)
		assert.Equal(t, tt.action, action, "key %v should produce %s", tt.key, tt.action)
		assert.True(t, consumed, "key %v should be consumed", tt.key)
	}

	// gg = scroll to top (two key sequence)
	vm.HandleKey(tea.Key{Code: 'g'})
	action, consumed := vm.HandleKey(tea.Key{Code: 'g'})
	assert.Equal(t, ActionScrollToTop, action)
	assert.True(t, consumed)
}

func TestVimPaste(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	action, consumed := vm.HandleKey(tea.Key{Code: 'p'})
	assert.Equal(t, ActionPaste, action)
	assert.True(t, consumed)

	action, consumed = vm.HandleKey(tea.Key{Code: 'P'})
	assert.Equal(t, ActionPasteBefore, action)
	assert.True(t, consumed)
}

func TestVimSearchN(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// n = next match
	action, consumed := vm.HandleKey(tea.Key{Code: 'n'})
	assert.Equal(t, ActionSearchNext, action)
	assert.True(t, consumed)

	// N = previous match
	action, consumed = vm.HandleKey(tea.Key{Code: 'N'})
	assert.Equal(t, ActionSearchPrev, action)
	assert.True(t, consumed)
}

func TestVimCC(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// c + c = change line
	vm.HandleKey(tea.Key{Code: 'c'})
	action, consumed := vm.HandleKey(tea.Key{Code: 'c'})
	assert.Equal(t, ActionChangeRange, action)
	assert.True(t, consumed)
}

func TestVimEscapeCancelsPending(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// d sets operator pending
	vm.HandleKey(tea.Key{Code: 'd'})
	assert.Equal(t, ModeOperatorPending, vm.Mode())

	// Escape cancels
	action, consumed := vm.HandleKey(tea.Key{Code: tea.KeyEscape})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)
	assert.True(t, vm.IsNormal())
}

func TestVimX(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	action, consumed := vm.HandleKey(tea.Key{Code: 'x'})
	assert.Equal(t, ActionDeleteRange, action)
	assert.True(t, consumed)
}

// --- Operator/Motion/TextObject unit tests ---

func TestResolveMotion_Word(t *testing.T) {
	text := "hello world foo"
	pos := ResolveMotion(MotionWord, text, 0, 1, 0)
	assert.Equal(t, 6, pos) // "world"

	pos = ResolveMotion(MotionWord, text, 0, 2, 0)
	assert.Equal(t, 12, pos) // "foo"
}

func TestResolveMotion_WordBack(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion(MotionWordBack, text, 8, 1, 0)
	assert.Equal(t, 6, pos) // start of "world"
}

func TestResolveMotion_WordEnd(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion(MotionWordEnd, text, 0, 1, 0)
	assert.Equal(t, 4, pos) // 'o' of "hello"
}

func TestResolveMotion_LineStartEnd(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion(MotionLineStart, text, 5, 1, 0)
	assert.Equal(t, 0, pos)

	pos = ResolveMotion(MotionLineEnd, text, 0, 1, 0)
	assert.Equal(t, 10, pos) // last char
}

func TestResolveMotion_FindChar(t *testing.T) {
	text := "hello world"
	pos := ResolveMotion(MotionFindChar, text, 0, 1, 'o')
	assert.Equal(t, 4, pos) // 'o' in hello

	pos = ResolveMotion(MotionTillChar, text, 0, 1, 'o')
	assert.Equal(t, 3, pos) // one before 'o'
}

func TestResolveMotion_TopBottom(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := ResolveMotion(MotionTop, text, 10, 1, 0)
	assert.Equal(t, 0, pos)

	pos = ResolveMotion(MotionBottom, text, 0, 1, 0)
	assert.Equal(t, 12, pos) // start of "line3"
}

func TestMotionRange(t *testing.T) {
	text := "hello world"
	start, end := MotionRange(MotionWord, text, 0, 1, 0)
	assert.Equal(t, 0, start)
	assert.Equal(t, 6, end)
}

func TestExecuteOperator_Delete(t *testing.T) {
	text := "hello world"
	result := ExecuteOperator(OpDelete, text, 0, 5)
	assert.Equal(t, " world", result.NewText)
	assert.Equal(t, "hello", result.DeletedText)
	assert.False(t, result.EnterInsert)
}

func TestExecuteOperator_Change(t *testing.T) {
	text := "hello world"
	result := ExecuteOperator(OpChange, text, 0, 5)
	assert.Equal(t, " world", result.NewText)
	assert.Equal(t, "hello", result.DeletedText)
	assert.True(t, result.EnterInsert)
	assert.Equal(t, 0, result.NewCursor)
}

func TestExecuteOperator_Yank(t *testing.T) {
	text := "hello world"
	result := ExecuteOperator(OpYank, text, 0, 5)
	assert.Equal(t, "hello world", result.NewText) // text unchanged
	assert.Equal(t, "hello", result.DeletedText)   // yanked text
}

func TestExecuteLineOp_DD(t *testing.T) {
	text := "line1\nline2\nline3"
	result := ExecuteLineOp(OpDelete, text, 0, 1)
	assert.Equal(t, "line2\nline3", result.NewText)
	assert.True(t, result.Linewise)
}

func TestExecuteLineOp_YY(t *testing.T) {
	text := "line1\nline2\nline3"
	result := ExecuteLineOp(OpYank, text, 0, 1)
	assert.Equal(t, text, result.NewText) // text unchanged
	assert.Contains(t, result.DeletedText, "line1")
	assert.True(t, result.Linewise)
}

func TestPaste_CharacterWise(t *testing.T) {
	reg := Register{Content: "xyz", Linewise: false}
	newText, newCursor := Paste(reg, "hello", 2, true)
	assert.Equal(t, "helxyzlo", newText)
	assert.True(t, newCursor >= 3) // cursor at end of pasted text
}

func TestPaste_Linewise(t *testing.T) {
	reg := Register{Content: "new line\n", Linewise: true}
	newText, _ := Paste(reg, "line1\nline2", 0, true)
	assert.Contains(t, newText, "new line")
}

func TestResolveTextObject_InnerWord(t *testing.T) {
	text := "hello world"
	r := ResolveTextObject(ObjInnerWord, text, 1)
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.Start)
	assert.Equal(t, 5, r.End)
	assert.Equal(t, "hello", text[r.Start:r.End])
}

func TestResolveTextObject_AWord(t *testing.T) {
	text := "hello world"
	r := ResolveTextObject(ObjAWord, text, 1)
	assert.NotNil(t, r)
	assert.Equal(t, 0, r.Start)
	assert.Equal(t, 6, r.End) // includes trailing space
}

func TestResolveTextObject_InnerDQuote(t *testing.T) {
	text := `say "hello" now`
	r := ResolveTextObject(ObjInnerDQuote, text, 6)
	assert.NotNil(t, r)
	assert.Equal(t, "hello", text[r.Start:r.End])
}

func TestResolveTextObject_ADQuote(t *testing.T) {
	text := `say "hello" now`
	r := ResolveTextObject(ObjADQuote, text, 6)
	assert.NotNil(t, r)
	assert.Equal(t, `"hello"`, text[r.Start:r.End])
}

func TestResolveTextObject_InnerParen(t *testing.T) {
	text := "foo(bar baz)end"
	r := ResolveTextObject(ObjInnerParen, text, 5)
	assert.NotNil(t, r)
	assert.Equal(t, "bar baz", text[r.Start:r.End])
}

func TestResolveTextObject_AParen(t *testing.T) {
	text := "foo(bar baz)end"
	r := ResolveTextObject(ObjAParen, text, 5)
	assert.NotNil(t, r)
	assert.Equal(t, "(bar baz)", text[r.Start:r.End])
}

func TestResolveTextObject_InnerBrace(t *testing.T) {
	text := "func {body}"
	r := ResolveTextObject(ObjInnerBrace, text, 7)
	assert.NotNil(t, r)
	assert.Equal(t, "body", text[r.Start:r.End])
}

func TestResolveTextObject_NestedBrackets(t *testing.T) {
	text := "a[b[c]d]e"
	r := ResolveTextObject(ObjInnerBracket, text, 4)
	assert.NotNil(t, r)
	assert.Equal(t, "c", text[r.Start:r.End])
}

func TestTextObjectFromKeys(t *testing.T) {
	assert.Equal(t, ObjInnerWord, TextObjectFromKeys('i', 'w'))
	assert.Equal(t, ObjAWord, TextObjectFromKeys('a', 'w'))
	assert.Equal(t, ObjInnerDQuote, TextObjectFromKeys('i', '"'))
	assert.Equal(t, ObjADQuote, TextObjectFromKeys('a', '"'))
	assert.Equal(t, ObjInnerParen, TextObjectFromKeys('i', '('))
	assert.Equal(t, ObjAParen, TextObjectFromKeys('a', ')'))
	assert.Equal(t, ObjInnerBrace, TextObjectFromKeys('i', '{'))
	assert.Equal(t, ObjABrace, TextObjectFromKeys('a', 'B'))
	assert.Equal(t, ObjNone, TextObjectFromKeys('i', 'z'))
}

func TestFindInText(t *testing.T) {
	text := "hello world hello"
	matches := FindInText(text, "hello", true)
	assert.Equal(t, []int{0, 12}, matches)
}

func TestNextMatch(t *testing.T) {
	matches := []int{0, 12, 24}
	assert.Equal(t, 12, NextMatch(matches, 5))
	assert.Equal(t, 0, NextMatch(matches, 24)) // wraps
}

func TestPrevMatch(t *testing.T) {
	matches := []int{0, 12, 24}
	assert.Equal(t, 12, PrevMatch(matches, 20))
	assert.Equal(t, 24, PrevMatch(matches, 0)) // wraps
}

func TestVimCountMultiDigit(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// Type "12" then "j"
	vm.HandleKey(tea.Key{Code: '1'})
	vm.HandleKey(tea.Key{Code: '2'})
	assert.Equal(t, 12, vm.effectiveCount())

	action, consumed := vm.HandleKey(tea.Key{Code: 'j'})
	assert.Equal(t, ActionScrollDown, action)
	assert.True(t, consumed)
	// Count resets after use
	assert.Equal(t, 1, vm.effectiveCount())
}

func TestVimOperatorFind_DF(t *testing.T) {
	vm := NewVimModel()
	vm.SetEnabled(true)

	// d + f starts find in operator context
	vm.HandleKey(tea.Key{Code: 'd'})
	action, consumed := vm.HandleKey(tea.Key{Code: 'f'})
	assert.Equal(t, ActionNone, action)
	assert.True(t, consumed)

	// Next char completes the find motion
	action, consumed = vm.HandleKey(tea.Key{Code: 'x'})
	assert.Equal(t, ActionDeleteRange, action)
	assert.True(t, consumed)
}

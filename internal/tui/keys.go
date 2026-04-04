package tui

import tea "charm.land/bubbletea/v2"

// KeyMap defines all key bindings for the TUI.
// Bubble Tea v2 uses KeyPressMsg with Key.Code and Key.Mod for matching,
// so this struct holds the expected key codes and modifiers for each action.
type KeyMap struct {
	SubmitCode    rune      // Enter to submit
	SubmitMod     tea.KeyMod // No modifier for plain enter
	NewLineMod    tea.KeyMod // Shift modifier for shift+enter
	QuitCode      rune      // 'c' with Ctrl modifier
	QuitMod       tea.KeyMod
	EscapeCode    rune
	ApproveRune   rune // 'y'
	DenyRune      rune // 'n'
	AlwaysRune    rune // 'a'
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		SubmitCode:  tea.KeyEnter,
		SubmitMod:   0,
		NewLineMod:  tea.ModShift,
		QuitCode:    'c',
		QuitMod:     tea.ModCtrl,
		EscapeCode:  tea.KeyEscape,
		ApproveRune: 'y',
		DenyRune:    'n',
		AlwaysRune:  'a',
	}
}

// IsSubmit checks if a key press is the submit action (Enter without shift).
func (km KeyMap) IsSubmit(k tea.Key) bool {
	return k.Code == km.SubmitCode && k.Mod == km.SubmitMod
}

// IsNewLine checks if a key press is the new-line action (Shift+Enter).
func (km KeyMap) IsNewLine(k tea.Key) bool {
	return k.Code == tea.KeyEnter && k.Mod&tea.ModShift != 0
}

// IsQuit checks if a key press is the quit action (Ctrl+C).
func (km KeyMap) IsQuit(k tea.Key) bool {
	return k.Code == km.QuitCode && k.Mod&tea.ModCtrl != 0
}

// IsEscape checks if a key press is the escape action.
func (km KeyMap) IsEscape(k tea.Key) bool {
	return k.Code == km.EscapeCode
}

// IsApprove checks if a key press is the approve action (y/Y).
func (km KeyMap) IsApprove(k tea.Key) bool {
	return k.Code == km.ApproveRune || k.Code == 'Y'
}

// IsDeny checks if a key press is the deny action (n/N).
func (km KeyMap) IsDeny(k tea.Key) bool {
	return k.Code == km.DenyRune || k.Code == 'N'
}

// IsAlways checks if a key press is the always-approve action (a/A).
func (km KeyMap) IsAlways(k tea.Key) bool {
	return k.Code == km.AlwaysRune || k.Code == 'A'
}

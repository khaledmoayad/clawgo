// Package keybind provides config-driven keybinding resolution and vim mode
// for the ClawGo TUI. It mirrors the TypeScript useKeybindings hook pattern.
package keybind

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Action represents a semantic action triggered by a key press.
type Action string

// Action constants for all bindable actions.
const (
	ActionSubmit     Action = "submit"
	ActionQuit       Action = "quit"
	ActionNewLine    Action = "newline"
	ActionEscape     Action = "escape"
	ActionApprove    Action = "approve"
	ActionDeny       Action = "deny"
	ActionAlways     Action = "always"
	ActionScrollUp   Action = "scroll_up"
	ActionScrollDown Action = "scroll_down"
	ActionPageUp     Action = "page_up"
	ActionPageDown   Action = "page_down"
	ActionHome       Action = "home"
	ActionEnd        Action = "end"
	ActionDeleteLine Action = "delete_line"
	ActionNone       Action = ""
)

// KeyCombo represents a key combination (key code + modifiers).
type KeyCombo struct {
	Code rune
	Mod  tea.KeyMod
}

// KeyBindConfig holds the mapping between actions and key combos.
type KeyBindConfig struct {
	bindings map[Action]KeyCombo
	reverse  map[KeyCombo]Action
}

// knownKeyNames maps human-readable key names to Bubble Tea key codes.
var knownKeyNames = map[string]rune{
	"enter":     tea.KeyEnter,
	"escape":    tea.KeyEscape,
	"esc":       tea.KeyEscape,
	"tab":       tea.KeyTab,
	"space":     ' ',
	"backspace": tea.KeyBackspace,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
}

// knownModNames maps modifier names to Bubble Tea key modifiers.
var knownModNames = map[string]tea.KeyMod{
	"ctrl":  tea.ModCtrl,
	"shift": tea.ModShift,
	"alt":   tea.ModAlt,
}

// ParseKeyCombo parses a string like "ctrl+c", "shift+enter", "escape" into a KeyCombo.
// Format: [modifier+]...[modifier+]key
// Key names: enter, escape, tab, space, backspace, up, down, left, right, or single character.
// Modifier names: ctrl, shift, alt.
func ParseKeyCombo(s string) (KeyCombo, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return KeyCombo{}, fmt.Errorf("empty key combo string")
	}

	parts := strings.Split(s, "+")
	keyPart := parts[len(parts)-1]
	modParts := parts[:len(parts)-1]

	// Resolve modifiers
	var mod tea.KeyMod
	for _, mp := range modParts {
		m, ok := knownModNames[mp]
		if !ok {
			return KeyCombo{}, fmt.Errorf("unknown modifier: %q", mp)
		}
		mod |= m
	}

	// Resolve key
	if code, ok := knownKeyNames[keyPart]; ok {
		return KeyCombo{Code: code, Mod: mod}, nil
	}

	// Single character key
	if len(keyPart) == 1 {
		return KeyCombo{Code: rune(keyPart[0]), Mod: mod}, nil
	}

	return KeyCombo{}, fmt.Errorf("unknown key name: %q", keyPart)
}

// DefaultBindings returns the default keybinding configuration matching KeyMap defaults.
func DefaultBindings() KeyBindConfig {
	bindings := map[Action]KeyCombo{
		ActionSubmit:  {Code: tea.KeyEnter, Mod: 0},
		ActionNewLine: {Code: tea.KeyEnter, Mod: tea.ModShift},
		ActionQuit:    {Code: 'c', Mod: tea.ModCtrl},
		ActionEscape:  {Code: tea.KeyEscape, Mod: 0},
		ActionApprove: {Code: 'y', Mod: 0},
		ActionDeny:    {Code: 'n', Mod: 0},
		ActionAlways:  {Code: 'a', Mod: 0},
	}

	return buildConfig(bindings)
}

// LoadKeyBindings creates a KeyBindConfig from default bindings with optional overrides.
// Keys in the overrides map are action names ("submit", "quit", etc.),
// values are key combo strings ("ctrl+enter", "escape").
func LoadKeyBindings(overrides map[string]string) (KeyBindConfig, error) {
	cfg := DefaultBindings()

	if len(overrides) == 0 {
		return cfg, nil
	}

	for actionName, comboStr := range overrides {
		combo, err := ParseKeyCombo(comboStr)
		if err != nil {
			return KeyBindConfig{}, fmt.Errorf("invalid key combo for action %q: %w", actionName, err)
		}
		action := Action(actionName)
		// Remove old reverse mapping if it existed
		if oldCombo, ok := cfg.bindings[action]; ok {
			delete(cfg.reverse, oldCombo)
		}
		cfg.bindings[action] = combo
		cfg.reverse[combo] = action
	}

	return cfg, nil
}

// ResolveAction looks up a key press in the reverse map and returns the matching action.
func (c KeyBindConfig) ResolveAction(k tea.Key) Action {
	combo := KeyCombo{Code: k.Code, Mod: k.Mod}
	if action, ok := c.reverse[combo]; ok {
		return action
	}
	return ActionNone
}

// ComboFor returns the key combo assigned to an action.
func (c KeyBindConfig) ComboFor(action Action) (KeyCombo, bool) {
	combo, ok := c.bindings[action]
	return combo, ok
}

// FormatCombo returns a human-readable string for a KeyCombo (e.g., "Ctrl+C").
func FormatCombo(c KeyCombo) string {
	var parts []string
	if c.Mod&tea.ModCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if c.Mod&tea.ModShift != 0 {
		parts = append(parts, "Shift")
	}
	if c.Mod&tea.ModAlt != 0 {
		parts = append(parts, "Alt")
	}

	// Reverse lookup key name
	for name, code := range knownKeyNames {
		if code == c.Code {
			parts = append(parts, strings.Title(name)) //nolint:staticcheck
			return strings.Join(parts, "+")
		}
	}

	// Single character
	parts = append(parts, strings.ToUpper(string(c.Code)))
	return strings.Join(parts, "+")
}

// AllActions returns a list of all standard bindable actions.
func AllActions() []Action {
	return []Action{
		ActionSubmit, ActionQuit, ActionNewLine, ActionEscape,
		ActionApprove, ActionDeny, ActionAlways,
		ActionScrollUp, ActionScrollDown, ActionPageUp, ActionPageDown,
		ActionHome, ActionEnd,
	}
}

// buildConfig constructs a KeyBindConfig from a bindings map, building the reverse map.
func buildConfig(bindings map[Action]KeyCombo) KeyBindConfig {
	reverse := make(map[KeyCombo]Action, len(bindings))
	for action, combo := range bindings {
		reverse[combo] = action
	}
	return KeyBindConfig{
		bindings: bindings,
		reverse:  reverse,
	}
}

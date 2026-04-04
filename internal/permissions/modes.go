// Package permissions implements the permission mode system for ClawGo.
// It defines permission modes (default, plan, auto, bypass) and the
// CheckPermission logic that gates tool execution.
package permissions

// Mode represents the permission enforcement level.
type Mode int

const (
	// ModeDefault asks for write tools, allows read tools.
	ModeDefault Mode = iota
	// ModePlan asks for all mutations (plan review mode).
	ModePlan
	// ModeAuto auto-approves everything (yolo mode).
	ModeAuto
	// ModeBypass bypasses all permission checks.
	ModeBypass
)

// ModeFromString converts a string to Mode.
// Accepts: "default", "plan", "auto", "yolo", "bypass".
// Returns ModeDefault for unrecognized strings (safe fallback).
func ModeFromString(s string) Mode {
	switch s {
	case "default":
		return ModeDefault
	case "plan":
		return ModePlan
	case "auto", "yolo":
		return ModeAuto
	case "bypass":
		return ModeBypass
	default:
		return ModeDefault
	}
}

// String returns the string representation of a Mode.
func (m Mode) String() string {
	switch m {
	case ModeDefault:
		return "default"
	case ModePlan:
		return "plan"
	case ModeAuto:
		return "auto"
	case ModeBypass:
		return "bypass"
	default:
		return "default"
	}
}

package tui

import "testing"

func TestIsShellCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"!ls", true},
		{"!ls -la", true},
		{"! pwd", true},
		{"!echo hello world", true},
		{"/help", false},
		{"hello", false},
		{"!", false},         // empty command after !
		{"!  ", false},       // whitespace-only command
		{"", false},          // empty input
		{"  !ls", true},     // leading whitespace
		{"not!a command", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsShellCommand(tt.input)
			if got != tt.want {
				t.Errorf("IsShellCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseShellCommand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"!ls -la", "ls -la"},
		{"! pwd", "pwd"},
		{"!echo hello", "echo hello"},
		{"!  git status", "git status"},
		{"  !ls", "ls"},
		{"hello", ""},
		{"/help", ""},
		{"!", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseShellCommand(tt.input)
			if got != tt.want {
				t.Errorf("ParseShellCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

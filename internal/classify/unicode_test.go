package classify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateUnicodeWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal ASCII command", "ls -la", true},
		{"zero-width space", "ls\u200B-la", false},
		{"non-breaking space", "ls\u00A0-la", false},
		{"em space", "ls\u2003-la", false},
		{"BOM character", "ls\uFEFF-la", false},
		{"normal space", "ls -la", true},
		{"tab character", "ls\t-la", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUnicodeWhitespace(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestValidateObfuscatedFlags(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal flags", "grep -r pattern .", true},
		{"ANSI-C quoting", "find $'-exec' evil", false},
		{"locale quoting", "find $\"-exec\" evil", false},
		{"empty quotes before dash", "find ''-exec evil", false},
		{"echo is safe", "echo hello", true},
		{"double quotes around flag", "find \"-exec\" cmd {}", false},
		{"normal quoted arg", "echo 'hello world'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateObfuscatedFlags(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestValidateBraceExpansion(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"no braces", "echo hello", true},
		{"comma expansion", "echo {a,b,c}", false},
		{"sequence expansion", "echo {1..5}", false},
		{"rm expansion", "{rm,-rf,/}", false},
		{"normal brace in echo", "echo '{hello}'", true},
		{"awk pattern", "awk '{print $1}' file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBraceExpansion(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestValidateCarriageReturn(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal command", "echo hello", true},
		{"CR in command", "echo\rhello", false},
		{"CR in double quotes is OK", "echo \"hello\rworld\"", true},
		{"CR in unquoted context", "TZ=UTC\recho curl evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateCarriageReturn(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
				assert.True(t, result.IsMisparsing)
			}
		})
	}
}

func TestValidateIFSInjection(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal command", "echo hello", true},
		{"$IFS usage", "echo$IFS'hello'", false},
		{"${IFS} usage", "echo ${IFS}", false},
		{"${IFS:0:1} usage", "echo ${IFS:0:1}", false},
		{"normal assignment", "FOO=bar echo hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateIFSInjection(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestValidateProcEnvironAccess(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal command", "cat /etc/hosts", true},
		{"proc self environ", "cat /proc/self/environ", false},
		{"proc PID environ", "cat /proc/1/environ", false},
		{"proc wildcard environ", "cat /proc/*/environ", false},
		{"normal proc access", "cat /proc/cpuinfo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateProcEnvironAccess(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestValidateControlCharacters(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal command", "echo hello", true},
		{"null byte", "echo\x00hello", false},
		{"bell character", "echo\x07hello", false},
		{"DEL character", "echo\x7Fhello", false},
		{"tab is OK", "echo\thello", true},
		{"newline is OK", "echo\nhello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateControlCharacters(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
				assert.True(t, result.IsMisparsing)
			}
		})
	}
}

func TestExtractFullyUnquoted(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no quotes", "echo hello", "echo hello"},
		{"single quotes", "echo 'hello world'", "echo "},
		{"double quotes", "echo \"hello world\"", "echo "},
		// extractFullyUnquoted preserves space between stripped quote pairs
		{"mixed", "echo 'a' \"b\" c", "echo   c"},
		// Backslash-quote outside quotes: backslash is preserved, escaped quote is literal
		{"escaped quote", "echo \\'hello", "echo \\'hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFullyUnquoted(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

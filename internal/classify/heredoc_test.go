package classify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsHeredoc(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{"basic heredoc", "cat <<EOF\nhello\nEOF", true},
		{"quoted heredoc", "cat <<'EOF'\nhello\nEOF", true},
		{"dash heredoc", "cat <<-EOF\nhello\nEOF", true},
		{"herestring", "cat <<<hello", false},
		{"no heredoc", "echo hello", false},
		{"double less than in comparison", "test 1 -lt 2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsHeredoc(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSafeHeredoc(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			"safe heredoc with quoted delimiter",
			"echo $(cat <<'EOF'\nhello world\nEOF\n)",
			true,
		},
		{
			"no heredoc in substitution",
			"echo hello",
			false,
		},
		{
			"unquoted delimiter is not safe",
			"echo $(cat <<EOF\nhello\nEOF\n)",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSafeHeredoc(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateHeredoc(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantNil  bool
		wantAsk  bool
	}{
		{"no heredoc", "echo hello", true, false},
		{
			"unquoted heredoc with variable expansion",
			"cat <<EOF\nhello $HOME\nEOF",
			false, true,
		},
		{
			"unquoted heredoc with command substitution",
			"cat <<EOF\nhello $(whoami)\nEOF",
			false, true,
		},
		{
			"quoted heredoc is safe",
			"cat <<'EOF'\nhello $(whoami)\nEOF",
			true, false,
		},
		{
			"unquoted heredoc with backtick",
			"cat <<EOF\nhello `whoami`\nEOF",
			false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateHeredoc(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else if tt.wantAsk {
				assert.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestHasSafeHeredocSubstitution(t *testing.T) {
	assert.True(t, HasSafeHeredocSubstitution("echo $(cat <<'EOF'\nhello\nEOF\n)"))
	assert.False(t, HasSafeHeredocSubstitution("echo hello"))
}

func TestExtractHeredocs(t *testing.T) {
	delimiters := ExtractHeredocs("cat <<EOF\nhello\nEOF")
	assert.Contains(t, delimiters, "EOF")

	delimiters = ExtractHeredocs("echo hello")
	assert.Nil(t, delimiters)
}

package classify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyBashCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected ClassificationResult
	}{
		// Read-only commands
		{"ls is readonly", "ls", ClassifyReadOnly},
		{"cat file is readonly", "cat file.txt", ClassifyReadOnly},
		{"grep pattern file is readonly", "grep pattern file", ClassifyReadOnly},
		{"git status is readonly", "git status", ClassifyReadOnly},
		{"git log is readonly", "git log", ClassifyReadOnly},
		{"pwd is readonly", "pwd", ClassifyReadOnly},
		{"echo hello is readonly", "echo hello", ClassifyReadOnly},
		{"cd /tmp && ls is readonly", "cd /tmp && ls", ClassifyReadOnly},
		{"echo with quoting", "echo 'hello world'", ClassifyReadOnly},

		// Destructive commands -> Deny
		{"rm -rf / is deny", "rm -rf /", ClassifyDeny},
		{"sudo rm -rf / is deny", "sudo rm -rf /", ClassifyDeny},
		{"mkfs is deny", "mkfs /dev/sda1", ClassifyDeny},
		{"dd is deny", "dd if=/dev/zero of=/dev/sda", ClassifyDeny},

		// Ask commands (writes, network, risky)
		{"rm file.txt is ask", "rm file.txt", ClassifyAsk},
		{"echo redirect is ask", "echo hello > file.txt", ClassifyAsk},
		{"curl is ask", "curl https://example.com", ClassifyAsk},
		{"wget is ask", "wget https://example.com", ClassifyAsk},
		{"pip install is ask", "pip install foo", ClassifyAsk},
		{"npm install is ask", "npm install foo", ClassifyAsk},
		{"docker run is ask", "docker run ubuntu", ClassifyAsk},

		// Pipes
		{"pipe of readonly is readonly", "ls | grep foo", ClassifyReadOnly},
		{"mixed pipe is ask", "ls && rm file", ClassifyAsk},

		// Command substitution
		{"command substitution deny", "$(rm -rf /)", ClassifyDeny},
		{"backtick substitution deny", "`rm -rf /`", ClassifyDeny},

		// Edge cases
		{"unparseable is ask", "{{invalid", ClassifyAsk},
		{"empty command is ask", "", ClassifyAsk},

		// Additional edge cases
		{"heredoc is ask", "cat <<EOF\nhello\nEOF", ClassifyReadOnly},
		{"background process is ask", "sleep 10 &", ClassifyAsk},
		{"redirect output is ask", "ls > output.txt", ClassifyAsk},
		{"semicolon readonly", "ls; pwd", ClassifyReadOnly},
		{"git diff is readonly", "git diff", ClassifyReadOnly},
		{"git show is readonly", "git show HEAD", ClassifyReadOnly},
		{"git push is ask", "git push origin main", ClassifyAsk},
		{"git commit is ask", "git commit -m 'test'", ClassifyAsk},
		{"find is readonly", "find . -name '*.go'", ClassifyReadOnly},
		{"wc is readonly", "wc -l file.txt", ClassifyReadOnly},
		{"head is readonly", "head -n 10 file.txt", ClassifyReadOnly},
		{"tail is readonly", "tail -f file.txt", ClassifyReadOnly},
		{"sort is readonly", "sort file.txt", ClassifyReadOnly},
		{"uniq is readonly", "uniq file.txt", ClassifyReadOnly},
		{"mkdir is ask", "mkdir -p /tmp/test", ClassifyAsk},
		{"chmod is ask", "chmod 755 file.txt", ClassifyAsk},
		{"chown is ask", "chown user:group file.txt", ClassifyAsk},
		{"kill is ask", "kill -9 1234", ClassifyAsk},
		{"ssh is ask", "ssh user@host", ClassifyAsk},
		{"shred is deny", "shred /dev/sda", ClassifyDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ClassifyBashCommand(tt.command)
			assert.Equal(t, tt.expected, result, "command: %q", tt.command)
		})
	}
}

func TestExtractCommands(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected []string
	}{
		{"simple command", "ls", []string{"ls"}},
		{"command with args", "grep -r pattern .", []string{"grep"}},
		{"pipe", "ls | grep foo", []string{"ls", "grep"}},
		{"and chain", "ls && pwd", []string{"ls", "pwd"}},
		{"or chain", "ls || echo fail", []string{"ls", "echo"}},
		{"semicolons", "ls; pwd; echo hi", []string{"ls", "pwd", "echo"}},
		{"subshell", "(ls && pwd)", []string{"ls", "pwd"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := ExtractCommandNames(tt.command)
			assert.Equal(t, tt.expected, cmds)
		})
	}
}

func TestClassificationResultString(t *testing.T) {
	assert.Equal(t, "safe", ClassifySafe.String())
	assert.Equal(t, "readonly", ClassifyReadOnly.String())
	assert.Equal(t, "ask", ClassifyAsk.String())
	assert.Equal(t, "deny", ClassifyDeny.String())
}

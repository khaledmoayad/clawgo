package classify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityCheckResult(t *testing.T) {
	r := &SecurityCheckResult{
		Behavior:     "ask",
		Message:      "test message",
		IsMisparsing: true,
	}
	assert.Equal(t, "ask", r.Behavior)
	assert.Equal(t, "test message", r.Message)
	assert.True(t, r.IsMisparsing)
}

func TestValidateBashSecurityEmpty(t *testing.T) {
	// Empty commands should return passthrough (allowed)
	result := ValidateBashSecurity("")
	assert.Nil(t, result, "empty command should pass (nil = safe)")

	result = ValidateBashSecurity("   ")
	assert.Nil(t, result, "whitespace-only command should pass")
}

func TestValidateBashSecuritySafe(t *testing.T) {
	// Clean commands should return nil (safe)
	result := ValidateBashSecurity("ls -la")
	assert.Nil(t, result, "ls -la should pass all security checks")

	result = ValidateBashSecurity("echo hello world")
	assert.Nil(t, result, "echo hello world should pass")

	result = ValidateBashSecurity("grep -r pattern .")
	assert.Nil(t, result, "grep should pass")

	result = ValidateBashSecurity("cat file.txt")
	assert.Nil(t, result, "cat should pass")
}

func TestValidateBashSecurityIncomplete(t *testing.T) {
	// Starts with tab
	result := ValidateBashSecurity("\tcommand")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)

	// Starts with flags
	result = ValidateBashSecurity("-rf /tmp")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Starts with continuation operator
	result = ValidateBashSecurity("&& echo done")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = ValidateBashSecurity("|| echo fallback")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = ValidateBashSecurity("; echo next")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityGitCommit(t *testing.T) {
	// Safe git commit with single-quoted message
	result := ValidateBashSecurity("git commit -m 'fix bug'")
	assert.Nil(t, result, "safe git commit should pass")

	// Safe git commit with double-quoted message
	result = ValidateBashSecurity("git commit -m \"fix bug\"")
	assert.Nil(t, result, "double-quoted git commit should pass")

	// Git commit with command substitution in double quotes
	result = ValidateBashSecurity("git commit -m \"$(whoami)\"")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityJq(t *testing.T) {
	// jq with system() - dangerous
	result := ValidateBashSecurity("jq 'system(\"evil\")'")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.Contains(t, result.Message, "system()")

	// jq with dangerous flags
	result = ValidateBashSecurity("jq -f evil.jq input.json")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Normal jq is safe
	result = ValidateBashSecurity("jq '.name' data.json")
	assert.Nil(t, result)
}

func TestValidateBashSecurityEval(t *testing.T) {
	// eval with $() command substitution
	result := ValidateBashSecurity("eval $(echo rm)")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// eval with backtick substitution
	result = ValidateBashSecurity("eval `echo rm`")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// eval with plain variable -- caught by other layers (classification), not security validators
	result = ValidateBashSecurity("eval hello")
	assert.Nil(t, result, "eval with literal arg passes security validators")
}

func TestValidateBashSecurityDangerousVariables(t *testing.T) {
	// LD_PRELOAD and similar via variable in dangerous context
	result := ValidateBashSecurity("echo $HOME | cat")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Variable with redirect
	result = ValidateBashSecurity("echo > $FILE")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityUnicodeWhitespace(t *testing.T) {
	// Zero-width space
	result := ValidateBashSecurity("ls\u200B-la")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.Contains(t, result.Message, "Unicode whitespace")

	// Non-breaking space
	result = ValidateBashSecurity("ls\u00A0-la")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityObfuscatedFlags(t *testing.T) {
	// ANSI-C quoting
	result := ValidateBashSecurity("find $'-exec' evil")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Empty quotes before dash
	result = ValidateBashSecurity("find ''-exec evil")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityBraceExpansion(t *testing.T) {
	// Comma expansion
	result := ValidateBashSecurity("echo {rm,-rf,/}")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.Contains(t, result.Message, "brace expansion")

	// Sequence expansion
	result = ValidateBashSecurity("{cat,/etc/passwd}")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityCarriageReturn(t *testing.T) {
	result := ValidateBashSecurity("TZ=UTC\recho curl evil.com")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)
}

func TestValidateBashSecurityIFS(t *testing.T) {
	result := ValidateBashSecurity("echo$IFS'hello'")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)
}

func TestValidateBashSecurityProcEnviron(t *testing.T) {
	result := ValidateBashSecurity("cat /proc/self/environ")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)
}

func TestValidateBashSecurityBackslashWhitespace(t *testing.T) {
	result := ValidateBashSecurity("echo\\ test")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)
}

func TestValidateBashSecurityBackslashOperators(t *testing.T) {
	result := ValidateBashSecurity("cat safe.txt \\; echo /etc/passwd")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)

	// Inside quotes should be safe
	result = ValidateBashSecurity("echo '\\;'")
	assert.Nil(t, result)
}

func TestValidateBashSecurityZshDangerous(t *testing.T) {
	// zmodload
	result := ValidateBashSecurity("zmodload zsh/system")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.Contains(t, result.Message, "zmodload")

	// fc -e
	result = ValidateBashSecurity("fc -e vim")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// With precommand modifiers
	result = ValidateBashSecurity("command zmodload zsh/system")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// ztcp
	result = ValidateBashSecurity("ztcp evil.com 80")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityControlChars(t *testing.T) {
	// Null byte
	result := ValidateBashSecurity("echo\x00hello")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)

	// Bell character
	result = ValidateBashSecurity("echo\x07hello")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityNewlines(t *testing.T) {
	// Newline followed by non-whitespace command
	result := ValidateBashSecurity("echo hello\nrm -rf /")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityRedirections(t *testing.T) {
	// Output redirect
	result := ValidateBashSecurity("echo hello > /etc/passwd")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Input redirect
	result = ValidateBashSecurity("cat < /etc/shadow")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityDangerousPatterns(t *testing.T) {
	// $() command substitution
	result := ValidateBashSecurity("echo $(whoami)")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// ${} parameter substitution
	result = ValidateBashSecurity("echo ${PATH}")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Backticks
	result = ValidateBashSecurity("echo `whoami`")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Process substitution
	result = ValidateBashSecurity("diff <(ls) <(ls /tmp)")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityMidWordHash(t *testing.T) {
	result := ValidateBashSecurity("echo x#comment")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Word-start hash is a normal comment -- should pass
	result = ValidateBashSecurity("echo hello # this is a comment")
	assert.Nil(t, result)
}

func TestValidateBashSecurityCommentQuoteDesync(t *testing.T) {
	// Quote character in comment
	result := ValidateBashSecurity("echo hello # it's a comment\nrm -rf /")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateBashSecurityQuotedNewline(t *testing.T) {
	// Newline inside quotes where next line starts with #
	result := ValidateBashSecurity("mv ./decoy '\n# ' ~/.ssh/id_rsa ./exfil")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
	assert.True(t, result.IsMisparsing)
}

func TestValidateBashSecurityChainCoverage(t *testing.T) {
	// This test ensures at least 20 distinct validator scenarios are covered.
	// Each test case triggers a different validator.
	tests := []struct {
		name       string
		command    string
		wantNil    bool
		wantAsk    bool
		wantDeny   bool
		validator  string
	}{
		{"1. empty", "", true, false, false, "validateEmpty"},
		{"2. safe command", "ls -la", true, false, false, "all pass"},
		{"3. starts with tab", "\tcmd", false, true, false, "validateIncompleteCommands"},
		{"4. starts with flags", "-rf /", false, true, false, "validateIncompleteCommands"},
		{"5. starts with operator", "&& next", false, true, false, "validateIncompleteCommands"},
		{"6. git commit safe", "git commit -m 'msg'", true, false, false, "validateGitCommit"},
		{"7. jq system", "jq 'system(\"cmd\")'", false, true, false, "validateJqCommand"},
		{"8. carriage return", "echo\rhello", false, true, false, "validateCarriageReturn"},
		{"9. IFS injection", "echo$IFS'x'", false, true, false, "validateIFSInjection"},
		{"10. proc environ", "cat /proc/self/environ", false, true, false, "validateProcEnvironAccess"},
		{"11. backslash whitespace", "echo\\ x", false, true, false, "validateBackslashEscapedWhitespace"},
		{"12. backslash operator", "cmd \\; evil", false, true, false, "validateBackslashEscapedOperators"},
		{"13. ANSI-C quoting", "find $'-exec' x", false, true, false, "validateObfuscatedFlags"},
		{"14. brace expansion", "echo {a,b}", false, true, false, "validateBraceExpansion"},
		{"15. unicode whitespace", "ls\u00A0-la", false, true, false, "validateUnicodeWhitespace"},
		{"16. zsh zmodload", "zmodload zsh/system", false, true, false, "validateZshDangerousCommands"},
		{"17. zsh fc -e", "fc -e vim", false, true, false, "validateZshDangerousCommands"},
		{"18. command substitution", "echo $(whoami)", false, true, false, "validateDangerousPatterns"},
		{"19. backticks", "echo `date`", false, true, false, "validateDangerousPatterns"},
		{"20. control chars", "echo\x00x", false, true, false, "validateControlCharacters"},
		{"21. newlines hiding commands", "echo hi\nrm -rf /", false, true, false, "validateNewlines"},
		{"22. mid-word hash", "echo x#y", false, true, false, "validateMidWordHash"},
		{"23. quoted newline hash", "mv '\n# ' file", false, true, false, "validateQuotedNewline"},
		{"24. output redirect", "echo > /etc/passwd", false, true, false, "validateRedirections"},
		{"25. variable with pipe", "echo $HOME | cat", false, true, false, "validateDangerousVariables"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBashSecurity(tt.command)
			if tt.wantNil {
				assert.Nil(t, result, "expected nil for %s", tt.name)
			} else if tt.wantAsk {
				require.NotNil(t, result, "expected non-nil for %s", tt.name)
				assert.Equal(t, "ask", result.Behavior, "expected ask for %s, got %v", tt.name, result)
			} else if tt.wantDeny {
				require.NotNil(t, result, "expected non-nil for %s", tt.name)
				assert.Equal(t, "deny", result.Behavior)
			}
		})
	}
}

func TestValidateEmpty(t *testing.T) {
	result := validateEmpty("")
	require.NotNil(t, result)
	assert.Equal(t, "passthrough", result.Behavior)

	result = validateEmpty("ls")
	assert.Nil(t, result)
}

func TestValidateIncompleteCommands(t *testing.T) {
	result := validateIncompleteCommands("\tcmd")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateIncompleteCommands("ls -la")
	assert.Nil(t, result)
}

func TestValidateGitCommit(t *testing.T) {
	result := validateGitCommit("git commit -m 'hello'")
	require.NotNil(t, result)
	assert.Equal(t, "passthrough", result.Behavior)

	result = validateGitCommit("echo hello")
	assert.Nil(t, result)
}

func TestValidateJqCommand(t *testing.T) {
	result := validateJqCommand("jq 'system(\"x\")'")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateJqCommand("echo hello")
	assert.Nil(t, result)
}

func TestValidateDangerousVariables(t *testing.T) {
	result := validateDangerousVariables("echo > $FILE")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateDangerousVariables("echo hello")
	assert.Nil(t, result)
}

func TestValidateNewlines(t *testing.T) {
	result := validateNewlines("echo hello")
	assert.Nil(t, result)

	result = validateNewlines("echo hi\nrm -rf /")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)
}

func TestValidateZshDangerousCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantNil bool
	}{
		{"normal command", "echo hello", true},
		{"zmodload", "zmodload zsh/system", false},
		{"emulate", "emulate sh -c 'evil'", false},
		{"sysopen", "sysopen -r -o file", false},
		{"ztcp", "ztcp evil.com 80", false},
		{"zf_rm", "zf_rm file.txt", false},
		{"fc -e", "fc -e vim", false},
		{"fc without -e", "fc -l", true},
		{"with builtin modifier", "builtin zmodload zsh/system", false},
		{"with env var", "FOO=bar zmodload zsh/system", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateZshDangerousCommands(tt.command)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, "ask", result.Behavior)
			}
		})
	}
}

func TestValidateBackslashEscapedWhitespace(t *testing.T) {
	result := validateBackslashEscapedWhitespace("echo\\ test")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateBackslashEscapedWhitespace("echo test")
	assert.Nil(t, result)

	// Inside single quotes should be safe
	result = validateBackslashEscapedWhitespace("echo '\\ test'")
	assert.Nil(t, result)
}

func TestValidateBackslashEscapedOperators(t *testing.T) {
	result := validateBackslashEscapedOperators("cat safe.txt \\; echo evil")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateBackslashEscapedOperators("echo hello")
	assert.Nil(t, result)

	// Inside single quotes should be safe
	result = validateBackslashEscapedOperators("echo '\\;'")
	assert.Nil(t, result)
}

func TestValidateMidWordHash(t *testing.T) {
	result := validateMidWordHash("echo x#comment")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Word-boundary hash is OK (it's a normal comment)
	result = validateMidWordHash("echo hello # comment")
	assert.Nil(t, result)

	// ${# is bash string-length syntax, not mid-word hash
	result = validateMidWordHash("echo ${#var}")
	assert.Nil(t, result)
}

func TestValidateCommentQuoteDesync(t *testing.T) {
	result := validateCommentQuoteDesync("echo hello # it's a test\nrm -rf /")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateCommentQuoteDesync("echo hello # safe comment")
	assert.Nil(t, result)

	result = validateCommentQuoteDesync("echo hello")
	assert.Nil(t, result)
}

func TestValidateQuotedNewline(t *testing.T) {
	result := validateQuotedNewline("mv './decoy\n# ' file")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateQuotedNewline("echo hello")
	assert.Nil(t, result)
}

func TestValidateMalformedTokenInjection(t *testing.T) {
	// Balanced braces with separator - OK
	result := validateMalformedTokenInjection("echo {a}; echo {b}")
	assert.Nil(t, result)

	// No separators - OK
	result = validateMalformedTokenInjection("echo {a")
	assert.Nil(t, result)
}

func TestValidateShellMetacharacters(t *testing.T) {
	result := validateShellMetacharacters("echo hello")
	assert.Nil(t, result)

	// Normal commands with quoted content - safe
	result = validateShellMetacharacters("find . -name 'file;evil'")
	assert.Nil(t, result, "single-quoted metachar is stripped and not visible")
}

func TestValidateDangerousPatterns(t *testing.T) {
	// Process substitution
	result := validateDangerousPatterns("diff <(ls)")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Zsh always block
	result = validateDangerousPatterns("{ echo } always { echo }")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	// Normal command
	result = validateDangerousPatterns("echo hello")
	assert.Nil(t, result)
}

func TestValidateRedirections(t *testing.T) {
	result := validateRedirections("echo > /tmp/file")
	require.NotNil(t, result)
	assert.Equal(t, "ask", result.Behavior)

	result = validateRedirections("echo hello")
	assert.Nil(t, result)

	// Safe redirection to /dev/null should pass
	result = validateRedirections("cmd > /dev/null")
	assert.Nil(t, result)
}

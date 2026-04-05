package classify

import (
	"regexp"
	"strings"
)

// SecurityCheckResult represents the result of a security validation check.
type SecurityCheckResult struct {
	Behavior     string // "passthrough", "ask", "deny"
	Message      string
	IsMisparsing bool // If true, this is a misparsing check (gate-able)
}

// destructiveCommands are commands that are always denied because
// they can cause irreversible damage to the system.
var destructiveCommands = map[string]bool{
	"mkfs":   true,
	"dd":     true,
	"format": true,
	"fdisk":  true,
	"parted": true,
	"shred":  true,
}

// isDestructiveCommand returns true if the command with its arguments
// represents a destructive operation that should be denied.
// "rm" is only destructive with -rf/-fr flags; plain "rm file" is just "ask".
func isDestructiveCommand(cmd string, args []string) bool {
	if destructiveCommands[cmd] {
		return true
	}

	// rm with recursive force flags is destructive
	if cmd == "rm" {
		for _, arg := range args {
			if arg == "-rf" || arg == "-fr" || arg == "-Rf" || arg == "-fR" {
				return true
			}
			// Also catch combined flags like -rfv, -fvr etc
			if len(arg) > 1 && arg[0] == '-' && containsAll(arg[1:], 'r', 'f') {
				return true
			}
		}
		return false
	}

	return false
}

// containsAll checks if a string contains all the specified characters.
func containsAll(s string, chars ...byte) bool {
	for _, c := range chars {
		found := false
		for i := 0; i < len(s); i++ {
			if s[i] == c || s[i] == c-32 { // case-insensitive for R/r, F/f
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// riskyCommands are commands that require user permission because they
// access the network, modify packages, or control system services.
var riskyCommands = map[string]bool{
	// Network
	"curl": true, "wget": true, "ssh": true, "scp": true, "rsync": true,
	// Package managers
	"pip": true, "pip3": true, "npm": true, "npx": true, "yarn": true, "pnpm": true,
	"apt": true, "apt-get": true, "yum": true, "dnf": true, "brew": true,
	"go": true,
	// Containers
	"docker": true, "kubectl": true, "podman": true,
	// System
	"mount": true, "umount": true,
	"kill": true, "pkill": true, "killall": true,
	"reboot": true, "shutdown": true, "poweroff": true, "halt": true,
	"systemctl": true, "service": true,
	// File modification
	"rm": true, "mv": true, "cp": true,
	"mkdir": true, "rmdir": true,
	"chmod": true, "chown": true, "chgrp": true,
	"touch": true, "truncate": true,
	"tee": true,
	"sed": true, "awk": true,
	"nano": true, "vim": true, "vi": true, "emacs": true,
	// Misc risky
	"sleep": true,
}

// isRiskyCommand returns true if the command is risky and should require
// user permission (classified as Ask).
func isRiskyCommand(cmd string) bool {
	return riskyCommands[cmd]
}

// --- Security validator functions ---
// Each returns *SecurityCheckResult or nil for passthrough.

// validateEmpty checks for empty/whitespace-only commands.
func validateEmpty(command string) *SecurityCheckResult {
	if strings.TrimSpace(command) == "" {
		return &SecurityCheckResult{
			Behavior: "passthrough",
			Message:  "Empty command is safe",
		}
	}
	return nil
}

// validateIncompleteCommands detects command fragments that start with
// tabs, flags, or continuation operators.
func validateIncompleteCommands(command string) *SecurityCheckResult {
	trimmed := strings.TrimSpace(command)

	// Starts with tab (incomplete fragment)
	if regexp.MustCompile(`^\s*\t`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command appears to be an incomplete fragment (starts with tab)",
			IsMisparsing: true,
		}
	}

	// Starts with flags
	if strings.HasPrefix(trimmed, "-") {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command appears to be an incomplete fragment (starts with flags)",
			IsMisparsing: true,
		}
	}

	// Starts with continuation operator
	if regexp.MustCompile(`^\s*(&&|\|\||;|>>?|<)`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command appears to be a continuation line (starts with operator)",
			IsMisparsing: true,
		}
	}

	return nil
}

// validateSafeCommandSubstitution checks for safe heredoc-in-substitution patterns.
func validateSafeCommandSubstitution(command string) *SecurityCheckResult {
	if !regexp.MustCompile(`\$\(.*<<`).MatchString(command) {
		return nil
	}

	if IsSafeHeredoc(command) {
		return &SecurityCheckResult{
			Behavior: "passthrough",
			Message:  "Safe command substitution: cat with quoted/escaped heredoc delimiter",
		}
	}

	return nil
}

// validateGitCommit provides special handling for git commit -m with heredocs.
func validateGitCommit(command string) *SecurityCheckResult {
	baseCommand := strings.Fields(command)
	if len(baseCommand) == 0 || baseCommand[0] != "git" {
		return nil
	}
	if !regexp.MustCompile(`^git\s+commit\s+`).MatchString(command) {
		return nil
	}

	// Bail on backslashes (can cause regex to mis-identify quote boundaries)
	if strings.Contains(command, "\\") {
		return nil // passthrough to full validation
	}

	// Match git commit -m 'message' or git commit -m "message"
	// Go regex doesn't support backreferences, so try both quote types.
	var quote, messageContent, remainder string
	noMetaPrefix := `^git[ \t]+commit[ \t]+[^;&|` + "`" + `$<>()\n\r]*?-m[ \t]+`

	sqMatch := regexp.MustCompile(noMetaPrefix + `'([^']*)'(.*)$`).FindStringSubmatch(command)
	dqMatch := regexp.MustCompile(noMetaPrefix + `"([^"]*)"(.*)$`).FindStringSubmatch(command)

	if sqMatch != nil {
		quote = "'"
		messageContent = sqMatch[1]
		remainder = sqMatch[2]
	} else if dqMatch != nil {
		quote = "\""
		messageContent = dqMatch[1]
		remainder = dqMatch[2]
	}

	if quote != "" {

		// Double-quoted message with command substitution
		if quote == "\"" && messageContent != "" &&
			regexp.MustCompile(`\$\(|` + "`" + `|\$\{`).MatchString(messageContent) {
			return &SecurityCheckResult{
				Behavior: "ask",
				Message:  "Git commit message contains command substitution patterns",
			}
		}

		// Check remainder for shell operators
		if remainder != "" && regexp.MustCompile(`[;|&()` + "`" + `]|\$\(|\$\{`).MatchString(remainder) {
			return nil // passthrough
		}

		// Check for unquoted redirects in remainder
		if remainder != "" {
			unquoted := stripQuotesSimple(remainder)
			if strings.ContainsAny(unquoted, "<>") {
				return nil // passthrough
			}
		}

		// Block messages starting with dash
		if messageContent != "" && strings.HasPrefix(messageContent, "-") {
			return &SecurityCheckResult{
				Behavior: "ask",
				Message:  "Command contains quoted characters in flag names",
			}
		}

		return &SecurityCheckResult{
			Behavior: "passthrough",
			Message:  "Git commit with simple quoted message is allowed",
		}
	}

	return nil
}

// stripQuotesSimple removes quoted content without backslash handling.
// Used for git commit remainder checking only.
func stripQuotesSimple(s string) string {
	var result strings.Builder
	inSQ := false
	inDQ := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDQ {
			inSQ = !inSQ
			continue
		}
		if c == '"' && !inSQ {
			inDQ = !inDQ
			continue
		}
		if !inSQ && !inDQ {
			result.WriteByte(c)
		}
	}
	return result.String()
}

// validateJqCommand detects jq commands with system() function or dangerous flags.
func validateJqCommand(command string) *SecurityCheckResult {
	fields := strings.Fields(command)
	if len(fields) == 0 || fields[0] != "jq" {
		return nil
	}

	if regexp.MustCompile(`\bsystem\s*\(`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "jq command contains system() function which executes arbitrary commands",
		}
	}

	// Dangerous flags that could read files into jq variables
	afterJq := strings.TrimSpace(command[2:])
	if regexp.MustCompile(`(?:^|\s)(?:-f\b|--from-file|--rawfile|--slurpfile|-L\b|--library-path)`).MatchString(afterJq) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "jq command contains dangerous flags that could execute code or read arbitrary files",
		}
	}

	return nil
}

// validateShellMetacharacters detects shell metacharacters in arguments.
func validateShellMetacharacters(command string) *SecurityCheckResult {
	// Extract content with double quotes preserved (single quotes stripped)
	unquotedContent := extractWithDoubleQuotes(command)

	message := "Command contains shell metacharacters (;, |, or &) in arguments"

	// Quoted metacharacters in arguments
	if regexp.MustCompile(`(?:^|\s)["'][^"']*[;&][^"']*["'](?:\s|$)`).MatchString(unquotedContent) {
		return &SecurityCheckResult{Behavior: "ask", Message: message}
	}

	// Glob patterns with metacharacters (find -name, -path, -iname)
	globPatterns := []*regexp.Regexp{
		regexp.MustCompile(`-name\s+["'][^"']*[;|&][^"']*["']`),
		regexp.MustCompile(`-path\s+["'][^"']*[;|&][^"']*["']`),
		regexp.MustCompile(`-iname\s+["'][^"']*[;|&][^"']*["']`),
	}
	for _, p := range globPatterns {
		if p.MatchString(unquotedContent) {
			return &SecurityCheckResult{Behavior: "ask", Message: message}
		}
	}

	// Regex patterns with metacharacters
	if regexp.MustCompile(`-regex\s+["'][^"']*[;&][^"']*["']`).MatchString(unquotedContent) {
		return &SecurityCheckResult{Behavior: "ask", Message: message}
	}

	return nil
}

// extractWithDoubleQuotes strips single-quoted content, preserving double-quoted content.
// Matches the TS extractQuotedContent's withDoubleQuotes output.
func extractWithDoubleQuotes(command string) string {
	var result strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		if escaped {
			escaped = false
			if !inSingleQuote {
				result.WriteByte(c)
			}
			continue
		}

		if c == '\\' && !inSingleQuote {
			escaped = true
			if !inSingleQuote {
				result.WriteByte(c)
			}
			continue
		}

		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if !inSingleQuote {
			result.WriteByte(c)
		}
	}

	return result.String()
}

// validateDangerousVariables detects variables used with redirections or pipes.
func validateDangerousVariables(command string) *SecurityCheckResult {
	fullyUnquoted := extractFullyUnquoted(command)
	fullyUnquoted = stripSafeRedirections(fullyUnquoted)

	if regexp.MustCompile(`[<>|]\s*\$[A-Za-z_]`).MatchString(fullyUnquoted) ||
		regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*\s*[|<>]`).MatchString(fullyUnquoted) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains variables in dangerous contexts (redirections or pipes)",
		}
	}

	return nil
}

// validateDangerousPatterns detects command substitution patterns.
func validateDangerousPatterns(command string) *SecurityCheckResult {
	unquotedContent := extractWithDoubleQuotes(command)

	// Check for unescaped backticks
	if hasUnescapedChar(unquotedContent, '`') {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains backticks (`) for command substitution",
		}
	}

	// Command substitution patterns matching the TS COMMAND_SUBSTITUTION_PATTERNS
	substitutionPatterns := []struct {
		pattern *regexp.Regexp
		message string
	}{
		{regexp.MustCompile(`<\(`), "process substitution <()"},
		{regexp.MustCompile(`>\(`), "process substitution >()"},
		{regexp.MustCompile(`=\(`), "Zsh process substitution =()"},
		{regexp.MustCompile(`(?:^|[\s;&|])=[a-zA-Z_]`), "Zsh equals expansion (=cmd)"},
		{regexp.MustCompile(`\$\(`), "$() command substitution"},
		{regexp.MustCompile(`\$\{`), "${} parameter substitution"},
		{regexp.MustCompile(`\$\[`), "$[] legacy arithmetic expansion"},
		{regexp.MustCompile(`~\[`), "Zsh-style parameter expansion"},
		{regexp.MustCompile(`\(e:`), "Zsh-style glob qualifiers"},
		{regexp.MustCompile(`\(\+`), "Zsh glob qualifier with command execution"},
		{regexp.MustCompile(`\}\s*always\s*\{`), "Zsh always block (try/always construct)"},
		{regexp.MustCompile(`<#`), "PowerShell comment syntax"},
	}

	for _, sp := range substitutionPatterns {
		if sp.pattern.MatchString(unquotedContent) {
			return &SecurityCheckResult{
				Behavior: "ask",
				Message:  "Command contains " + sp.message,
			}
		}
	}

	return nil
}

// hasUnescapedChar checks if content contains an unescaped occurrence of a character.
func hasUnescapedChar(content string, char byte) bool {
	i := 0
	for i < len(content) {
		if content[i] == '\\' && i+1 < len(content) {
			i += 2
			continue
		}
		if content[i] == char {
			return true
		}
		i++
	}
	return false
}

// validateRedirections detects input/output redirections in unquoted content.
func validateRedirections(command string) *SecurityCheckResult {
	fullyUnquoted := extractFullyUnquoted(command)
	fullyUnquoted = stripSafeRedirections(fullyUnquoted)

	if strings.Contains(fullyUnquoted, "<") {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains input redirection (<) which could read sensitive files",
		}
	}

	if strings.Contains(fullyUnquoted, ">") {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains output redirection (>) which could write to arbitrary files",
		}
	}

	return nil
}

// validateNewlines detects newlines in unquoted content that could hide commands.
func validateNewlines(command string) *SecurityCheckResult {
	fullyUnquoted := extractFullyUnquoted(command)

	if !strings.ContainsAny(fullyUnquoted, "\n\r") {
		return nil
	}

	// Flag newline/CR followed by non-whitespace, except backslash-newline continuations
	// at word boundaries.
	lines := strings.Split(fullyUnquoted, "\n")
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Check previous line doesn't end with space+backslash (continuation)
		prevLine := lines[i-1]
		if len(prevLine) > 0 {
			// Safe backslash-newline continuation at word boundary
			if len(prevLine) >= 2 &&
				(prevLine[len(prevLine)-2] == ' ' || prevLine[len(prevLine)-2] == '\t') &&
				prevLine[len(prevLine)-1] == '\\' {
				continue
			}
		}
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains newlines that could separate multiple commands",
		}
	}

	// Also check for \r
	if strings.Contains(fullyUnquoted, "\r") {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains newlines that could separate multiple commands",
		}
	}

	return nil
}

// validateMalformedTokenInjection detects malformed tokens with command separators.
// In Go we use the mvdan.cc/sh parser instead of shell-quote, so this is a
// simplified check for unbalanced delimiters combined with separators.
func validateMalformedTokenInjection(command string) *SecurityCheckResult {
	// Check for command separators
	if !strings.ContainsAny(command, ";") &&
		!strings.Contains(command, "&&") &&
		!strings.Contains(command, "||") {
		return nil
	}

	// Count unbalanced delimiters
	var openBraces, closeBraces, openParens, closeParens int
	inSQ := false
	inDQ := false
	escaped := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && !inSQ {
			escaped = true
			continue
		}
		if c == '\'' && !inDQ {
			inSQ = !inSQ
			continue
		}
		if c == '"' && !inSQ {
			inDQ = !inDQ
			continue
		}
		if !inSQ && !inDQ {
			switch c {
			case '{':
				openBraces++
			case '}':
				closeBraces++
			case '(':
				openParens++
			case ')':
				closeParens++
			}
		}
	}

	// Unbalanced delimiters with separators is suspicious
	if openBraces != closeBraces || openParens != closeParens {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command contains ambiguous syntax with command separators that could be misinterpreted",
			IsMisparsing: true,
		}
	}

	return nil
}

// validateBackslashEscapedWhitespace detects backslash-escaped spaces/tabs outside quotes.
func validateBackslashEscapedWhitespace(command string) *SecurityCheckResult {
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		if c == '\\' && !inSingleQuote {
			if !inDoubleQuote {
				if i+1 < len(command) {
					next := command[i+1]
					if next == ' ' || next == '\t' {
						return &SecurityCheckResult{
							Behavior:     "ask",
							Message:      "Command contains backslash-escaped whitespace that could alter command parsing",
							IsMisparsing: true,
						}
					}
				}
			}
			i++
			continue
		}

		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
	}

	return nil
}

// shellOperators is the set of operators that trigger the backslash-escaped-operator check.
var shellOperators = map[byte]bool{
	';': true, '|': true, '&': true, '<': true, '>': true,
}

// validateBackslashEscapedOperators detects backslash before shell operators outside quotes.
func validateBackslashEscapedOperators(command string) *SecurityCheckResult {
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		// Handle backslash before quote toggles
		if c == '\\' && !inSingleQuote {
			if !inDoubleQuote {
				if i+1 < len(command) && shellOperators[command[i+1]] {
					return &SecurityCheckResult{
						Behavior:     "ask",
						Message:      "Command contains a backslash before a shell operator which can hide command structure",
						IsMisparsing: true,
					}
				}
			}
			i++
			continue
		}

		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
	}

	return nil
}

// validateMidWordHash detects # in mid-word position (comment injection).
func validateMidWordHash(command string) *SecurityCheckResult {
	unquotedKeep := extractUnquotedKeepChars(command)

	// Match # preceded by non-whitespace, excluding ${#
	if regexp.MustCompile(`\S#`).MatchString(unquotedKeep) {
		// Exclude ${# which is bash string-length syntax
		if !regexp.MustCompile(`\$\{#`).MatchString(unquotedKeep) {
			return &SecurityCheckResult{
				Behavior:     "ask",
				Message:      "Command contains mid-word # which is parsed differently by different shell parsers",
				IsMisparsing: true,
			}
		}
		// Check if there's a non-${# occurrence of \S#
		cleaned := regexp.MustCompile(`\$\{#`).ReplaceAllString(unquotedKeep, "   ")
		if regexp.MustCompile(`\S#`).MatchString(cleaned) {
			return &SecurityCheckResult{
				Behavior:     "ask",
				Message:      "Command contains mid-word # which is parsed differently by different shell parsers",
				IsMisparsing: true,
			}
		}
	}

	return nil
}

// validateCommentQuoteDesync detects quote characters inside # comments
// that could desync quote tracking in downstream validators.
func validateCommentQuoteDesync(command string) *SecurityCheckResult {
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		if escaped {
			escaped = false
			continue
		}

		if inSingleQuote {
			if c == '\'' {
				inSingleQuote = false
			}
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if inDoubleQuote {
			if c == '"' {
				inDoubleQuote = false
			}
			continue
		}

		if c == '\'' {
			inSingleQuote = true
			continue
		}
		if c == '"' {
			inDoubleQuote = true
			continue
		}

		// Unquoted # starts a comment
		if c == '#' {
			lineEnd := strings.Index(command[i+1:], "\n")
			var commentText string
			if lineEnd == -1 {
				commentText = command[i+1:]
			} else {
				commentText = command[i+1 : i+1+lineEnd]
			}
			if strings.ContainsAny(commentText, "'\"") {
				return &SecurityCheckResult{
					Behavior:     "ask",
					Message:      "Command contains quote characters inside a # comment which can desync quote tracking",
					IsMisparsing: true,
				}
			}
			if lineEnd == -1 {
				break
			}
			i = i + 1 + lineEnd
		}
	}

	return nil
}

// validateQuotedNewline detects newlines inside quoted strings where the next line
// starts with # (which stripCommentLines would incorrectly remove).
func validateQuotedNewline(command string) *SecurityCheckResult {
	if !strings.Contains(command, "\n") || !strings.Contains(command, "#") {
		return nil
	}

	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && !inSingleQuote {
			escaped = true
			continue
		}

		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if c == '\n' && (inSingleQuote || inDoubleQuote) {
			lineStart := i + 1
			nextNL := strings.Index(command[lineStart:], "\n")
			var nextLine string
			if nextNL == -1 {
				nextLine = command[lineStart:]
			} else {
				nextLine = command[lineStart : lineStart+nextNL]
			}
			if strings.HasPrefix(strings.TrimSpace(nextLine), "#") {
				return &SecurityCheckResult{
					Behavior:     "ask",
					Message:      "Command contains a quoted newline followed by a #-prefixed line, which can hide arguments from line-based permission checks",
					IsMisparsing: true,
				}
			}
		}
	}

	return nil
}

// zshDangerousCommands lists Zsh-specific commands that can bypass security checks.
var zshDangerousCommands = map[string]bool{
	"zmodload": true, "emulate": true,
	"sysopen": true, "sysread": true, "syswrite": true, "sysseek": true,
	"zpty": true, "ztcp": true, "zsocket": true, "mapfile": true,
	"zf_rm": true, "zf_mv": true, "zf_ln": true, "zf_chmod": true,
	"zf_chown": true, "zf_mkdir": true, "zf_rmdir": true, "zf_chgrp": true,
}

// zshPrecommandModifiers are Zsh modifiers that don't change the actual command.
var zshPrecommandModifiers = map[string]bool{
	"command": true, "builtin": true, "noglob": true, "nocorrect": true,
}

// validateZshDangerousCommands detects Zsh-specific commands that bypass security.
func validateZshDangerousCommands(command string) *SecurityCheckResult {
	trimmed := strings.TrimSpace(command)
	tokens := strings.Fields(trimmed)
	baseCmd := ""
	for _, token := range tokens {
		// Skip env var assignments
		if regexp.MustCompile(`^[A-Za-z_]\w*=`).MatchString(token) {
			continue
		}
		// Skip Zsh precommand modifiers
		if zshPrecommandModifiers[token] {
			continue
		}
		baseCmd = token
		break
	}

	if zshDangerousCommands[baseCmd] {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command uses Zsh-specific '" + baseCmd + "' which can bypass security checks",
		}
	}

	// Check for fc -e (eval via editor)
	if baseCmd == "fc" && regexp.MustCompile(`\s-\S*e`).MatchString(trimmed) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command uses 'fc -e' which can execute arbitrary commands via editor",
		}
	}

	return nil
}

// --- Main security validation chain ---

// ValidateBashSecurity runs all 23+ security validators against a command.
// Returns nil if the command passes all checks (safe from security perspective).
// Returns a SecurityCheckResult with Behavior="ask" or "deny" if a check fails.
// Short-circuits on the first failure, but defers non-misparsing results to
// ensure misparsing validators later in the chain are not missed.
func ValidateBashSecurity(command string) *SecurityCheckResult {
	// Pre-check: Control characters (before any other processing)
	if result := ValidateControlCharacters(command); result != nil {
		return result
	}

	// --- Group 1: Early validators ---
	// These can short-circuit with "passthrough" (allow) or "ask" results.
	earlyValidators := []func(string) *SecurityCheckResult{
		validateEmpty,
		validateIncompleteCommands,
		validateSafeCommandSubstitution,
		validateGitCommit,
	}

	for _, validator := range earlyValidators {
		result := validator(command)
		if result == nil {
			continue
		}
		if result.Behavior == "passthrough" {
			return nil // Allowed, skip remaining validators
		}
		if result.Behavior == "ask" || result.Behavior == "deny" {
			result.IsMisparsing = true
			return result
		}
	}

	// --- Group 2: Misparsing validators ---
	// These detect parser differential attacks. Their "ask" results get
	// IsMisparsing=true, causing early blocking in the permission flow.
	misparsingValidators := []func(string) *SecurityCheckResult{
		ValidateCarriageReturn,
		ValidateIFSInjection,
		ValidateProcEnvironAccess,
		validateMalformedTokenInjection,
		validateBackslashEscapedWhitespace,
		validateBackslashEscapedOperators,
		validateQuotedNewline,
	}

	for _, validator := range misparsingValidators {
		result := validator(command)
		if result != nil {
			result.IsMisparsing = true
			return result
		}
	}

	// --- Group 3: Standard validators ---
	// Non-misparsing validators are deferred; if a misparsing one fires later,
	// it takes priority.
	type validatorEntry struct {
		fn            func(string) *SecurityCheckResult
		isMisparsing  bool
		isNonMisparsing bool
	}

	standardValidators := []validatorEntry{
		{fn: validateJqCommand},
		{fn: ValidateObfuscatedFlags},
		{fn: validateShellMetacharacters},
		{fn: validateDangerousVariables},
		{fn: validateCommentQuoteDesync},
		{fn: validateQuotedNewline},
		{fn: ValidateHeredoc},
		{fn: validateDangerousPatterns},
		{fn: validateNewlines, isNonMisparsing: true},
		{fn: validateRedirections, isNonMisparsing: true},
		{fn: ValidateUnicodeWhitespace},
		{fn: validateMidWordHash},
		{fn: ValidateBraceExpansion},
		{fn: validateZshDangerousCommands},
	}

	var deferredNonMisparsing *SecurityCheckResult
	for _, v := range standardValidators {
		result := v.fn(command)
		if result != nil && (result.Behavior == "ask" || result.Behavior == "deny") {
			if v.isNonMisparsing {
				if deferredNonMisparsing == nil {
					deferredNonMisparsing = result
				}
				continue
			}
			return result
		}
	}

	if deferredNonMisparsing != nil {
		return deferredNonMisparsing
	}

	return nil // All checks passed
}

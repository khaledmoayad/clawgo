package classify

import (
	"regexp"
	"strings"
	"unicode"
)

// unicodeWhitespaceRE matches Unicode whitespace characters that bash treats
// as literal word content but other parsers may treat as word separators.
// Extended beyond the TS original to also include zero-width space (U+200B)
// through zero-width joiner (U+200D) as defense-in-depth.
var unicodeWhitespaceRE = regexp.MustCompile(
	"[\u00A0\u1680\u2000-\u200D\u2028\u2029\u202F\u205F\u3000\uFEFF]",
)

// ValidateUnicodeWhitespace detects non-ASCII whitespace characters that can
// be used to obfuscate commands (zero-width space, non-breaking space, etc.).
func ValidateUnicodeWhitespace(command string) *SecurityCheckResult {
	if unicodeWhitespaceRE.MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains Unicode whitespace characters that could cause parsing inconsistencies",
		}
	}
	return nil
}

// ValidateObfuscatedFlags detects various flag obfuscation techniques:
// - ANSI-C quoting ($'...')
// - Locale quoting ($"...")
// - Empty quote pairs before dashes (""-rf, ''-rf)
// - Quoted characters in flag names ("-exec", '-f')
// - Consecutive quote characters at word start
func ValidateObfuscatedFlags(command string) *SecurityCheckResult {
	// Extract base command for echo exception
	baseCommand := strings.Fields(command)
	isEcho := len(baseCommand) > 0 && baseCommand[0] == "echo"
	hasShellOperators := strings.ContainsAny(command, "|&;")

	// Echo without shell operators is safe for obfuscated flags
	if isEcho && !hasShellOperators {
		return nil
	}

	// 1. Block ANSI-C quoting ($'...')
	if regexp.MustCompile(`\$'[^']*'`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains ANSI-C quoting which can hide characters",
		}
	}

	// 2. Block locale quoting ($"...")
	if regexp.MustCompile(`\$"[^"]*"`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains locale quoting which can hide characters",
		}
	}

	// 3. Block empty ANSI-C or locale quotes followed by dash
	if regexp.MustCompile(`\$['"]{2}\s*-`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains empty special quotes before dash (potential bypass)",
		}
	}

	// 4. Block sequences of empty quotes followed by dash
	if regexp.MustCompile(`(?:^|\s)(?:''|"")+\s*-`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains empty quotes before dash (potential bypass)",
		}
	}

	// 4b. Block homogeneous empty quote pairs adjacent to quoted dash
	if regexp.MustCompile(`(?:""|'')+['"]-`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains empty quote pair adjacent to quoted dash (potential flag obfuscation)",
		}
	}

	// 4c. Block 3+ consecutive quotes at word start
	if regexp.MustCompile(`(?:^|\s)['"]{3,}`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains consecutive quote characters at word start (potential obfuscation)",
		}
	}

	// Track quote state to detect quoted flags
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command)-1; i++ {
		c := command[i]
		next := command[i+1]

		if escaped {
			escaped = false
			continue
		}

		// Backslash escaping only outside single quotes
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

		if inSingleQuote || inDoubleQuote {
			continue
		}

		// Look for whitespace followed by quote containing a dash
		if isASCIIWhitespace(c) && (next == '\'' || next == '"' || next == '`') {
			quoteChar := next
			j := i + 2
			var insideQuote strings.Builder
			for j < len(command) && command[j] != quoteChar {
				insideQuote.WriteByte(command[j])
				j++
			}

			if j < len(command) && command[j] == quoteChar {
				content := insideQuote.String()
				// Flag chars inside: "-exec", "--flag"
				if regexp.MustCompile(`^-+[a-zA-Z0-9$` + "`" + `]`).MatchString(content) {
					return &SecurityCheckResult{
						Behavior: "ask",
						Message:  "Command contains quoted characters in flag names",
					}
				}
				// Flag chars continuing after quote: "-"exec
				if regexp.MustCompile(`^-+$`).MatchString(content) {
					if j+1 < len(command) && isFlagContinuationChar(command[j+1]) {
						return &SecurityCheckResult{
							Behavior: "ask",
							Message:  "Command contains quoted characters in flag names",
						}
					}
				}
			}
		}

		// Look for whitespace followed by dash - quoted flag names
		if isASCIIWhitespace(c) && next == '-' {
			j := i + 1
			var flagContent strings.Builder
			for j < len(command) {
				fc := command[j]
				if isASCIIWhitespace(fc) || fc == '=' {
					break
				}
				if fc == '\'' || fc == '"' || fc == '`' {
					// Special case for cut -d
					if len(baseCommand) > 0 && baseCommand[0] == "cut" && flagContent.String() == "-d" {
						break
					}
					if j+1 < len(command) {
						nextFC := command[j+1]
						if !isAlphaNumOrQuoteDash(nextFC) {
							break
						}
					}
				}
				flagContent.WriteByte(fc)
				j++
			}

			content := flagContent.String()
			if strings.Contains(content, "\"") || strings.Contains(content, "'") {
				return &SecurityCheckResult{
					Behavior: "ask",
					Message:  "Command contains quoted characters in flag names",
				}
			}
		}
	}

	// Check fullyUnquoted for quote-dash patterns
	fullyUnquoted := extractFullyUnquoted(command)
	if regexp.MustCompile(`\s['"\x60]-`).MatchString(fullyUnquoted) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains quoted characters in flag names",
		}
	}
	if regexp.MustCompile(`['"\x60]{2}-`).MatchString(fullyUnquoted) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command contains quoted characters in flag names",
		}
	}

	return nil
}

// ValidateBraceExpansion detects suspicious brace expansion patterns.
// Bash expands {a,b,c} and {1..5} but shell parsers may treat them as literal.
func ValidateBraceExpansion(command string) *SecurityCheckResult {
	// Extract fully unquoted content
	content := extractFullyUnquoted(command)

	// Count unescaped braces for mismatch detection
	var unescapedOpen, unescapedClose int
	for i := 0; i < len(content); i++ {
		if content[i] == '{' && !isEscapedAt(content, i) {
			unescapedOpen++
		} else if content[i] == '}' && !isEscapedAt(content, i) {
			unescapedClose++
		}
	}

	// Excess closing braces indicate quote-stripped brace obfuscation
	if unescapedOpen > 0 && unescapedClose > unescapedOpen {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Command has excess closing braces after quote stripping, indicating possible brace expansion obfuscation",
		}
	}

	// Check for quoted brace inside unquoted brace context
	if unescapedOpen > 0 {
		if regexp.MustCompile(`['"][{}]['"]`).MatchString(command) {
			return &SecurityCheckResult{
				Behavior: "ask",
				Message:  "Command contains quoted brace character inside brace context (potential brace expansion obfuscation)",
			}
		}
	}

	// Scan for brace expansion patterns: {a,b} or {1..5}
	for i := 0; i < len(content); i++ {
		if content[i] != '{' || isEscapedAt(content, i) {
			continue
		}

		// Find matching closing brace with depth tracking
		depth := 1
		matchingClose := -1
		for j := i + 1; j < len(content); j++ {
			if content[j] == '{' && !isEscapedAt(content, j) {
				depth++
			} else if content[j] == '}' && !isEscapedAt(content, j) {
				depth--
				if depth == 0 {
					matchingClose = j
					break
				}
			}
		}

		if matchingClose == -1 {
			continue
		}

		// Check for comma or .. at outermost level between braces
		innerDepth := 0
		for k := i + 1; k < matchingClose; k++ {
			if content[k] == '{' && !isEscapedAt(content, k) {
				innerDepth++
			} else if content[k] == '}' && !isEscapedAt(content, k) {
				innerDepth--
			} else if innerDepth == 0 {
				if content[k] == ',' {
					return &SecurityCheckResult{
						Behavior: "ask",
						Message:  "Command contains brace expansion that could alter command parsing",
					}
				}
				if content[k] == '.' && k+1 < matchingClose && content[k+1] == '.' {
					return &SecurityCheckResult{
						Behavior: "ask",
						Message:  "Command contains brace expansion that could alter command parsing",
					}
				}
			}
		}
	}

	return nil
}

// ValidateCarriageReturn detects carriage return characters (\r) in commands.
// CR can cause parser differentials between shell-quote and bash.
func ValidateCarriageReturn(command string) *SecurityCheckResult {
	if !strings.Contains(command, "\r") {
		return nil
	}

	// Check if CR appears outside double quotes
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
		if c == '\r' && !inDoubleQuote {
			return &SecurityCheckResult{
				Behavior:     "ask",
				Message:      "Command contains carriage return which shell-quote and bash tokenize differently",
				IsMisparsing: true,
			}
		}
	}

	return nil
}

// ValidateIFSInjection detects IFS variable usage that could bypass
// security validation by changing the field separator.
func ValidateIFSInjection(command string) *SecurityCheckResult {
	if regexp.MustCompile(`\$IFS|\$\{[^}]*IFS`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command contains IFS variable usage which could bypass security validation",
			IsMisparsing: true,
		}
	}
	return nil
}

// ValidateProcEnvironAccess detects access to /proc/*/environ which could
// expose sensitive environment variables like API keys.
func ValidateProcEnvironAccess(command string) *SecurityCheckResult {
	if regexp.MustCompile(`/proc/.*/environ`).MatchString(command) {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command accesses /proc/*/environ which could expose sensitive environment variables",
			IsMisparsing: true,
		}
	}
	return nil
}

// controlCharRE matches non-printable control characters that have no
// legitimate use in shell commands.
var controlCharRE = regexp.MustCompile("[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]")

// ValidateControlCharacters detects non-printable control characters
// that bash silently drops but can be used to slip metacharacters past checks.
func ValidateControlCharacters(command string) *SecurityCheckResult {
	if controlCharRE.MatchString(command) {
		return &SecurityCheckResult{
			Behavior:     "ask",
			Message:      "Command contains non-printable control characters that could be used to bypass security checks",
			IsMisparsing: true,
		}
	}
	return nil
}

// isASCIIWhitespace returns true for space and tab characters.
func isASCIIWhitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

// isFlagContinuationChar returns true for characters that can continue a flag.
func isFlagContinuationChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '\\' || c == '$' || c == '{' || c == '`' || c == '-'
}

// isAlphaNumOrQuoteDash returns true for alphanumeric, quote, or dash characters.
func isAlphaNumOrQuoteDash(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '\'' || c == '"' || c == '-'
}

// isEscapedAt checks if the character at position pos is escaped by
// counting consecutive backslashes before it.
func isEscapedAt(content string, pos int) bool {
	backslashCount := 0
	i := pos - 1
	for i >= 0 && content[i] == '\\' {
		backslashCount++
		i--
	}
	return backslashCount%2 == 1
}

// extractFullyUnquoted strips all single-quoted and double-quoted content
// from a command string, returning only the unquoted portions.
func extractFullyUnquoted(command string) string {
	var result strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		if escaped {
			escaped = false
			if !inSingleQuote && !inDoubleQuote {
				result.WriteByte(c)
			}
			continue
		}

		if c == '\\' && !inSingleQuote {
			escaped = true
			if !inSingleQuote && !inDoubleQuote {
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

		if !inSingleQuote && !inDoubleQuote {
			result.WriteByte(c)
		}
	}

	return result.String()
}

// extractUnquotedKeepChars is like extractFullyUnquoted but preserves
// quote characters ('/"") while stripping quoted content.
func extractUnquotedKeepChars(command string) string {
	var result strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		if escaped {
			escaped = false
			if !inSingleQuote && !inDoubleQuote {
				result.WriteByte(c)
			}
			continue
		}

		if c == '\\' && !inSingleQuote {
			escaped = true
			if !inSingleQuote && !inDoubleQuote {
				result.WriteByte(c)
			}
			continue
		}

		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			result.WriteByte(c) // Keep the quote char
			continue
		}

		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			result.WriteByte(c) // Keep the quote char
			continue
		}

		if !inSingleQuote && !inDoubleQuote {
			result.WriteByte(c)
		}
	}

	return result.String()
}

// stripSafeRedirections removes safe redirections like >/dev/null, 2>&1
// from content to avoid false positives in redirection checks.
func stripSafeRedirections(content string) string {
	// Must have trailing boundary to prevent partial matches
	r := regexp.MustCompile(`\s+2\s*>&\s*1(?:\s|$)`)
	content = r.ReplaceAllString(content, " ")
	r = regexp.MustCompile(`[012]?\s*>\s*/dev/null(?:\s|$)`)
	content = r.ReplaceAllString(content, " ")
	r = regexp.MustCompile(`\s*<\s*/dev/null(?:\s|$)`)
	content = r.ReplaceAllString(content, " ")
	return content
}

// hasUnicodeWhitespace checks if the string contains any Unicode whitespace
// characters beyond basic ASCII space/tab/newline.
func hasUnicodeWhitespace(s string) bool {
	for _, r := range s {
		if r > 127 && unicode.IsSpace(r) {
			return true
		}
		// Also check specific zero-width and special space characters
		switch r {
		case '\u00A0', '\u1680', '\u202F', '\u205F', '\u3000', '\uFEFF':
			return true
		case '\u200B': // zero-width space
			return true
		}
		if r >= '\u2000' && r <= '\u200A' {
			return true
		}
		if r == '\u2028' || r == '\u2029' {
			return true
		}
	}
	return false
}

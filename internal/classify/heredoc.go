package classify

import (
	"regexp"
	"strings"
)

// heredocStartPattern matches heredoc syntax: <<WORD, <<'WORD', <<"WORD", <<-WORD
// Go regex doesn't support backreferences or lookbehind, so we match each quote variant separately
// and manually check for <<< (herestring) in ContainsHeredoc.
// Group 1: dash prefix (-); Group 2: single-quoted delimiter; Group 3: double-quoted delimiter; Group 4: unquoted delimiter
var heredocStartPattern = regexp.MustCompile(`<<(-)?[ \t]*(?:'(\\?\w+)'|"(\\?\w+)"|\\?(\w+))`)

// simpleHeredocCheck is a quick test for heredoc presence.
var simpleHeredocCheck = regexp.MustCompile(`<<`)

// ContainsHeredoc returns true if the command appears to contain heredoc syntax.
// This is a quick check that does not validate the heredoc is well-formed.
// It excludes herestrings (<<<) by checking for triple less-than.
func ContainsHeredoc(command string) bool {
	if !strings.Contains(command, "<<") {
		return false
	}
	// Find all << positions and check they're not part of <<<
	matches := heredocStartPattern.FindAllStringIndex(command, -1)
	for _, m := range matches {
		start := m[0]
		// Check that this << is not preceded by < (making it <<<)
		if start > 0 && command[start-1] == '<' {
			continue
		}
		// Check that this << is not followed by < (making it <<<)
		if start+2 < len(command) && command[start+2] == '<' {
			continue
		}
		return true
	}
	return false
}

// heredocInSubstitution matches $( followed by << later.
var heredocInSubstitution = regexp.MustCompile(`\$\(.*<<`)

// safeHeredocPattern matches $(cat <<(-)'DELIM' or $(cat <<(-)\DELIM
var safeHeredocPattern = regexp.MustCompile(`\$\(cat[ \t]*<<(-?)[ \t]*(?:'+([A-Za-z_]\w*)'+|\\([A-Za-z_]\w*))`)

// IsSafeHeredoc returns true if the command contains only safe heredoc patterns.
// A heredoc is safe if it uses $(cat <<'DELIM'...) where the delimiter is
// single-quoted or escaped (suppresses expansion in body).
func IsSafeHeredoc(command string) bool {
	if !heredocInSubstitution.MatchString(command) {
		return false
	}

	matches := safeHeredocPattern.FindAllStringSubmatchIndex(command, -1)
	if len(matches) == 0 {
		return false
	}

	type verifiedRange struct {
		start, end int
	}
	var verified []verifiedRange

	for _, m := range matches {
		start := m[0]
		operatorEnd := m[1]

		// Extract delimiter from whichever group matched
		var delimiter string
		if m[4] >= 0 && m[5] >= 0 {
			delimiter = command[m[4]:m[5]]
		} else if m[6] >= 0 && m[7] >= 0 {
			delimiter = command[m[6]:m[7]]
		}
		if delimiter == "" {
			continue
		}

		isDash := m[2] >= 0 && m[3] >= 0 && command[m[2]:m[3]] == "-"

		// Opening line must end immediately after delimiter
		afterOperator := command[operatorEnd:]
		openLineEnd := strings.Index(afterOperator, "\n")
		if openLineEnd == -1 {
			return false
		}
		openLineTail := afterOperator[:openLineEnd]
		if strings.TrimRight(openLineTail, " \t") != "" {
			return false
		}

		// Body starts after newline
		bodyStart := operatorEnd + openLineEnd + 1
		body := command[bodyStart:]
		bodyLines := strings.Split(body, "\n")

		closingLineIdx := -1
		closeParenOffset := -1

		for i, rawLine := range bodyLines {
			line := rawLine
			if isDash {
				line = strings.TrimLeft(line, "\t")
			}

			// Form 1: delimiter alone on line
			if line == delimiter {
				closingLineIdx = i
				if i+1 < len(bodyLines) {
					nextLine := bodyLines[i+1]
					trimmed := strings.TrimLeft(nextLine, " \t")
					if strings.HasPrefix(trimmed, ")") {
						// Calculate end position
						pos := bodyStart
						for j := 0; j <= i+1; j++ {
							if j > 0 {
								pos++ // newline
							}
							pos += len(bodyLines[j])
						}
						// Back up to the position of )
						closeParenOffset = pos - len(bodyLines[i+1]) + (len(nextLine) - len(trimmed))
					}
				}
				break
			}

			// Form 2: delimiter immediately followed by )
			if strings.HasPrefix(line, delimiter) {
				afterDelim := line[len(delimiter):]
				trimmedAfter := strings.TrimLeft(afterDelim, " \t")
				if strings.HasPrefix(trimmedAfter, ")") {
					closingLineIdx = i
					// Calculate end position
					pos := bodyStart
					for j := 0; j < i; j++ {
						pos += len(bodyLines[j]) + 1
					}
					pos += len(rawLine) - len(afterDelim) + (len(afterDelim) - len(trimmedAfter))
					closeParenOffset = pos
					break
				}
				// Metacharacter after delimiter
				if len(afterDelim) > 0 {
					first := afterDelim[0]
					if first == ')' || first == '}' || first == '`' || first == '|' || first == '&' || first == ';' || first == '(' || first == '<' || first == '>' {
						return false
					}
				}
			}
		}

		if closingLineIdx == -1 || closeParenOffset == -1 {
			return false
		}

		verified = append(verified, verifiedRange{start: start, end: closeParenOffset + 1})
	}

	if len(verified) == 0 {
		return false
	}

	// Check for nested matches
	for i, outer := range verified {
		for j, inner := range verified {
			if i == j {
				continue
			}
			if inner.start > outer.start && inner.start < outer.end {
				return false
			}
		}
	}

	// Strip verified heredocs and check remaining
	remaining := command
	// Process in reverse to preserve indices
	for i := len(verified) - 1; i >= 0; i-- {
		v := verified[i]
		remaining = remaining[:v.start] + remaining[v.end:]
	}

	// Check prefix: $() must not be in command-name position
	trimmedRemaining := strings.TrimSpace(remaining)
	if len(trimmedRemaining) > 0 {
		firstStart := verified[0].start
		for _, v := range verified {
			if v.start < firstStart {
				firstStart = v.start
			}
		}
		prefix := command[:firstStart]
		if strings.TrimSpace(prefix) == "" {
			return false
		}
	}

	// Remaining must contain only safe characters
	safeChars := regexp.MustCompile(`^[a-zA-Z0-9 \t"'.\-/_@=,:+~]*$`)
	if !safeChars.MatchString(remaining) {
		return false
	}

	return true
}

// HasSafeHeredocSubstitution returns true if the command contains a safe
// heredoc substitution pattern like $(cat <<'EOF'...).
func HasSafeHeredocSubstitution(command string) bool {
	return IsSafeHeredoc(command)
}

// ValidateHeredoc checks if the command contains heredoc syntax and validates
// its safety. Returns nil if no heredoc or heredoc is safe. Returns an Ask
// result if the heredoc contains potentially dangerous patterns.
func ValidateHeredoc(command string) *SecurityCheckResult {
	if !ContainsHeredoc(command) {
		return nil // No heredoc, passthrough
	}

	// Check for heredoc in command substitution -- if safe, passthrough
	if heredocInSubstitution.MatchString(command) {
		if IsSafeHeredoc(command) {
			return nil // Safe heredoc substitution
		}
	}

	// Check heredoc bodies for command substitution / variable expansion
	// in unquoted heredocs (<<WORD without quotes around delimiter)
	if hasUnsafeHeredocExpansion(command) {
		return &SecurityCheckResult{
			Behavior: "ask",
			Message:  "Heredoc contains command substitution or variable expansion",
		}
	}

	return nil
}

// hasUnsafeHeredocExpansion checks if any unquoted heredoc body contains
// command substitution ($(), ``) or variable expansion (${}, $VAR).
func hasUnsafeHeredocExpansion(command string) bool {
	if !strings.Contains(command, "<<") {
		return false
	}

	// Find heredoc start positions
	matches := heredocStartPattern.FindAllStringSubmatchIndex(command, -1)
	for _, m := range matches {
		operatorEnd := m[1]

		// Check if delimiter is quoted
		isQuoted := m[4] >= 0 && m[5] >= 0 // group 2 matched (quote char)

		// Find delimiter
		var delimiter string
		if m[6] >= 0 && m[7] >= 0 {
			delimiter = command[m[6]:m[7]]
		} else if m[8] >= 0 && m[9] >= 0 {
			delimiter = command[m[8]:m[9]]
		}
		if delimiter == "" {
			continue
		}

		isDash := m[2] >= 0 && m[3] >= 0 && m[3]-m[2] == 1 && command[m[2]:m[3]] == "-"

		// Quoted delimiters suppress expansion -- always safe
		if isQuoted {
			continue
		}

		// Find heredoc body
		afterOp := command[operatorEnd:]
		nlIdx := strings.Index(afterOp, "\n")
		if nlIdx == -1 {
			continue
		}

		bodyStart := operatorEnd + nlIdx + 1
		if bodyStart >= len(command) {
			continue
		}

		bodyText := command[bodyStart:]
		bodyLines := strings.Split(bodyText, "\n")

		// Find closing delimiter and extract body content
		var bodyContent strings.Builder
		for _, line := range bodyLines {
			checkLine := line
			if isDash {
				checkLine = strings.TrimLeft(checkLine, "\t")
			}
			if checkLine == delimiter {
				break
			}
			bodyContent.WriteString(line)
			bodyContent.WriteByte('\n')
		}

		body := bodyContent.String()

		// Check for dangerous expansion patterns
		if strings.Contains(body, "$(") ||
			strings.Contains(body, "`") ||
			strings.Contains(body, "${") ||
			regexp.MustCompile(`\$[A-Za-z_]`).MatchString(body) {
			return true
		}
	}

	return false
}

// ExtractHeredocs extracts heredoc info from a command string.
// Returns the list of heredoc delimiters found. Used for downstream analysis.
func ExtractHeredocs(command string) []string {
	if !strings.Contains(command, "<<") {
		return nil
	}

	matches := heredocStartPattern.FindAllStringSubmatch(command, -1)
	var delimiters []string
	for _, m := range matches {
		if len(m) > 4 && m[4] != "" {
			delimiters = append(delimiters, m[4])
		} else if len(m) > 3 && m[3] != "" {
			delimiters = append(delimiters, m[3])
		}
	}
	return delimiters
}

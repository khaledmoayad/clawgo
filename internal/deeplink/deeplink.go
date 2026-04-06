// Package deeplink parses and builds claude-cli:// deep link URIs.
// Deep links allow external tools to launch ClawGo with pre-filled prompts,
// working directories, and repository context.
package deeplink

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Protocol is the URI scheme for ClawGo deep links.
const Protocol = "claude-cli"

// MaxQueryLength is the maximum allowed length for the query parameter.
const MaxQueryLength = 5000

// MaxCWDLength is the maximum allowed length for the cwd parameter.
const MaxCWDLength = 4096

// repoSlugRe validates GitHub owner/repo slugs (owner/repo, no extra slashes).
var repoSlugRe = regexp.MustCompile(`^[\w.\-]+/[\w.\-]+$`)

// hiddenUnicodeReplacer strips invisible Unicode characters that could be used
// for injection attacks: zero-width spaces, direction overrides, BOM, tag chars.
var hiddenUnicodeReplacer = strings.NewReplacer(
	"\u200B", "", // zero-width space
	"\u200C", "", // zero-width non-joiner
	"\u200D", "", // zero-width joiner
	"\u200E", "", // left-to-right mark
	"\u200F", "", // right-to-left mark
	"\u2028", "", // line separator
	"\u2029", "", // paragraph separator
	"\uFEFF", "", // byte order mark
)

// Action represents a parsed deep link action.
type Action struct {
	Query string // Pre-fill prompt (not auto-submitted)
	CWD   string // Working directory (must be absolute)
	Repo  string // GitHub owner/repo slug
}

// ParseDeepLink parses a claude-cli://open URI into an Action.
// Returns error for malformed URIs, control characters, oversized values,
// non-absolute CWD, or invalid repo slugs.
func ParseDeepLink(uri string) (*Action, error) {
	if uri == "" {
		return nil, fmt.Errorf("empty URI")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	// Validate scheme
	if u.Scheme != Protocol {
		return nil, fmt.Errorf("invalid scheme %q: expected %q", u.Scheme, Protocol)
	}

	// Validate host (action) is "open"
	if u.Host != "open" {
		return nil, fmt.Errorf("invalid host %q: expected \"open\"", u.Host)
	}

	params := u.Query()
	action := &Action{}

	// Parse query parameter
	if q := params.Get("q"); q != "" {
		if containsControlChars(q) {
			return nil, fmt.Errorf("query contains control character")
		}
		// Strip hidden Unicode characters
		q = hiddenUnicodeReplacer.Replace(q)
		// Strip tag characters (U+E0000-U+E007F)
		q = stripTagCharacters(q)
		if len(q) > MaxQueryLength {
			return nil, fmt.Errorf("query exceeds maximum length of %d", MaxQueryLength)
		}
		action.Query = q
	}

	// Parse cwd parameter
	if cwd := params.Get("cwd"); cwd != "" {
		if len(cwd) > MaxCWDLength {
			return nil, fmt.Errorf("cwd exceeds maximum length of %d", MaxCWDLength)
		}
		if !isAbsolutePath(cwd) {
			return nil, fmt.Errorf("cwd must be absolute path, got %q", cwd)
		}
		action.CWD = cwd
	}

	// Parse repo parameter
	if repo := params.Get("repo"); repo != "" {
		if !repoSlugRe.MatchString(repo) {
			return nil, fmt.Errorf("invalid repo slug %q: expected owner/repo format", repo)
		}
		action.Repo = repo
	}

	return action, nil
}

// BuildDeepLink constructs a claude-cli://open URI from an Action.
func BuildDeepLink(action *Action) string {
	params := url.Values{}

	if action.Query != "" {
		params.Set("q", action.Query)
	}
	if action.CWD != "" {
		params.Set("cwd", action.CWD)
	}
	if action.Repo != "" {
		params.Set("repo", action.Repo)
	}

	base := Protocol + "://open"
	encoded := params.Encode()
	if encoded == "" {
		return base
	}
	return base + "?" + encoded
}

// containsControlChars checks for ASCII control characters (0x00-0x1F, 0x7F).
func containsControlChars(s string) bool {
	for _, b := range []byte(s) {
		if b <= 0x1F || b == 0x7F {
			return true
		}
	}
	return false
}

// isAbsolutePath checks if a path is absolute (Unix: starts with /,
// Windows: starts with drive letter like C:\ or C:/).
func isAbsolutePath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	// Windows drive path: letter followed by colon and separator
	if len(p) >= 3 && isLetter(p[0]) && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		return true
	}
	return false
}

// isLetter returns true if the byte is an ASCII letter.
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// stripTagCharacters removes Unicode tag characters (U+E0000-U+E007F).
func stripTagCharacters(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0xE0000 && r <= 0xE007F {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

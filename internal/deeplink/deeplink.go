// Package deeplink parses and builds claude-cli:// deep link URIs.
// Deep links allow external tools to launch ClawGo with pre-filled prompts,
// working directories, and repository context.
package deeplink

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Protocol is the URI scheme for ClawGo deep links.
const Protocol = "claude-cli"

// MaxQueryLength is the maximum allowed length for the query parameter.
const MaxQueryLength = 5000

// MaxCWDLength is the maximum allowed length for the cwd parameter.
const MaxCWDLength = 4096

// repoSlugRe validates GitHub owner/repo slugs.
var repoSlugRe = regexp.MustCompile(`^[\w.\-]+/[\w.\-]+$`)

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
	return nil, fmt.Errorf("not implemented")
}

// BuildDeepLink constructs a claude-cli://open URI from an Action.
func BuildDeepLink(action *Action) string {
	_ = url.URL{}
	_ = strings.Builder{}
	_ = utf8.RuneError
	return ""
}

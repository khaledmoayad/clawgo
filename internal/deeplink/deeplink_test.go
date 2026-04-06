package deeplink

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDeepLink_BasicQuery(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open?q=hello+world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", action.Query)
	assert.Empty(t, action.CWD)
	assert.Empty(t, action.Repo)
}

func TestParseDeepLink_QueryAndRepo(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open?q=fix+tests&repo=owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "fix tests", action.Query)
	assert.Equal(t, "owner/repo", action.Repo)
}

func TestParseDeepLink_CWDOnly(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open?cwd=/home/user/project")
	require.NoError(t, err)
	assert.Equal(t, "/home/user/project", action.CWD)
}

func TestParseDeepLink_AllFields(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open?q=hello&cwd=/tmp&repo=owner/name")
	require.NoError(t, err)
	assert.Equal(t, "hello", action.Query)
	assert.Equal(t, "/tmp", action.CWD)
	assert.Equal(t, "owner/name", action.Repo)
}

func TestParseDeepLink_URLEncodedQuery(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open?q=hello%20world%21")
	require.NoError(t, err)
	assert.Equal(t, "hello world!", action.Query)
}

func TestParseDeepLink_ControlCharsInQuery(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?q=hello%00world")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control character")
}

func TestParseDeepLink_ControlCharsTab(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?q=hello%09world")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control character")
}

func TestParseDeepLink_QueryTooLong(t *testing.T) {
	longQuery := strings.Repeat("a", MaxQueryLength+1)
	_, err := ParseDeepLink("claude-cli://open?q=" + longQuery)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query exceeds maximum length")
}

func TestParseDeepLink_QueryAtMaxLength(t *testing.T) {
	maxQuery := strings.Repeat("a", MaxQueryLength)
	action, err := ParseDeepLink("claude-cli://open?q=" + maxQuery)
	require.NoError(t, err)
	assert.Len(t, action.Query, MaxQueryLength)
}

func TestParseDeepLink_CWDTooLong(t *testing.T) {
	longCWD := "/" + strings.Repeat("a", MaxCWDLength)
	_, err := ParseDeepLink("claude-cli://open?cwd=" + longCWD)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cwd exceeds maximum length")
}

func TestParseDeepLink_RelativeCWD(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?cwd=relative/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be absolute")
}

func TestParseDeepLink_DotRelativeCWD(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?cwd=./relative")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be absolute")
}

func TestParseDeepLink_InvalidRepoSlug_NoSlash(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?repo=justarepo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo slug")
}

func TestParseDeepLink_InvalidRepoSlug_TooManySlashes(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?repo=a/b/c")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo slug")
}

func TestParseDeepLink_InvalidRepoSlug_SpecialChars(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://open?repo=owner/repo%20name")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo slug")
}

func TestParseDeepLink_ValidRepoSlugs(t *testing.T) {
	tests := []string{
		"owner/repo",
		"my-org/my-repo",
		"user.name/repo.go",
		"owner/repo-name_v2",
	}
	for _, slug := range tests {
		t.Run(slug, func(t *testing.T) {
			action, err := ParseDeepLink("claude-cli://open?repo=" + slug)
			require.NoError(t, err)
			assert.Equal(t, slug, action.Repo)
		})
	}
}

func TestParseDeepLink_WrongScheme(t *testing.T) {
	_, err := ParseDeepLink("http://example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheme")
}

func TestParseDeepLink_WrongHost(t *testing.T) {
	_, err := ParseDeepLink("claude-cli://close?q=hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestParseDeepLink_InvalidURI(t *testing.T) {
	_, err := ParseDeepLink("not-a-uri")
	assert.Error(t, err)
}

func TestParseDeepLink_EmptyURI(t *testing.T) {
	_, err := ParseDeepLink("")
	assert.Error(t, err)
}

func TestParseDeepLink_NoParams(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open")
	require.NoError(t, err)
	assert.Empty(t, action.Query)
	assert.Empty(t, action.CWD)
	assert.Empty(t, action.Repo)
}

func TestParseDeepLink_WindowsCWD(t *testing.T) {
	action, err := ParseDeepLink("claude-cli://open?cwd=C%3A%5CUsers%5Ctest")
	require.NoError(t, err)
	assert.Equal(t, `C:\Users\test`, action.CWD)
}

func TestParseDeepLink_HiddenUnicode(t *testing.T) {
	// Zero-width space U+200B should be stripped from query
	action, err := ParseDeepLink("claude-cli://open?q=hello%E2%80%8Bworld")
	require.NoError(t, err)
	assert.Equal(t, "helloworld", action.Query)
}

func TestBuildDeepLink_BasicQuery(t *testing.T) {
	uri := BuildDeepLink(&Action{Query: "hello"})
	assert.Equal(t, "claude-cli://open?q=hello", uri)
}

func TestBuildDeepLink_AllFields(t *testing.T) {
	uri := BuildDeepLink(&Action{
		Query: "fix bug",
		CWD:   "/home/user",
		Repo:  "owner/repo",
	})
	assert.Contains(t, uri, "claude-cli://open?")
	assert.Contains(t, uri, "q=fix+bug")
	assert.Contains(t, uri, "cwd=%2Fhome%2Fuser")
	assert.Contains(t, uri, "repo=owner%2Frepo")
}

func TestBuildDeepLink_EmptyAction(t *testing.T) {
	uri := BuildDeepLink(&Action{})
	assert.Equal(t, "claude-cli://open", uri)
}

func TestBuildDeepLink_SpecialCharsEncoded(t *testing.T) {
	uri := BuildDeepLink(&Action{Query: "hello world&more"})
	assert.Contains(t, uri, "q=hello+world%26more")
}

func TestRoundTrip(t *testing.T) {
	original := &Action{
		Query: "fix the bug in main.go",
		CWD:   "/home/user/project",
		Repo:  "owner/repo",
	}

	uri := BuildDeepLink(original)
	parsed, err := ParseDeepLink(uri)
	require.NoError(t, err)

	assert.Equal(t, original.Query, parsed.Query)
	assert.Equal(t, original.CWD, parsed.CWD)
	assert.Equal(t, original.Repo, parsed.Repo)
}

func TestRoundTrip_QueryOnly(t *testing.T) {
	original := &Action{Query: "hello world!"}
	uri := BuildDeepLink(original)
	parsed, err := ParseDeepLink(uri)
	require.NoError(t, err)
	assert.Equal(t, original.Query, parsed.Query)
}

func TestRoundTrip_CWDOnly(t *testing.T) {
	original := &Action{CWD: "/tmp/test dir"}
	uri := BuildDeepLink(original)
	parsed, err := ParseDeepLink(uri)
	require.NoError(t, err)
	assert.Equal(t, original.CWD, parsed.CWD)
}

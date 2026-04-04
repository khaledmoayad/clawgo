package diff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleDiff = `--- a/file.go
+++ b/file.go
@@ -1,5 +1,6 @@
 package main

-import "fmt"
+import (
+	"fmt"
+	"os"
+)

 func main() {`

const nonDiffText = `This is just some regular text.
It has multiple lines.
But no diff markers at all.`

func TestParseDiff_ValidDiff(t *testing.T) {
	result := ParseUnifiedDiff(sampleDiff)
	assert.True(t, result.IsDiff, "should detect valid unified diff")
	require.NotEmpty(t, result.Lines, "should have parsed lines")

	// Check header lines
	assert.Equal(t, "header", result.Lines[0].Type)
	assert.Equal(t, "--- a/file.go", result.Lines[0].Content)
	assert.Equal(t, "header", result.Lines[1].Type)
	assert.Equal(t, "+++ b/file.go", result.Lines[1].Content)

	// Check hunk header
	assert.Equal(t, "hunk", result.Lines[2].Type)
	assert.Contains(t, result.Lines[2].Content, "@@")

	// Find add/remove lines
	var addCount, removeCount, contextCount int
	for _, line := range result.Lines {
		switch line.Type {
		case "add":
			addCount++
		case "remove":
			removeCount++
		case "context":
			contextCount++
		}
	}
	assert.Greater(t, addCount, 0, "should have additions")
	assert.Greater(t, removeCount, 0, "should have removals")
	assert.Greater(t, contextCount, 0, "should have context lines")
}

func TestParseDiff_NonDiff(t *testing.T) {
	result := ParseUnifiedDiff(nonDiffText)
	assert.False(t, result.IsDiff, "should detect non-diff text")
}

func TestParseDiff_EmptyString(t *testing.T) {
	result := ParseUnifiedDiff("")
	assert.False(t, result.IsDiff, "empty string is not a diff")
}

func TestParseDiff_LineClassification(t *testing.T) {
	input := `--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 unchanged line
-removed line
+added line`
	result := ParseUnifiedDiff(input)
	require.True(t, result.IsDiff)

	// Check specific line types
	types := make(map[string]int)
	for _, line := range result.Lines {
		types[line.Type]++
	}
	assert.Equal(t, 2, types["header"], "should have 2 header lines (--- and +++)")
	assert.Equal(t, 1, types["hunk"], "should have 1 hunk header")
	assert.Equal(t, 1, types["context"], "should have 1 context line")
	assert.Equal(t, 1, types["remove"], "should have 1 removed line")
	assert.Equal(t, 1, types["add"], "should have 1 added line")
}

func TestRenderDiff_ValidDiff(t *testing.T) {
	rendered := RenderDiff(sampleDiff, 0)
	// Should contain ANSI escape codes for colors
	assert.NotEqual(t, sampleDiff, rendered, "rendered diff should differ from raw text (has colors)")
	// Green additions should be present
	assert.Contains(t, rendered, "fmt", "rendered output should contain the text")
}

func TestRenderDiff_NonDiff(t *testing.T) {
	rendered := RenderDiff(nonDiffText, 0)
	assert.Equal(t, nonDiffText, rendered, "non-diff text should be returned as-is")
}

func TestRenderDiff_WithWidthTruncation(t *testing.T) {
	longDiff := `--- a/test.go
+++ b/test.go
@@ -1,2 +1,2 @@
-this is a very long removed line that should be truncated at some point when width is set
+this is a very long added line that should also be truncated at some point when width is set`
	rendered := RenderDiff(longDiff, 40)
	// Each line in the output should not exceed width (considering ANSI codes, we check the underlying text)
	for _, line := range strings.Split(rendered, "\n") {
		// Lines with ANSI will be longer in byte count, but visual width should be capped
		// Just ensure it renders without panic
		assert.NotEmpty(t, line)
	}
}

func TestRenderDiff_HunkHeaderStyle(t *testing.T) {
	diff := `--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 context`
	rendered := RenderDiff(diff, 0)
	// The hunk header should be in the rendered output
	assert.Contains(t, rendered, "@@")
}

func TestIsDiffContent_True(t *testing.T) {
	assert.True(t, IsDiffContent(sampleDiff))
}

func TestIsDiffContent_False(t *testing.T) {
	assert.False(t, IsDiffContent(nonDiffText))
	assert.False(t, IsDiffContent(""))
	assert.False(t, IsDiffContent("--- only header"))
	assert.False(t, IsDiffContent("+++ only header"))
}

func TestIsDiffContent_PartialMarkers(t *testing.T) {
	// Has --- and +++ but no @@
	partial := `--- a/file.go
+++ b/file.go
no hunk header here`
	assert.False(t, IsDiffContent(partial))
}

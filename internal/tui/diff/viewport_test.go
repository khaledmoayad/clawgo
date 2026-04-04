package diff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewViewportModel(t *testing.T) {
	vp := NewViewportModel(80, 24)
	assert.Equal(t, 80, vp.Width())
	assert.Equal(t, 24, vp.Height())
}

func TestViewportModel_SetContent(t *testing.T) {
	vp := NewViewportModel(80, 10)
	content := "line1\nline2\nline3\nline4\nline5"
	vp.SetContent(content)
	// The viewport should render something
	view := vp.View()
	assert.Contains(t, view, "line1")
	assert.Contains(t, view, "line5")
}

func TestViewportModel_SetSize(t *testing.T) {
	vp := NewViewportModel(80, 24)
	vp.SetSize(120, 40)
	assert.Equal(t, 120, vp.Width())
	assert.Equal(t, 40, vp.Height())
}

func TestViewportModel_AtTopInitially(t *testing.T) {
	vp := NewViewportModel(80, 10)
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line"
	}
	vp.SetContent(strings.Join(lines, "\n"))
	assert.True(t, vp.AtTop(), "should be at top initially")
	assert.False(t, vp.AtBottom(), "should not be at bottom with 50 lines in 10-high viewport")
}

func TestViewportModel_ScrollDown(t *testing.T) {
	vp := NewViewportModel(80, 5)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	vp.SetContent(strings.Join(lines, "\n"))

	assert.True(t, vp.AtTop())
	vp.ScrollDown(3)
	assert.False(t, vp.AtTop(), "should no longer be at top after scrolling down")
}

func TestViewportModel_ScrollUp(t *testing.T) {
	vp := NewViewportModel(80, 5)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	vp.SetContent(strings.Join(lines, "\n"))

	vp.ScrollDown(5)
	assert.False(t, vp.AtTop())
	vp.ScrollUp(5)
	assert.True(t, vp.AtTop(), "should be back at top after scrolling back up")
}

func TestViewportModel_ViewShowsVisiblePortion(t *testing.T) {
	vp := NewViewportModel(80, 3)
	vp.SetContent("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10")

	view := vp.View()
	assert.Contains(t, view, "line1", "initially should show first lines")
	// After scrolling far enough, line1 should not be visible
	vp.ScrollDown(5)
	view = vp.View()
	assert.NotContains(t, view, "line1", "line1 should not be visible after scrolling down")
}

func TestViewportModel_ShortContent(t *testing.T) {
	vp := NewViewportModel(80, 10)
	vp.SetContent("short\ncontent")

	assert.True(t, vp.AtTop())
	assert.True(t, vp.AtBottom(), "short content should be at bottom (all fits)")
	assert.False(t, vp.NeedsViewport(2), "2 lines in 10-high viewport should not need scrolling")
}

func TestViewportModel_NeedsViewport(t *testing.T) {
	vp := NewViewportModel(80, 10)
	assert.False(t, vp.NeedsViewport(5), "5 lines in 10-high viewport")
	assert.False(t, vp.NeedsViewport(10), "10 lines in 10-high viewport")
	assert.True(t, vp.NeedsViewport(11), "11 lines in 10-high viewport needs scrolling")
	assert.True(t, vp.NeedsViewport(100), "100 lines in 10-high viewport needs scrolling")
}

func TestViewportModel_TotalLines(t *testing.T) {
	vp := NewViewportModel(80, 5)
	vp.SetContent("a\nb\nc\nd\ne\nf\ng")
	assert.Equal(t, 7, vp.TotalLines())
}

func TestViewportModel_ScrollPercent(t *testing.T) {
	vp := NewViewportModel(80, 5)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	vp.SetContent(strings.Join(lines, "\n"))

	// At top, scroll percent should be 0
	pct := vp.ScrollPercent()
	assert.InDelta(t, 0.0, pct, 0.01, "should be at 0%% at top")
}

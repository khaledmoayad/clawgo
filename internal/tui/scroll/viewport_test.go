package scroll

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simpleRenderer returns a render function that produces a fixed number of
// lines per message (line content is "msg{index}-line{n}").
func simpleRenderer(heightPerMsg int) RenderFunc {
	return func(msg DisplayMessage, width int) string {
		var lines []string
		for i := 0; i < heightPerMsg; i++ {
			lines = append(lines, fmt.Sprintf("%s-line%d", msg.Content, i))
		}
		return strings.Join(lines, "\n")
	}
}

// variableRenderer returns a render function where each message's line count
// equals its index + 1 (msg 0 = 1 line, msg 1 = 2 lines, etc.).
func variableRenderer() RenderFunc {
	return func(msg DisplayMessage, width int) string {
		// Parse index from content "msg{N}"
		var idx int
		fmt.Sscanf(msg.Content, "msg%d", &idx)
		var lines []string
		for i := 0; i <= idx; i++ {
			lines = append(lines, fmt.Sprintf("%s-line%d", msg.Content, i))
		}
		return strings.Join(lines, "\n")
	}
}

func makeMessages(n int) []DisplayMessage {
	msgs := make([]DisplayMessage, n)
	for i := range msgs {
		msgs[i] = DisplayMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("msg%d", i),
		}
	}
	return msgs
}

// --- HeightCache tests ---

func TestHeightCacheGetSet(t *testing.T) {
	c := NewHeightCache()

	// Miss
	_, ok := c.Get(0)
	assert.False(t, ok)

	// Set and hit
	c.Set(0, 5)
	h, ok := c.Get(0)
	assert.True(t, ok)
	assert.Equal(t, 5, h)
}

func TestHeightCacheInvalidate(t *testing.T) {
	c := NewHeightCache()
	c.Set(0, 3)
	c.Set(1, 4)
	c.Set(2, 5)

	c.Invalidate(1)
	_, ok := c.Get(1)
	assert.False(t, ok, "invalidated entry should be gone")

	// Others still present
	h, ok := c.Get(0)
	assert.True(t, ok)
	assert.Equal(t, 3, h)
}

func TestHeightCacheInvalidateFrom(t *testing.T) {
	c := NewHeightCache()
	for i := 0; i < 5; i++ {
		c.Set(i, i+1)
	}

	c.InvalidateFrom(2)
	assert.Equal(t, 2, c.Len(), "only entries 0 and 1 should remain")

	_, ok := c.Get(2)
	assert.False(t, ok)

	h, ok := c.Get(1)
	assert.True(t, ok)
	assert.Equal(t, 2, h)
}

func TestHeightCacheClear(t *testing.T) {
	c := NewHeightCache()
	c.Set(0, 1)
	c.Set(1, 2)
	c.Clear()
	assert.Equal(t, 0, c.Len())
}

// --- VirtualScrollViewport tests ---

func TestVirtualScrollRenderWindow(t *testing.T) {
	// 10 messages with variable heights: msg0=1 line, msg1=2 lines, ... msg9=10 lines
	// Total lines = 1+2+3+4+5+6+7+8+9+10 = 55
	msgs := makeMessages(10)
	vp := New(80, 5, variableRenderer())
	vp.SetMessages(msgs)

	assert.Equal(t, 55, vp.TotalLines(), "total lines should be sum 1..10")

	// At scroll offset 0, we should see only the first few messages
	view := vp.View()
	lines := strings.Split(view, "\n")
	// Viewport is 5 lines high, so we get exactly 5 content lines
	assert.Equal(t, 5, len(lines), "view should have exactly viewHeight lines")
	assert.Contains(t, lines[0], "msg0-line0", "first line should be from msg0")
}

func TestScrollToBottom(t *testing.T) {
	// 5 messages, 2 lines each = 10 total lines, viewport = 4
	msgs := makeMessages(5)
	vp := New(80, 4, simpleRenderer(2))
	vp.SetMessages(msgs)

	assert.Equal(t, 10, vp.TotalLines())

	vp.ScrollToBottom()
	view := vp.View()
	lines := strings.Split(view, "\n")
	assert.Equal(t, 4, len(lines))
	// Last visible line should be from the last message
	assert.Contains(t, view, "msg4-line1", "bottom should show the last message's last line")
}

func TestScrollToBottomOnNewMessage(t *testing.T) {
	msgs := makeMessages(5)
	vp := New(80, 4, simpleRenderer(2))
	vp.SetMessages(msgs)
	vp.ScrollToBottom()

	// Add a new message
	msgs = append(msgs, DisplayMessage{Role: "assistant", Content: "msg5"})
	vp.SetMessages(msgs)
	vp.OnNewMessage()

	view := vp.View()
	assert.Contains(t, view, "msg5", "new message should be visible after auto-scroll")
}

func TestScrollPercent(t *testing.T) {
	// 10 messages, 2 lines each = 20 lines, viewport = 5
	msgs := makeMessages(10)
	vp := New(80, 5, simpleRenderer(2))
	vp.SetMessages(msgs)

	// At top
	vp.ScrollToTop()
	assert.InDelta(t, 0.0, vp.ScrollPercent(), 0.01, "top should be 0%%")

	// At bottom
	vp.ScrollToBottom()
	assert.InDelta(t, 1.0, vp.ScrollPercent(), 0.01, "bottom should be 100%%")

	// Somewhere in the middle
	vp.ScrollToTop()
	vp.ScrollDown(7) // offset 7 out of max 15
	pct := vp.ScrollPercent()
	assert.Greater(t, pct, 0.0)
	assert.Less(t, pct, 1.0)
}

func TestScrollUpDown(t *testing.T) {
	msgs := makeMessages(10)
	vp := New(80, 5, simpleRenderer(2))
	vp.SetMessages(msgs)

	vp.ScrollDown(3)
	assert.InDelta(t, 3.0/15.0, vp.ScrollPercent(), 0.01)

	vp.ScrollUp(1)
	assert.InDelta(t, 2.0/15.0, vp.ScrollPercent(), 0.01)
}

func TestScrollClamps(t *testing.T) {
	msgs := makeMessages(3)
	vp := New(80, 5, simpleRenderer(2))
	vp.SetMessages(msgs)

	// Scroll up past top
	vp.ScrollUp(100)
	assert.InDelta(t, 0.0, vp.ScrollPercent(), 0.01)

	// Scroll down past bottom
	vp.ScrollDown(1000)
	assert.InDelta(t, 1.0, vp.ScrollPercent(), 0.01)
}

func TestPageUpDown(t *testing.T) {
	msgs := makeMessages(20)
	vp := New(80, 5, simpleRenderer(2))
	vp.SetMessages(msgs)

	vp.PageDown()
	// Should have scrolled down by viewHeight (5)
	pct := vp.ScrollPercent()
	assert.Greater(t, pct, 0.0)

	vp.PageUp()
	assert.InDelta(t, 0.0, vp.ScrollPercent(), 0.01, "page up from page 1 should return to top")
}

func TestEmptyViewport(t *testing.T) {
	vp := New(80, 5, simpleRenderer(2))
	vp.SetMessages(nil)

	view := vp.View()
	assert.Equal(t, "", view, "empty viewport should return empty string")
	assert.Equal(t, 0, vp.TotalLines())
	assert.InDelta(t, 0.0, vp.ScrollPercent(), 0.01)
}

func TestSetSizeClearsCache(t *testing.T) {
	renderCount := 0
	countingRenderer := func(msg DisplayMessage, width int) string {
		renderCount++
		return fmt.Sprintf("%s (w=%d)", msg.Content, width)
	}

	msgs := makeMessages(3)
	vp := New(80, 5, countingRenderer)
	vp.SetMessages(msgs)

	// Cache should be populated after SetMessages (which calls recalcTotalLines)
	require.Equal(t, 3, vp.cache.Len())
	initialRenderCount := renderCount

	// Changing width clears cache, then recalculates (re-renders all messages)
	vp.SetSize(120, 5)
	assert.Equal(t, 3, vp.cache.Len(), "cache repopulated after recalc")
	assert.Equal(t, initialRenderCount+3, renderCount, "all 3 messages re-rendered after width change")

	// Same width -- no cache clear
	prevCount := renderCount
	vp.SetSize(120, 10)
	assert.Equal(t, prevCount, renderCount, "same width should not re-render")
}

func TestAutoScrollDisabledOnManualScroll(t *testing.T) {
	msgs := makeMessages(10)
	vp := New(80, 5, simpleRenderer(2))
	vp.SetMessages(msgs)

	assert.True(t, vp.AutoScroll(), "auto-scroll should be on by default")

	vp.ScrollUp(1)
	assert.False(t, vp.AutoScroll(), "manual scroll should disable auto-scroll")

	vp.ScrollToBottom()
	assert.True(t, vp.AutoScroll(), "scroll-to-bottom should re-enable auto-scroll")
}

func TestContentFitsViewport(t *testing.T) {
	// 2 messages, 2 lines each = 4 total; viewport = 10
	msgs := makeMessages(2)
	vp := New(80, 10, simpleRenderer(2))
	vp.SetMessages(msgs)

	assert.Equal(t, 4, vp.TotalLines())
	assert.InDelta(t, 0.0, vp.ScrollPercent(), 0.01, "content fits viewport -- percent should be 0")

	view := vp.View()
	lines := strings.Split(view, "\n")
	// Should have content + padding
	assert.Equal(t, 10, len(lines), "view should be padded to viewHeight")
}

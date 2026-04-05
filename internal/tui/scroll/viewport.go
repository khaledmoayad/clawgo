package scroll

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// DisplayMessage is the interface that the viewport needs from messages.
// Matches the tui.DisplayMessage struct so callers can pass it directly.
type DisplayMessage struct {
	Role        string
	Content     string
	ToolName    string
	DiffContent bool
}

// RenderFunc renders a DisplayMessage into a string for the given terminal width.
// The returned string may contain newlines; the viewport counts lines to
// determine how much vertical space the message occupies.
type RenderFunc func(msg DisplayMessage, width int) string

// VirtualScrollViewport renders only the visible portion of a message list.
// Messages that are fully off-screen are never rendered, and rendered heights
// are cached via HeightCache so repeated frames are cheap.
type VirtualScrollViewport struct {
	messages     []DisplayMessage
	cache        *HeightCache
	scrollOffset int // first visible line (0 = top of conversation)
	viewHeight   int // terminal lines available for messages
	viewWidth    int
	totalLines   int // sum of all message heights
	renderFunc   RenderFunc
	autoScroll   bool // when true, new messages scroll the view to bottom
}

// New creates a VirtualScrollViewport with the given dimensions and render function.
func New(viewWidth, viewHeight int, renderFn RenderFunc) *VirtualScrollViewport {
	return &VirtualScrollViewport{
		cache:      NewHeightCache(),
		viewWidth:  viewWidth,
		viewHeight: viewHeight,
		renderFunc: renderFn,
		autoScroll: true,
	}
}

// SetMessages replaces the message slice and recalculates total line count.
func (v *VirtualScrollViewport) SetMessages(msgs []DisplayMessage) {
	prevLen := len(v.messages)
	v.messages = msgs

	// Invalidate cache entries for messages that may have changed
	if len(msgs) != prevLen {
		v.cache.InvalidateFrom(prevLen)
	}

	v.recalcTotalLines()
}

// SetSize updates the viewport dimensions (e.g., on terminal resize).
// Clears the height cache because line wrapping changes with width.
func (v *VirtualScrollViewport) SetSize(w, h int) {
	if w != v.viewWidth {
		v.cache.Clear()
		v.viewWidth = w
		v.recalcTotalLines()
	}
	v.viewHeight = h
	v.clampScrollOffset()
}

// ScrollUp moves the viewport up by n lines.
func (v *VirtualScrollViewport) ScrollUp(n int) {
	v.autoScroll = false
	v.scrollOffset -= n
	if v.scrollOffset < 0 {
		v.scrollOffset = 0
	}
}

// ScrollDown moves the viewport down by n lines.
func (v *VirtualScrollViewport) ScrollDown(n int) {
	v.autoScroll = false
	v.scrollOffset += n
	v.clampScrollOffset()
}

// ScrollToTop scrolls to the very beginning of the conversation.
func (v *VirtualScrollViewport) ScrollToTop() {
	v.autoScroll = false
	v.scrollOffset = 0
}

// ScrollToBottom scrolls to the very end of the conversation and re-enables
// auto-scroll so new messages continue to appear.
func (v *VirtualScrollViewport) ScrollToBottom() {
	v.autoScroll = true
	maxOffset := v.maxScrollOffset()
	if maxOffset < 0 {
		maxOffset = 0
	}
	v.scrollOffset = maxOffset
}

// PageUp scrolls up by one full page (viewHeight lines).
func (v *VirtualScrollViewport) PageUp() {
	v.ScrollUp(v.viewHeight)
}

// PageDown scrolls down by one full page (viewHeight lines).
func (v *VirtualScrollViewport) PageDown() {
	v.ScrollDown(v.viewHeight)
}

// OnNewMessage should be called when a new message is appended.
// If autoScroll is enabled, the viewport scrolls to show the new message.
// The last message is invalidated in case it was a streaming message whose
// content changed.
func (v *VirtualScrollViewport) OnNewMessage() {
	if len(v.messages) > 0 {
		v.cache.Invalidate(len(v.messages) - 1)
	}
	v.recalcTotalLines()
	if v.autoScroll {
		v.ScrollToBottom()
	}
}

// TotalLines returns the total number of lines across all messages.
func (v *VirtualScrollViewport) TotalLines() int {
	return v.totalLines
}

// ScrollPercent returns the scroll position as a percentage (0.0 at top, 1.0 at bottom).
// Returns 0.0 if all content fits in the viewport.
func (v *VirtualScrollViewport) ScrollPercent() float64 {
	maxOff := v.maxScrollOffset()
	if maxOff <= 0 {
		return 0.0
	}
	pct := float64(v.scrollOffset) / float64(maxOff)
	if pct > 1.0 {
		return 1.0
	}
	return pct
}

// AutoScroll returns whether the viewport is auto-following new content.
func (v *VirtualScrollViewport) AutoScroll() bool {
	return v.autoScroll
}

// Update handles Bubble Tea messages (key events for scrolling).
func (v *VirtualScrollViewport) Update(msg tea.Msg) (*VirtualScrollViewport, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		k := msg.Key()
		switch {
		case k.Code == tea.KeyUp:
			v.ScrollUp(1)
		case k.Code == tea.KeyDown:
			v.ScrollDown(1)
		case k.Code == 'k':
			v.ScrollUp(1)
		case k.Code == 'j':
			v.ScrollDown(1)
		case k.Code == tea.KeyPgUp:
			v.PageUp()
		case k.Code == tea.KeyPgDown:
			v.PageDown()
		case k.Code == tea.KeyHome:
			v.ScrollToTop()
		case k.Code == tea.KeyEnd:
			v.ScrollToBottom()
		case k.Code == 'g':
			v.ScrollToTop()
		case k.Code == 'G':
			v.ScrollToBottom()
		}
	case tea.WindowSizeMsg:
		v.SetSize(msg.Width, msg.Height)
	}
	return v, nil
}

// View renders the visible portion of the message list. This is the core
// virtual scrolling algorithm:
//
//  1. Walk messages to find the first one whose cumulative height exceeds scrollOffset.
//  2. Render only messages from that point through scrollOffset + viewHeight.
//  3. Trim the first visible message if scrollOffset falls mid-message.
//  4. Pad or trim the last visible message to exactly fill viewHeight.
//  5. Return the composed string.
func (v *VirtualScrollViewport) View() string {
	if len(v.messages) == 0 || v.viewHeight <= 0 {
		return ""
	}

	// Step 1: Find the first visible message
	cumulative := 0
	firstIdx := 0
	firstLineSkip := 0

	for i := range v.messages {
		h := v.ensureMessageHeight(i)
		if cumulative+h > v.scrollOffset {
			firstIdx = i
			firstLineSkip = v.scrollOffset - cumulative
			break
		}
		cumulative += h
		if i == len(v.messages)-1 {
			// scrollOffset beyond all messages -- show last screen
			firstIdx = i
			firstLineSkip = 0
		}
	}

	// Step 2-4: Render visible messages and compose output
	outputLines := make([]string, 0, v.viewHeight)

	for i := firstIdx; i < len(v.messages) && len(outputLines) < v.viewHeight; i++ {
		rendered := v.renderFunc(v.messages[i], v.viewWidth)
		lines := strings.Split(rendered, "\n")

		// For the first message, skip lines above the scroll offset
		startLine := 0
		if i == firstIdx {
			startLine = firstLineSkip
		}

		for j := startLine; j < len(lines) && len(outputLines) < v.viewHeight; j++ {
			outputLines = append(outputLines, lines[j])
		}
	}

	// Pad with empty lines if content doesn't fill the viewport
	for len(outputLines) < v.viewHeight {
		outputLines = append(outputLines, "")
	}

	return strings.Join(outputLines, "\n")
}

// ensureMessageHeight returns the height (line count) for the message at idx,
// computing and caching it if not already cached.
func (v *VirtualScrollViewport) ensureMessageHeight(idx int) int {
	if h, ok := v.cache.Get(idx); ok {
		return h
	}
	rendered := v.renderFunc(v.messages[idx], v.viewWidth)
	h := strings.Count(rendered, "\n") + 1
	v.cache.Set(idx, h)
	return h
}

// recalcTotalLines recomputes the total line count from all messages.
func (v *VirtualScrollViewport) recalcTotalLines() {
	total := 0
	for i := range v.messages {
		total += v.ensureMessageHeight(i)
	}
	v.totalLines = total
}

// maxScrollOffset returns the maximum valid scroll offset.
func (v *VirtualScrollViewport) maxScrollOffset() int {
	return v.totalLines - v.viewHeight
}

// clampScrollOffset ensures scrollOffset is within valid bounds.
func (v *VirtualScrollViewport) clampScrollOffset() {
	maxOff := v.maxScrollOffset()
	if maxOff < 0 {
		v.scrollOffset = 0
		return
	}
	if v.scrollOffset > maxOff {
		v.scrollOffset = maxOff
	}
	if v.scrollOffset < 0 {
		v.scrollOffset = 0
	}
}

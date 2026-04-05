package overlay

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// --- OverlayManager Tests ---

func TestOverlayManagerPushPopCurrent(t *testing.T) {
	mgr := NewOverlayManager()

	if mgr.IsActive() {
		t.Fatal("expected empty manager to be inactive")
	}
	if mgr.Current() != nil {
		t.Fatal("expected Current() to be nil on empty manager")
	}
	if mgr.Size() != 0 {
		t.Fatalf("expected size 0, got %d", mgr.Size())
	}

	// Push first overlay
	o1 := NewMessageSelector([]DisplayMessage{
		{Role: "user", Content: "hello"},
	})
	mgr.Push(o1)

	if !mgr.IsActive() {
		t.Fatal("expected manager to be active after push")
	}
	if mgr.Current() != o1 {
		t.Fatal("expected Current() to return pushed overlay")
	}
	if mgr.Size() != 1 {
		t.Fatalf("expected size 1, got %d", mgr.Size())
	}

	// Push second overlay (stacking)
	o2 := NewHistorySearch([]HistoryItem{
		{Text: "test prompt", Date: "2024-01-15", SessionID: "abc"},
	})
	mgr.Push(o2)

	if mgr.Size() != 2 {
		t.Fatalf("expected size 2 after second push, got %d", mgr.Size())
	}
	if mgr.Current() != o2 {
		t.Fatal("expected Current() to return topmost overlay")
	}

	// Pop top
	popped := mgr.Pop()
	if popped != o2 {
		t.Fatal("expected Pop() to return the second overlay")
	}
	if mgr.Size() != 1 {
		t.Fatalf("expected size 1 after pop, got %d", mgr.Size())
	}
	if mgr.Current() != o1 {
		t.Fatal("expected Current() to return first overlay after pop")
	}

	// Pop last
	mgr.Pop()
	if mgr.IsActive() {
		t.Fatal("expected manager to be inactive after popping all")
	}
	if mgr.Size() != 0 {
		t.Fatalf("expected size 0 after popping all, got %d", mgr.Size())
	}

	// Pop on empty should return nil
	if mgr.Pop() != nil {
		t.Fatal("expected Pop() on empty to return nil")
	}
}

func TestOverlayManagerAutoPop(t *testing.T) {
	mgr := NewOverlayManager()
	o := NewMessageSelector([]DisplayMessage{
		{Role: "user", Content: "hello"},
	})
	mgr.Push(o)

	// Send Escape to dismiss the overlay
	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	mgr.Update(msg)

	// After dismiss, manager should auto-pop
	if mgr.IsActive() {
		t.Fatal("expected manager to auto-pop dismissed overlay")
	}
	if mgr.Size() != 0 {
		t.Fatalf("expected size 0 after auto-pop, got %d", mgr.Size())
	}
}

// --- MessageSelector Tests ---

func TestMessageSelectorFilter(t *testing.T) {
	messages := []DisplayMessage{
		{Role: "user", Content: "How do I write a Go test?"},
		{Role: "assistant", Content: "Here is an example test in Go using the testing package."},
		{Role: "user", Content: "What about benchmarks?"},
		{Role: "assistant", Content: "Benchmarks use testing.B with the Benchmark prefix."},
	}

	sel := NewMessageSelector(messages)

	// Initially all messages visible
	if len(sel.filtered) != 4 {
		t.Fatalf("expected 4 filtered items initially, got %d", len(sel.filtered))
	}

	// Set filter text directly (textinput keystroke simulation is unreliable in tests)
	sel.textinput.SetValue("benchmark")
	sel.applyFilter()

	// After filtering, should match "benchmarks" and "Benchmark"
	if len(sel.filtered) != 2 {
		t.Fatalf("expected 2 filtered items for 'benchmark', got %d", len(sel.filtered))
	}

	// Verify the matches contain expected content
	for _, item := range sel.filtered {
		lower := strings.ToLower(item.Preview)
		if !strings.Contains(lower, "benchmark") {
			t.Errorf("filtered item %q does not contain 'benchmark'", item.Preview)
		}
	}
}

func TestMessageSelectorEscape(t *testing.T) {
	sel := NewMessageSelector([]DisplayMessage{
		{Role: "user", Content: "hello"},
	})

	if !sel.IsActive() {
		t.Fatal("expected selector to be active initially")
	}

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	updated, _ := sel.Update(msg)
	sel = updated.(*MessageSelectorOverlay)

	if sel.IsActive() {
		t.Fatal("expected selector to be inactive after Escape")
	}
}

// --- TranscriptSearch Tests ---

func TestTranscriptSearchMatches(t *testing.T) {
	messages := []DisplayMessage{
		{Role: "user", Content: "How do I write tests?\nI need help with testing."},
		{Role: "assistant", Content: "Here is a test example.\nUse the testing package for tests."},
		{Role: "user", Content: "No tests needed here."},
	}

	search := NewTranscriptSearch(messages)

	// Set query directly (textinput keystroke simulation is unreliable in tests)
	search.query = "test"
	search.textinput.SetValue("test")
	search.scanMatches()

	matches := search.Matches()

	// "test" should match multiple times:
	// msg0 line0: "tests" (1), msg0 line1: "testing" (1)
	// msg1 line0: "test" (1), msg1 line1: "testing" + "tests" (2)
	// msg2 line0: "tests" (1)
	// Total: 6
	if len(matches) != 6 {
		t.Fatalf("expected 6 matches for 'test', got %d", len(matches))
	}

	// Verify all matches are within valid message range
	for _, m := range matches {
		if m.MsgIndex < 0 || m.MsgIndex >= len(messages) {
			t.Errorf("match MsgIndex %d out of range", m.MsgIndex)
		}
		if m.Start < 0 || m.End <= m.Start {
			t.Errorf("invalid match range: start=%d end=%d", m.Start, m.End)
		}
	}
}

func TestTranscriptSearchHighlight(t *testing.T) {
	result := HighlightMatches("Hello World hello", "hello")

	// Should contain styled "Hello" and "hello" (case-insensitive match)
	if !strings.Contains(result, "World") {
		t.Error("expected non-matching text 'World' to be preserved")
	}
	// The highlighted text should not be plain "Hello" anymore (it has ANSI codes)
	if result == "Hello World hello" {
		t.Error("expected highlighting to modify the output")
	}
}

func TestTranscriptSearchEmpty(t *testing.T) {
	search := NewTranscriptSearch([]DisplayMessage{})
	matches := search.Matches()
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches on empty messages, got %d", len(matches))
	}
}

// --- Fullscreen Tests ---

func TestFullscreenScroll(t *testing.T) {
	// Create content taller than viewport
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, strings.Repeat("x", 40))
	}
	content := strings.Join(lines, "\n")

	fs := NewFullscreen("Test", content, 80, 20)

	if !fs.IsActive() {
		t.Fatal("expected fullscreen to be active")
	}

	// Scroll down
	for i := 0; i < 5; i++ {
		msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
		fs.Update(msg)
	}

	// Verify it rendered without panic
	view := fs.View(80, 20)
	if view == "" {
		t.Fatal("expected non-empty view")
	}

	// Verify title appears in view
	if !strings.Contains(view, "Test") {
		t.Error("expected title 'Test' in fullscreen view")
	}

	// Verify status bar info
	if !strings.Contains(view, "Esc") {
		t.Error("expected 'Esc' hint in status bar")
	}
	if !strings.Contains(view, "Ctrl+O") {
		t.Error("expected 'Ctrl+O' hint in status bar")
	}
}

func TestFullscreenDismissEscape(t *testing.T) {
	fs := NewFullscreen("Title", "content", 80, 20)

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	updated, _ := fs.Update(msg)
	fs = updated.(*FullscreenOverlay)

	if fs.IsActive() {
		t.Fatal("expected fullscreen to be inactive after Escape")
	}
}

func TestFullscreenDismissCtrlO(t *testing.T) {
	fs := NewFullscreen("Title", "content", 80, 20)

	msg := tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl})
	updated, _ := fs.Update(msg)
	fs = updated.(*FullscreenOverlay)

	if fs.IsActive() {
		t.Fatal("expected fullscreen to be inactive after Ctrl+O")
	}
}

// --- OverlayManager View Tests ---

func TestOverlayManagerView(t *testing.T) {
	mgr := NewOverlayManager()

	// Empty manager should render empty
	if mgr.View(80, 24) != "" {
		t.Fatal("expected empty view for inactive manager")
	}

	// Push selector and verify view renders
	sel := NewMessageSelector([]DisplayMessage{
		{Role: "user", Content: "test message"},
	})
	mgr.Push(sel)

	view := mgr.View(80, 24)
	if view == "" {
		t.Fatal("expected non-empty view when overlay is active")
	}
}

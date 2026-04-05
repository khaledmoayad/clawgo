package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"charm.land/lipgloss/v2"
)

func TestNotificationModel_AddAndShow(t *testing.T) {
	m := NewNotificationModel()

	cmd := m.Add(Notification{
		Key:      "test-1",
		Text:     "Hello, notification!",
		Priority: PriorityMedium,
	})

	assert.NotNil(t, m.Current())
	assert.Equal(t, "test-1", m.Current().Key)
	assert.Equal(t, "Hello, notification!", m.Current().Text)
	assert.Equal(t, 0, m.QueueLen())
	// A dismiss command should be scheduled
	assert.NotNil(t, cmd)
}

func TestNotificationModel_QueueOrdering(t *testing.T) {
	m := NewNotificationModel()

	// Add first notification (becomes current)
	m.Add(Notification{Key: "low-1", Text: "Low", Priority: PriorityLow})
	assert.Equal(t, "low-1", m.Current().Key)

	// Add more while current is showing
	m.Add(Notification{Key: "high-1", Text: "High", Priority: PriorityHigh})
	m.Add(Notification{Key: "med-1", Text: "Medium", Priority: PriorityMedium})
	m.Add(Notification{Key: "low-2", Text: "Low 2", Priority: PriorityLow})

	// Queue should be ordered by priority
	assert.Equal(t, 3, m.QueueLen())

	// Dismiss current, high should be next
	m.Dismiss("low-1")
	assert.Equal(t, "high-1", m.Current().Key)

	// Dismiss again, medium next
	m.Dismiss("high-1")
	assert.Equal(t, "med-1", m.Current().Key)

	// Dismiss again, low-2 next
	m.Dismiss("med-1")
	assert.Equal(t, "low-2", m.Current().Key)
}

func TestNotificationModel_ImmediatePriority(t *testing.T) {
	m := NewNotificationModel()

	// Show a regular notification
	m.Add(Notification{Key: "regular", Text: "Regular", Priority: PriorityMedium})
	assert.Equal(t, "regular", m.Current().Key)

	// Add immediate - should pre-empt current
	m.Add(Notification{Key: "urgent", Text: "Urgent!", Priority: PriorityImmediate})
	assert.Equal(t, "urgent", m.Current().Key)

	// Regular should be re-queued
	assert.Equal(t, 1, m.QueueLen())
}

func TestNotificationModel_Dedup(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "dup", Text: "First", Priority: PriorityMedium})
	m.Add(Notification{Key: "dup", Text: "Second", Priority: PriorityMedium})

	// Should still have only the first one (dedup prevents double-add)
	assert.Equal(t, "First", m.Current().Text)
	assert.Equal(t, 0, m.QueueLen())
}

func TestNotificationModel_DedupInQueue(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "current", Text: "Current", Priority: PriorityMedium})
	m.Add(Notification{Key: "queued", Text: "Queued 1", Priority: PriorityMedium})
	m.Add(Notification{Key: "queued", Text: "Queued 2", Priority: PriorityMedium})

	assert.Equal(t, 1, m.QueueLen())
}

func TestNotificationModel_Fold(t *testing.T) {
	m := NewNotificationModel()

	// Register a fold function that appends text
	m.RegisterFold("foldable", func(existing, incoming Notification) Notification {
		existing.Text = existing.Text + " + " + incoming.Text
		return existing
	})

	m.Add(Notification{Key: "foldable", Text: "First", Priority: PriorityMedium})
	m.Add(Notification{Key: "foldable", Text: "Second", Priority: PriorityMedium})

	// Should be folded
	assert.Equal(t, "First + Second", m.Current().Text)
	assert.Equal(t, 0, m.QueueLen())
}

func TestNotificationModel_FoldInQueue(t *testing.T) {
	m := NewNotificationModel()

	m.RegisterFold("foldable", func(existing, incoming Notification) Notification {
		existing.Text = existing.Text + " + " + incoming.Text
		return existing
	})

	// Occupy current with something else
	m.Add(Notification{Key: "other", Text: "Other", Priority: PriorityMedium})

	// Add foldable to queue
	m.Add(Notification{Key: "foldable", Text: "First", Priority: PriorityMedium})
	m.Add(Notification{Key: "foldable", Text: "Second", Priority: PriorityMedium})

	assert.Equal(t, 1, m.QueueLen())
}

func TestNotificationModel_Invalidation(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "old", Text: "Old", Priority: PriorityMedium})
	m.Add(Notification{Key: "queued-old", Text: "Queued Old", Priority: PriorityMedium})

	// Add notification that invalidates "queued-old"
	m.Add(Notification{
		Key:         "new",
		Text:        "New",
		Priority:    PriorityMedium,
		Invalidates: []string{"queued-old"},
	})

	// "queued-old" should be removed from queue
	assert.Equal(t, 1, m.QueueLen()) // only "new" remains
}

func TestNotificationModel_InvalidateCurrent(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "current", Text: "Current", Priority: PriorityMedium})

	// Invalidate the current notification
	m.Add(Notification{
		Key:         "replacement",
		Text:        "Replacement",
		Priority:    PriorityMedium,
		Invalidates: []string{"current"},
	})

	// Current should now be "replacement"
	assert.Equal(t, "replacement", m.Current().Key)
}

func TestNotificationModel_Dismiss(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "a", Text: "A", Priority: PriorityMedium})
	m.Add(Notification{Key: "b", Text: "B", Priority: PriorityMedium})

	assert.Equal(t, "a", m.Current().Key)
	m.Dismiss("a")
	assert.Equal(t, "b", m.Current().Key)
}

func TestNotificationModel_DismissNotCurrent(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "a", Text: "A", Priority: PriorityMedium})
	m.Add(Notification{Key: "b", Text: "B", Priority: PriorityMedium})

	// Dismiss queued item
	m.Dismiss("b")
	assert.Equal(t, "a", m.Current().Key)
	assert.Equal(t, 0, m.QueueLen())
}

func TestNotificationModel_DismissAll(t *testing.T) {
	m := NewNotificationModel()

	m.Add(Notification{Key: "a", Text: "A", Priority: PriorityMedium})
	m.Dismiss("a")

	assert.Nil(t, m.Current())
	assert.Equal(t, 0, m.QueueLen())
}

func TestNotificationModel_View(t *testing.T) {
	m := NewNotificationModel()
	m.SetWidth(80)

	// No notification - empty view
	assert.Equal(t, "", m.View())

	// Add notification
	m.Add(Notification{Key: "test", Text: "Test notification", Priority: PriorityMedium})
	view := m.View()
	assert.Contains(t, view, "Test notification")
}

func TestNotificationModel_ViewWithQueue(t *testing.T) {
	m := NewNotificationModel()
	m.SetWidth(80)

	m.Add(Notification{Key: "a", Text: "Current", Priority: PriorityMedium})
	m.Add(Notification{Key: "b", Text: "Queued", Priority: PriorityMedium})

	view := m.View()
	assert.Contains(t, view, "Current")
	// Should show queue indicator
	assert.Contains(t, view, "+")
}

func TestNotificationModel_ViewTruncation(t *testing.T) {
	m := NewNotificationModel()
	m.SetWidth(20)

	longText := "This is a very long notification that should be truncated"
	m.Add(Notification{Key: "long", Text: longText, Priority: PriorityMedium})

	view := m.View()
	assert.Contains(t, view, "...")
}

func TestNotificationModel_CustomColor(t *testing.T) {
	m := NewNotificationModel()
	m.SetWidth(80)

	m.Add(Notification{
		Key:      "colored",
		Text:     "Success!",
		Color:    lipgloss.Color("#98C379"),
		Priority: PriorityMedium,
	})

	// Just verify it renders without error
	view := m.View()
	assert.Contains(t, view, "Success!")
}

func TestNotificationModel_CustomTimeout(t *testing.T) {
	m := NewNotificationModel()

	cmd := m.Add(Notification{
		Key:      "fast",
		Text:     "Quick",
		Priority: PriorityMedium,
		Timeout:  500 * time.Millisecond,
	})

	require.NotNil(t, cmd)
	assert.NotNil(t, m.Current())
}

func TestNotificationModel_UpdateDismissMsg(t *testing.T) {
	m := NewNotificationModel()
	m.Add(Notification{Key: "test", Text: "Test", Priority: PriorityMedium})

	m, cmd := m.Update(notificationDismissMsg{key: "test"})
	// The dismiss msg should trigger a Dismiss call
	_ = cmd // cmd might schedule next notification
	// After processing, the notification should eventually be cleared
}

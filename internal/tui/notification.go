package tui

import (
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// NotificationPriority determines display order and behavior of notifications.
type NotificationPriority int

const (
	// PriorityLow notifications display after all higher-priority ones are shown.
	PriorityLow NotificationPriority = iota
	// PriorityMedium notifications display before low-priority ones.
	PriorityMedium
	// PriorityHigh notifications display before medium and low.
	PriorityHigh
	// PriorityImmediate notifications pre-empt the current notification.
	PriorityImmediate
)

// DefaultNotificationTimeout is the duration a notification displays before auto-dismiss.
const DefaultNotificationTimeout = 8 * time.Second

// Notification represents a single toast notification.
type Notification struct {
	Key      string               // Unique identifier for dedup and fold
	Text  string      // Display text
	Color color.Color // Text color (nil means default)
	Priority NotificationPriority // Display priority
	Timeout  time.Duration        // Auto-dismiss duration (0 = use default)

	// Invalidates lists keys of other notifications that this one cancels.
	Invalidates []string

	// CreatedAt tracks when the notification was created for FIFO ordering.
	CreatedAt time.Time
}

// NotificationFoldFunc merges two notifications with the same key.
// Called as fold(existing, incoming) and returns the merged result.
type NotificationFoldFunc func(existing, incoming Notification) Notification

// notificationDismissMsg is sent when a notification's timeout expires.
type notificationDismissMsg struct {
	key string
}

// NotificationModel manages a priority queue of toast notifications.
// It mirrors the TypeScript useNotifications hook with queue, current,
// priority ordering, fold/merge, and timeout-based auto-dismiss.
type NotificationModel struct {
	current *Notification  // Currently displayed notification
	queue   []Notification // Pending notifications, ordered by priority then FIFO
	folds   map[string]NotificationFoldFunc
	width   int
}

// NewNotificationModel creates a new notification model.
func NewNotificationModel() NotificationModel {
	return NotificationModel{
		queue: make([]Notification, 0),
		folds: make(map[string]NotificationFoldFunc),
		width: 60,
	}
}

// SetWidth sets the available terminal width for rendering.
func (m *NotificationModel) SetWidth(w int) {
	m.width = w
}

// RegisterFold registers a fold function for notifications with the given key.
func (m *NotificationModel) RegisterFold(key string, fn NotificationFoldFunc) {
	m.folds[key] = fn
}

// Add queues a notification for display. If a fold function is registered
// for this key and a notification with the same key exists, they are merged.
func (m *NotificationModel) Add(n Notification) tea.Cmd {
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	timeout := n.Timeout
	if timeout == 0 {
		timeout = DefaultNotificationTimeout
	}

	// Handle invalidation: remove invalidated notifications from queue and current
	if len(n.Invalidates) > 0 {
		invalidSet := make(map[string]bool, len(n.Invalidates))
		for _, k := range n.Invalidates {
			invalidSet[k] = true
		}
		if m.current != nil && invalidSet[m.current.Key] {
			m.current = nil
		}
		filtered := m.queue[:0]
		for _, q := range m.queue {
			if !invalidSet[q.Key] {
				filtered = append(filtered, q)
			}
		}
		m.queue = filtered
	}

	// Try fold
	if fold, ok := m.folds[n.Key]; ok {
		// Fold into current
		if m.current != nil && m.current.Key == n.Key {
			merged := fold(*m.current, n)
			m.current = &merged
			return m.scheduleDismiss(merged.Key, timeout)
		}
		// Fold into queued
		for i, q := range m.queue {
			if q.Key == n.Key {
				m.queue[i] = fold(q, n)
				return nil // Don't need to schedule dismiss for queued items yet
			}
		}
	}

	// Dedup: skip if key already queued or current
	if m.current != nil && m.current.Key == n.Key {
		return nil
	}
	for _, q := range m.queue {
		if q.Key == n.Key {
			return nil
		}
	}

	// Handle immediate priority: pre-empt current
	if n.Priority == PriorityImmediate {
		if m.current != nil {
			// Re-queue the current notification (unless it's also immediate)
			if m.current.Priority != PriorityImmediate {
				m.queue = append([]Notification{*m.current}, m.queue...)
			}
		}
		m.current = &n
		return m.scheduleDismiss(n.Key, timeout)
	}

	// Insert into queue in priority order (higher priority first, FIFO within same priority)
	inserted := false
	for i, q := range m.queue {
		if n.Priority > q.Priority {
			m.queue = append(m.queue[:i], append([]Notification{n}, m.queue[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		m.queue = append(m.queue, n)
	}

	// If no current notification, show next
	if m.current == nil {
		return m.showNext()
	}
	return nil
}

// Dismiss removes the notification with the given key.
func (m *NotificationModel) Dismiss(key string) tea.Cmd {
	if m.current != nil && m.current.Key == key {
		m.current = nil
		return m.showNext()
	}
	// Remove from queue
	for i, q := range m.queue {
		if q.Key == key {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			break
		}
	}
	return nil
}

// Current returns the currently displayed notification, or nil.
func (m NotificationModel) Current() *Notification {
	return m.current
}

// QueueLen returns the number of queued notifications.
func (m NotificationModel) QueueLen() int {
	return len(m.queue)
}

// showNext pops the highest-priority notification from the queue and makes it current.
func (m *NotificationModel) showNext() tea.Cmd {
	if len(m.queue) == 0 {
		return nil
	}
	next := m.queue[0]
	m.queue = m.queue[1:]
	m.current = &next

	timeout := next.Timeout
	if timeout == 0 {
		timeout = DefaultNotificationTimeout
	}
	return m.scheduleDismiss(next.Key, timeout)
}

// scheduleDismiss creates a command that fires a dismiss message after the timeout.
func (m *NotificationModel) scheduleDismiss(key string, timeout time.Duration) tea.Cmd {
	return tea.Tick(timeout, func(time.Time) tea.Msg {
		return notificationDismissMsg{key: key}
	})
}

// Update processes notification-related messages.
func (m NotificationModel) Update(msg tea.Msg) (NotificationModel, tea.Cmd) {
	switch msg := msg.(type) {
	case notificationDismissMsg:
		return m, m.Dismiss(msg.key)
	}
	return m, nil
}

// View renders the current notification as a styled toast bar.
func (m NotificationModel) View() string {
	if m.current == nil {
		return ""
	}

	n := m.current
	text := n.Text

	// Truncate to available width
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 10
	}
	if len(text) > maxW {
		text = text[:maxW-3] + "..."
	}

	c := n.Color
	if c == nil {
		c = lipgloss.Color("#61AFEF")
	}

	style := lipgloss.NewStyle().
		Foreground(c).
		PaddingLeft(1).
		PaddingRight(1)

	// Show queue indicator if there are more notifications pending
	queueIndicator := ""
	if len(m.queue) > 0 {
		queueIndicator = DimStyle.Render(strings.Repeat(" ", 1) + "+" + strings.Repeat(".", min(len(m.queue), 3)))
	}

	return style.Render(text) + queueIndicator
}

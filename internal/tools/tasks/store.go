// Package tasks provides a shared in-memory task store used by all task management tools.
// Tasks represent background work units (sub-processes, sub-agents).
package tasks

import (
	"context"
	"fmt"
	"sync"
)

// Task represents a background task entry.
type Task struct {
	ID          string             `json:"id"`
	Description string             `json:"description"`
	Type        string             `json:"type"`   // "local_bash", "local_agent"
	Status      string             `json:"status"` // "pending", "running", "completed", "stopped", "failed"
	Output      string             `json:"output"`
	CancelFunc  context.CancelFunc `json:"-"`   // Used to cancel running agents
	OutputCh    chan string         `json:"-"`   // Streaming output chunks (buffered channel, capacity 100)
}

// Store is a thread-safe in-memory task registry.
type Store struct {
	mu      sync.Mutex
	tasks   map[string]*Task
	counter int
}

// NewStore creates an empty task store.
func NewStore() *Store {
	return &Store{
		tasks: make(map[string]*Task),
	}
}

// Create adds a new task to the store and returns it.
func (s *Store) Create(desc, typ string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	id := fmt.Sprintf("task_%d", s.counter)

	if typ == "" {
		typ = "local_bash"
	}

	t := &Task{
		ID:          id,
		Description: desc,
		Type:        typ,
		Status:      "pending",
		OutputCh:    make(chan string, 100),
	}
	s.tasks[id] = t
	return t
}

// CreateWithCancel adds a new task with a cancel function and returns it.
func (s *Store) CreateWithCancel(desc, typ string, cancel context.CancelFunc) *Task {
	t := s.Create(desc, typ)
	s.mu.Lock()
	defer s.mu.Unlock()
	t.CancelFunc = cancel
	return t
}

// Get returns a task by ID, or nil and false if not found.
func (s *Store) Get(id string) (*Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	return t, ok
}

// Update modifies a task's status and/or output message.
func (s *Store) Update(id, status, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	if status != "" {
		t.Status = status
	}
	if message != "" {
		t.Output = message
	}
	return nil
}

// List returns all tasks, optionally filtered by status.
// If statusFilter is empty, all tasks are returned.
func (s *Store) List(statusFilter string) []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if statusFilter == "" || t.Status == statusFilter {
			result = append(result, t)
		}
	}
	return result
}

// Stop marks a task as stopped. Returns an error if the task is not found.
func (s *Store) Stop(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	t.Status = "stopped"
	return nil
}

// Cancel calls the task's CancelFunc (if set) and sets status to "stopped".
func (s *Store) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	if t.CancelFunc != nil {
		t.CancelFunc()
	}
	t.Status = "stopped"
	return nil
}

// GetOutput returns the output of a task by ID.
func (s *Store) GetOutput(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return "", fmt.Errorf("task %q not found", id)
	}
	return t.Output, nil
}

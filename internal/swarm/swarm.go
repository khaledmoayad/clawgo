package swarm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
)

// Manager coordinates teams of worker agents, routing messages and collecting
// task notifications. It is the central orchestration point for the swarm system.
type Manager struct {
	mu          sync.RWMutex
	teams       map[string]*Team
	workers     map[string]*Worker
	taskStore   *tasks.Store
	client      *api.Client
	registry    *tools.Registry
	permCtx     *permissions.PermissionContext
	notifyCh    chan TaskNotification // Delivers notifications back to the coordinator
	workingDir  string
	projectRoot string
	sessionID   string
}

// NewManager creates a swarm manager wired to the given task store, API client,
// tool registry, and permission context.
func NewManager(
	store *tasks.Store,
	client *api.Client,
	registry *tools.Registry,
	permCtx *permissions.PermissionContext,
	workingDir, projectRoot, sessionID string,
) *Manager {
	return &Manager{
		teams:       make(map[string]*Team),
		workers:     make(map[string]*Worker),
		taskStore:   store,
		client:      client,
		registry:    registry,
		permCtx:     permCtx,
		notifyCh:    make(chan TaskNotification, 64),
		workingDir:  workingDir,
		projectRoot: projectRoot,
		sessionID:   sessionID,
	}
}

// CreateTeam creates a new named team. If a team with the same name already
// exists, it returns the existing team.
func (m *Manager) CreateTeam(name string) *Team {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.teams[name]; ok {
		return t
	}

	t := &Team{
		Name:      name,
		Workers:   make(map[string]*Worker),
		CreatedAt: time.Now(),
	}
	m.teams[name] = t
	return t
}

// DeleteTeam cancels all workers in the team and removes it from the manager.
func (m *Manager) DeleteTeam(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.teams[name]
	if !ok {
		return fmt.Errorf("team %q not found", name)
	}

	// Cancel all workers in this team
	for _, w := range t.Workers {
		if w.Cancel != nil && w.Status == WorkerRunning {
			w.Cancel()
			w.Status = WorkerStopped
		}
		// Remove from the global workers map
		delete(m.workers, w.ID)
	}

	delete(m.teams, name)
	return nil
}

// CurrentTeam returns the name of the current (most recently created) team,
// or empty string if no teams exist. In the TypeScript version this comes
// from appState.teamContext.teamName; here we track the last-created team.
func (m *Manager) CurrentTeam() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return the most recently created team
	var latest *Team
	for _, t := range m.teams {
		if latest == nil || t.CreatedAt.After(latest.CreatedAt) {
			latest = t
		}
	}
	if latest != nil {
		return latest.Name
	}
	return ""
}

// GetTeam returns a team by name.
func (m *Manager) GetTeam(name string) (*Team, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.teams[name]
	return t, ok
}

// SpawnWorker creates a new worker agent goroutine in the specified team.
// The worker runs a query loop with the given prompt and sends a TaskNotification
// to notifyCh upon completion.
//
// agentName is the human-readable name for the worker (e.g., "researcher", "tester").
// The worker ID is deterministic: agentName@teamName (matching TS agentId.ts format).
// If agentName is empty, a random hex name is generated as a fallback.
func (m *Manager) SpawnWorker(ctx context.Context, teamName, agentName, description, prompt string) (*Worker, error) {
	m.mu.Lock()

	team, ok := m.teams[teamName]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("team %q not found", teamName)
	}

	// Sanitize agent name and generate deterministic ID
	if agentName == "" {
		agentName = generateRandomName()
	} else {
		agentName = SanitizeAgentName(agentName)
	}
	workerID := FormatAgentID(agentName, teamName)

	// Check for duplicate worker ID (same name in same team)
	if _, exists := m.workers[workerID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("worker %q already exists in team %q", agentName, teamName)
	}

	// Create cancellable context for the worker
	workerCtx, cancel := context.WithCancel(ctx)

	// Register task in the task store
	task := m.taskStore.CreateWithCancel(description, "local_agent", cancel)

	w := &Worker{
		ID:          workerID,
		AgentName:   agentName,
		TeamName:    teamName,
		Description: description,
		TaskID:      task.ID,
		InputCh:     make(chan string, 10),
		Cancel:      cancel,
		Status:      WorkerRunning,
		CreatedAt:   time.Now(),
	}

	// Register worker
	m.workers[workerID] = w
	team.Workers[workerID] = w

	m.mu.Unlock()

	// Update task store status
	_ = m.taskStore.Update(task.ID, "running", "")

	// Launch worker goroutine
	go m.runWorker(workerCtx, w, prompt, task)

	return w, nil
}

// runWorker executes the worker's query loop and sends a TaskNotification
// upon completion. It also monitors InputCh for follow-up messages.
func (m *Manager) runWorker(ctx context.Context, w *Worker, prompt string, task *tasks.Task) {
	startTime := time.Now()
	var output strings.Builder

	// Deferred panic recovery
	defer func() {
		if r := recover(); r != nil {
			m.mu.Lock()
			w.Status = WorkerFailed
			m.mu.Unlock()
			_ = m.taskStore.Update(task.ID, "failed", fmt.Sprintf("panic: %v", r))

			m.sendNotification(w, "failed", fmt.Sprintf("panic: %v", r), output.String(), startTime)
		}
	}()

	textCallback := func(text string) {
		output.WriteString(text)
		// Non-blocking send to task output channel
		select {
		case task.OutputCh <- text:
		default:
		}
	}

	// Build system prompt with teammate addendum for proper team communication
	defaultParts := []string{
		fmt.Sprintf("You are a worker agent (ID: %s). Complete the assigned task using available tools. Be focused and efficient.", w.ID),
	}
	systemPrompt := BuildTeammateSystemPrompt(SystemPromptDefault, "", defaultParts, "")

	params := &query.LoopParams{
		Client:      m.client,
		Registry:    m.registry,
		PermCtx:     m.permCtx,
		CostTracker: cost.NewTracker(m.client.Model),
		Messages: []api.Message{
			api.UserMessage(prompt),
		},
		SystemPrompt: systemPrompt,
		MaxTurns:     30,
		WorkingDir:   m.workingDir,
		ProjectRoot:  m.projectRoot,
		SessionID:    m.sessionID,
		TextCallback: textCallback,
	}

	err := query.RunLoop(ctx, params)

	// Check for follow-up messages after RunLoop completes
	for {
		select {
		case msg := <-w.InputCh:
			// Received a follow-up message -- add it and run another loop
			output.Reset()
			params.Messages = append(params.Messages, api.UserMessage(msg))
			err = query.RunLoop(ctx, params)
			if err != nil {
				break
			}
			continue
		default:
			// No more messages -- exit
		}
		break
	}

	// Update final state
	m.mu.Lock()
	if err != nil {
		w.Status = WorkerFailed
		m.mu.Unlock()
		_ = m.taskStore.Update(task.ID, "failed", fmt.Sprintf("worker error: %s", err.Error()))
		m.sendNotification(w, "failed", fmt.Sprintf("failed: %s", err.Error()), output.String(), startTime)
		return
	}

	w.Status = WorkerCompleted
	m.mu.Unlock()

	result := output.String()
	if result == "" {
		result = "(worker produced no output)"
	}
	_ = m.taskStore.Update(task.ID, "completed", result)
	m.sendNotification(w, "completed", fmt.Sprintf("Agent %q completed", w.Description), result, startTime)
}

// sendNotification builds and sends a TaskNotification to the coordinator channel.
func (m *Manager) sendNotification(w *Worker, status, summary, result string, startTime time.Time) {
	notif := TaskNotification{
		TaskID:     w.ID,
		Status:     status,
		Summary:    summary,
		Result:     result,
		DurationMs: time.Since(startTime).Milliseconds(),
	}

	// Non-blocking send to prevent deadlock if coordinator isn't reading
	select {
	case m.notifyCh <- notif:
	default:
	}
}

// SendMessage delivers a follow-up message to a running worker via its InputCh.
func (m *Manager) SendMessage(workerID, message string) error {
	m.mu.RLock()
	w, ok := m.workers[workerID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("worker %q not found", workerID)
	}

	// Non-blocking send with error if channel full
	select {
	case w.InputCh <- message:
		return nil
	default:
		return fmt.Errorf("worker %q message buffer full", workerID)
	}
}

// StopWorker cancels the worker's context and updates its status.
func (m *Manager) StopWorker(workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if w.Cancel != nil {
		w.Cancel()
	}
	w.Status = WorkerStopped

	// Also update the task store
	_ = m.taskStore.Cancel(w.TaskID)
	return nil
}

// GetNotifications returns the channel that delivers task notifications
// to the coordinator. The coordinator reads from this channel to inject
// notifications as user-role messages in the conversation.
func (m *Manager) GetNotifications() <-chan TaskNotification {
	return m.notifyCh
}

// GetWorker returns a worker by ID.
func (m *Manager) GetWorker(id string) (*Worker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[id]
	return w, ok
}

// Close cancels all workers and closes the notification channel.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.workers {
		if w.Cancel != nil && w.Status == WorkerRunning {
			w.Cancel()
			w.Status = WorkerStopped
		}
	}

	close(m.notifyCh)
}

// generateRandomName creates a random agent name as a fallback when no
// explicit name is provided. Format: agent-{random6hex}.
func generateRandomName() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "agent-" + hex.EncodeToString(b)
}

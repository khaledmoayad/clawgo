// Package swarm implements leader-worker agent coordination for multi-agent
// orchestration. It provides team management, worker spawning via goroutines,
// message routing, and task notifications matching the TypeScript swarm system.
package swarm

import (
	"context"
	"fmt"
	"time"
)

// Worker status constants.
const (
	WorkerRunning   = "running"
	WorkerCompleted = "completed"
	WorkerFailed    = "failed"
	WorkerStopped   = "stopped"
)

// Worker represents a spawned agent goroutine executing a task.
type Worker struct {
	ID          string             // Unique deterministic ID (format: agentName@teamName)
	AgentName   string             // Agent name component of the ID
	TeamName    string             // Team name component of the ID
	Description string             // Human-readable description of the worker's task
	TaskID      string             // References tasks.Store task ID for lifecycle tracking
	InputCh     chan string         // Buffered channel for follow-up messages from SendMessage
	Cancel      context.CancelFunc // Cancels the worker's context
	Status      string             // "running", "completed", "failed", "stopped"
	CreatedAt   time.Time
}

// Team groups related workers under a named team.
type Team struct {
	Name      string             // Team name
	LeaderID  string             // Agent ID of the team leader
	Workers   map[string]*Worker // Worker ID -> Worker
	CreatedAt time.Time
}

// TaskNotification is delivered back to the coordinator when a worker completes,
// fails, or is stopped. The coordinator injects these as user-role messages
// so Claude sees them naturally in the conversation.
type TaskNotification struct {
	TaskID      string // Worker/agent ID
	Status      string // "completed", "failed", "killed"
	Summary     string // Human-readable outcome summary
	Result      string // Worker's final text response (may be empty)
	TotalTokens int    // Total tokens consumed
	ToolUses    int    // Number of tool invocations
	DurationMs  int64  // Wall-clock duration in milliseconds
}

// ToXML generates the <task-notification> XML block matching the TypeScript
// coordinator format. The coordinator system prompt instructs Claude to parse
// this format from user-role messages.
func (n *TaskNotification) ToXML() string {
	xml := "<task-notification>\n"
	xml += fmt.Sprintf("<task-id>%s</task-id>\n", n.TaskID)
	xml += fmt.Sprintf("<status>%s</status>\n", n.Status)
	xml += fmt.Sprintf("<summary>%s</summary>\n", n.Summary)
	if n.Result != "" {
		xml += fmt.Sprintf("<result>%s</result>\n", n.Result)
	}
	xml += "<usage>\n"
	xml += fmt.Sprintf("  <total_tokens>%d</total_tokens>\n", n.TotalTokens)
	xml += fmt.Sprintf("  <tool_uses>%d</tool_uses>\n", n.ToolUses)
	xml += fmt.Sprintf("  <duration_ms>%d</duration_ms>\n", n.DurationMs)
	xml += "</usage>\n"
	xml += "</task-notification>"
	return xml
}

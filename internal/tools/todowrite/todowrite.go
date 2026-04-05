// Package todowrite implements the TodoWriteTool for creating and managing task lists.
// Each call REPLACES the full todo list (not merge by ID), matching Claude Code behavior.
// Todos are persisted to .claude/todos.json in the project root.
package todowrite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// TodoItem represents a single todo entry matching Claude Code's schema.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`     // "pending", "in_progress", "completed"
	ActiveForm string `json:"activeForm"` // The currently active phrasing of this task
}

type input struct {
	Todos []TodoItem `json:"todos"`
}

// TodoWriteTool creates and updates todo/task files.
type TodoWriteTool struct{}

// New creates a new TodoWriteTool.
func New() *TodoWriteTool { return &TodoWriteTool{} }

func (t *TodoWriteTool) Name() string                { return "TodoWrite" }
func (t *TodoWriteTool) Description() string          { return toolDescription }
func (t *TodoWriteTool) IsReadOnly() bool             { return false }
func (t *TodoWriteTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because it writes to a shared todo file.
func (t *TodoWriteTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *TodoWriteTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TodoWrite", false, permCtx), nil
}

func (t *TodoWriteTool) Call(_ context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// Validate todo items
	for i, todo := range in.Todos {
		if strings.TrimSpace(todo.Content) == "" {
			return tools.ErrorResult(fmt.Sprintf("todo[%d]: content is required", i)), nil
		}
		if todo.Status != "pending" && todo.Status != "in_progress" && todo.Status != "completed" {
			return tools.ErrorResult(fmt.Sprintf("todo[%d]: invalid status %q (must be pending, in_progress, or completed)", i, todo.Status)), nil
		}
		if strings.TrimSpace(todo.ActiveForm) == "" {
			return tools.ErrorResult(fmt.Sprintf("todo[%d]: activeForm is required", i)), nil
		}
	}

	// Determine file path
	projectRoot := toolCtx.ProjectRoot
	if projectRoot == "" {
		projectRoot = toolCtx.WorkingDir
	}
	todosPath := filepath.Join(projectRoot, ".claude", "todos.json")

	// Read existing todos for the "old" state
	var oldTodos []TodoItem
	data, err := os.ReadFile(todosPath)
	if err == nil {
		_ = json.Unmarshal(data, &oldTodos) // ignore error, start fresh if corrupt
	}

	// Full replacement: the new list IS the input (not merged by ID)
	newTodos := in.Todos

	// If all completed, clear the list (matching Claude Code behavior)
	allCompleted := len(newTodos) > 0
	for _, todo := range newTodos {
		if todo.Status != "completed" {
			allCompleted = false
			break
		}
	}
	if allCompleted {
		newTodos = []TodoItem{}
	}

	// Create parent directory if needed
	dir := filepath.Dir(todosPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create directory %s: %s", dir, err.Error())), nil
	}

	// Write todos file
	jsonData, err := json.MarshalIndent(newTodos, "", "  ")
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to marshal todos: %s", err.Error())), nil
	}
	if err := os.WriteFile(todosPath, jsonData, 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write todos file: %s", err.Error())), nil
	}

	return tools.TextResult(fmt.Sprintf(
		"Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable",
	)), nil
}

// Package todowrite implements the TodoWriteTool for creating and managing task lists.
// Todos are persisted to .claude/todos.json in the project root, matching the
// TypeScript TodoWrite tool behavior.
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

// TodoItem represents a single todo entry.
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`   // "pending", "in_progress", "done"
	Priority string `json:"priority"` // "high", "medium", "low"
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
	if len(in.Todos) == 0 {
		return tools.ErrorResult("required field \"todos\" is missing or empty"), nil
	}

	// Validate todo items
	for i, todo := range in.Todos {
		if strings.TrimSpace(todo.ID) == "" {
			return tools.ErrorResult(fmt.Sprintf("todo[%d]: id is required", i)), nil
		}
		if strings.TrimSpace(todo.Content) == "" {
			return tools.ErrorResult(fmt.Sprintf("todo[%d]: content is required", i)), nil
		}
		if todo.Status != "pending" && todo.Status != "in_progress" && todo.Status != "done" {
			return tools.ErrorResult(fmt.Sprintf("todo[%d]: invalid status %q (must be pending, in_progress, or done)", i, todo.Status)), nil
		}
	}

	// Determine file path
	projectRoot := toolCtx.ProjectRoot
	if projectRoot == "" {
		projectRoot = toolCtx.WorkingDir
	}
	todosPath := filepath.Join(projectRoot, ".claude", "todos.json")

	// Read existing todos
	existing := make(map[string]TodoItem)
	data, err := os.ReadFile(todosPath)
	if err == nil {
		var existingList []TodoItem
		if jsonErr := json.Unmarshal(data, &existingList); jsonErr == nil {
			for _, item := range existingList {
				existing[item.ID] = item
			}
		}
	}

	// Merge: update existing by id, add new
	var added, updated int
	for _, todo := range in.Todos {
		if _, exists := existing[todo.ID]; exists {
			updated++
		} else {
			added++
		}
		// Set default priority if not specified
		if todo.Priority == "" {
			todo.Priority = "medium"
		}
		existing[todo.ID] = todo
	}

	// Convert map back to sorted list (by id for deterministic output)
	todoList := make([]TodoItem, 0, len(existing))
	for _, item := range existing {
		todoList = append(todoList, item)
	}

	// Create parent directory if needed
	dir := filepath.Dir(todosPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create directory %s: %s", dir, err.Error())), nil
	}

	// Write todos file
	jsonData, err := json.MarshalIndent(todoList, "", "  ")
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to marshal todos: %s", err.Error())), nil
	}
	if err := os.WriteFile(todosPath, jsonData, 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write todos file: %s", err.Error())), nil
	}

	return tools.TextResult(fmt.Sprintf("Updated todos: %d added, %d updated, %d total", added, updated, len(todoList))), nil
}

// Package skill implements the SkillTool for loading skill files.
// Skills provide specialized knowledge and instructions for specific tasks,
// matching the TypeScript Skill tool behavior.
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/skills"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Name string `json:"name"`
}

// SkillTool loads skill files from the skills directories.
type SkillTool struct{}

// New creates a new SkillTool.
func New() *SkillTool { return &SkillTool{} }

func (t *SkillTool) Name() string                { return "Skill" }
func (t *SkillTool) Description() string          { return toolDescription }
func (t *SkillTool) IsReadOnly() bool             { return true }
func (t *SkillTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true -- reading skill files is safe.
func (t *SkillTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions returns Allow -- loading skills is always permitted.
func (t *SkillTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *SkillTool) Call(_ context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Name) == "" {
		return tools.ErrorResult("required field \"name\" is missing or empty"), nil
	}

	// Determine config directory
	projectRoot := toolCtx.ProjectRoot
	if projectRoot == "" {
		projectRoot = toolCtx.WorkingDir
	}

	configDir, err := os.UserHomeDir()
	if err != nil {
		configDir = projectRoot
	} else {
		configDir = configDir + "/.claude"
	}

	// Use the skills package to load all skills
	allSkills, err := skills.LoadSkills(projectRoot, configDir)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("error loading skills: %s", err.Error())), nil
	}

	// Find the requested skill by name
	for _, s := range allSkills {
		if s.Name == in.Name {
			result := tools.TextResult(s.Content)
			// If frontmatter has AllowedTools, include in metadata
			if s.Frontmatter != nil && len(s.Frontmatter.AllowedTools) > 0 {
				result.Metadata = map[string]any{
					"allowed_tools": s.Frontmatter.AllowedTools,
				}
			}
			return result, nil
		}
	}

	// Skill not found -- list available skills
	names := skills.SkillNames(allSkills)
	if len(names) > 0 {
		return tools.TextResult(fmt.Sprintf("Skill %q not found. Available skills: %s", in.Name, strings.Join(names, ", "))), nil
	}
	return tools.TextResult(fmt.Sprintf("Skill %q not found. No skills are available in the search paths.", in.Name)), nil
}

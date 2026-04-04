// Package configtool implements the ConfigTool for reading and modifying
// project configuration settings. Named "configtool" to avoid conflict
// with the config package, matching the TypeScript Config tool behavior.
package configtool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Setting string          `json:"setting"`
	Value   json.RawMessage `json:"value,omitempty"`
}

// ConfigTool reads and modifies project configuration.
type ConfigTool struct{}

// New creates a new ConfigTool.
func New() *ConfigTool { return &ConfigTool{} }

func (t *ConfigTool) Name() string                { return "Config" }
func (t *ConfigTool) Description() string          { return toolDescription }
func (t *ConfigTool) IsReadOnly() bool             { return false }
func (t *ConfigTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because "set" action modifies config files.
func (t *ConfigTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions returns Ask for "set" actions (when value is present), Allow for "get".
func (t *ConfigTool) CheckPermissions(_ context.Context, inp json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	var in input
	if err := json.Unmarshal(inp, &in); err != nil {
		return permissions.Ask, nil
	}
	// If value is present, it's a set operation -- requires permission
	if len(in.Value) > 0 && string(in.Value) != "null" {
		return permissions.CheckPermission("Config", false, permCtx), nil
	}
	return permissions.Allow, nil
}

func (t *ConfigTool) Call(_ context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// Determine config file path
	projectRoot := toolCtx.ProjectRoot
	if projectRoot == "" {
		projectRoot = toolCtx.WorkingDir
	}
	configPath := filepath.Join(projectRoot, ".claude", "settings.json")

	// Read existing config
	config := make(map[string]any)
	data, err := os.ReadFile(configPath)
	if err == nil {
		if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
			return tools.ErrorResult(fmt.Sprintf("failed to parse config: %s", jsonErr.Error())), nil
		}
	}

	// If no setting provided, list all settings
	if strings.TrimSpace(in.Setting) == "" {
		if len(config) == 0 {
			return tools.TextResult("No configuration values set."), nil
		}
		keys := make([]string, 0, len(config))
		for k := range config {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		for _, k := range keys {
			v, _ := json.Marshal(config[k])
			sb.WriteString(fmt.Sprintf("%s = %s\n", k, string(v)))
		}
		return tools.TextResult(sb.String()), nil
	}

	// Infer GET vs SET from whether value is present
	isSet := len(in.Value) > 0 && string(in.Value) != "null"

	if !isSet {
		// GET operation
		val, ok := config[in.Setting]
		if !ok {
			return tools.TextResult(fmt.Sprintf("Key %q is not set.", in.Setting)), nil
		}
		v, _ := json.Marshal(val)
		return tools.TextResult(fmt.Sprintf("%s = %s", in.Setting, string(v))), nil
	}

	// SET operation
	var parsedValue any
	if err := json.Unmarshal(in.Value, &parsedValue); err != nil {
		// Fall back to treating it as a raw string
		parsedValue = string(in.Value)
	}
	config[in.Setting] = parsedValue

	// Create parent directory if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create directory: %s", err.Error())), nil
	}

	// Write config
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to marshal config: %s", err.Error())), nil
	}
	if err := os.WriteFile(configPath, jsonData, 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write config: %s", err.Error())), nil
	}

	return tools.TextResult(fmt.Sprintf("Set %s = %s", in.Setting, string(in.Value))), nil
}

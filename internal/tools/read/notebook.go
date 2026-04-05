package read

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/tools"
)

// notebookJSON represents the top-level structure of a Jupyter notebook file.
type notebookJSON struct {
	Cells    []cellJSON      `json:"cells"`
	Metadata notebookMetaJSON `json:"metadata"`
	NBFormat int              `json:"nbformat"`
}

// notebookMetaJSON represents notebook-level metadata.
type notebookMetaJSON struct {
	LanguageInfo *languageInfoJSON `json:"language_info,omitempty"`
}

// languageInfoJSON holds the notebook's language info.
type languageInfoJSON struct {
	Name string `json:"name"`
}

// cellJSON represents a single cell in a Jupyter notebook.
type cellJSON struct {
	ID             string            `json:"id,omitempty"`
	CellType       string            `json:"cell_type"`
	Source         json.RawMessage   `json:"source"`
	Outputs        []cellOutputJSON  `json:"outputs,omitempty"`
	ExecutionCount *int              `json:"execution_count,omitempty"`
}

// cellOutputJSON represents a cell output entry.
type cellOutputJSON struct {
	OutputType string                 `json:"output_type"`
	Text       json.RawMessage        `json:"text,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	EName      string                 `json:"ename,omitempty"`
	EValue     string                 `json:"evalue,omitempty"`
	Traceback  []string               `json:"traceback,omitempty"`
}

// isNotebookFile checks whether the given path has a .ipynb extension.
func isNotebookFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".ipynb"
}

// parseSource extracts the source text from a cell's source field.
// The source field can be either a string or an array of strings.
func parseSource(raw json.RawMessage) string {
	// Try as a single string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try as array of strings
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}

	return string(raw)
}

// parseOutputText extracts text from an output's text field.
// The text field can be either a string or an array of strings.
func parseOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try as a single string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try as array of strings
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}

	return ""
}

// extractOutputText returns the text representation of a cell output.
func extractOutputText(output cellOutputJSON) string {
	switch output.OutputType {
	case "stream":
		return parseOutputText(output.Text)
	case "execute_result", "display_data":
		if output.Data != nil {
			if textPlain, ok := output.Data["text/plain"]; ok {
				switch v := textPlain.(type) {
				case string:
					return v
				case []interface{}:
					var parts []string
					for _, item := range v {
						if s, ok := item.(string); ok {
							parts = append(parts, s)
						}
					}
					return strings.Join(parts, "")
				}
			}
		}
		return ""
	case "error":
		var parts []string
		if output.EName != "" || output.EValue != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", output.EName, output.EValue))
		}
		if len(output.Traceback) > 0 {
			parts = append(parts, strings.Join(output.Traceback, "\n"))
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

// readNotebookFile reads and parses a Jupyter notebook file, returning formatted cell content.
func readNotebookFile(path string) (*tools.ToolResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read notebook file: %w", err)
	}

	var nb notebookJSON
	if err := json.Unmarshal(data, &nb); err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to parse notebook JSON: %s", err.Error())), nil
	}

	language := "python"
	if nb.Metadata.LanguageInfo != nil && nb.Metadata.LanguageInfo.Name != "" {
		language = nb.Metadata.LanguageInfo.Name
	}

	var out strings.Builder
	for i, cell := range nb.Cells {
		cellID := cell.ID
		if cellID == "" {
			cellID = fmt.Sprintf("cell-%d", i)
		}

		source := parseSource(cell.Source)

		// Format: Cell [N] (type):
		fmt.Fprintf(&out, "Cell [%d] (%s", i+1, cell.CellType)
		if cell.CellType == "code" {
			fmt.Fprintf(&out, ", %s", language)
		}
		out.WriteString("):\n")

		// Source in code block
		out.WriteString("```\n")
		out.WriteString(source)
		if !strings.HasSuffix(source, "\n") {
			out.WriteString("\n")
		}
		out.WriteString("```\n")

		// Outputs for code cells
		if cell.CellType == "code" && len(cell.Outputs) > 0 {
			out.WriteString("Output:\n")
			for _, output := range cell.Outputs {
				text := extractOutputText(output)
				if text != "" {
					out.WriteString(text)
					if !strings.HasSuffix(text, "\n") {
						out.WriteString("\n")
					}
				}
			}
		}

		out.WriteString("\n")
	}

	return tools.TextResult(out.String()), nil
}

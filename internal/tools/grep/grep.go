package grep

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const defaultMaxResults = 250

// GrepTool searches file contents using ripgrep with a Go-native fallback.
type GrepTool struct{}

// New creates a new GrepTool instance.
func New() *GrepTool {
	return &GrepTool{}
}

// Name returns the tool name.
func (t *GrepTool) Name() string { return "Grep" }

// Description returns the tool description.
func (t *GrepTool) Description() string { return toolDescription }

// InputSchema returns the JSON schema for tool input.
func (t *GrepTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsReadOnly returns true because grep only reads files.
func (t *GrepTool) IsReadOnly() bool { return true }

// IsConcurrencySafe returns true because grep only reads files, safe for concurrent execution.
func (t *GrepTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// Call executes the grep tool.
// It tries ripgrep first, falling back to a Go-native implementation.
func (t *GrepTool) Call(ctx context.Context, input json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	data, err := tools.ParseRawInput(input)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	pattern, err := tools.RequireString(data, "pattern")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	searchPath := tools.OptionalString(data, "path", toolCtx.WorkingDir)
	include := tools.OptionalString(data, "include", "")
	contextLines := tools.OptionalInt(data, "context", 0)
	maxResults := tools.OptionalInt(data, "max_results", defaultMaxResults)

	// Make path absolute if needed
	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(toolCtx.WorkingDir, searchPath)
	}

	// Check if ripgrep is available
	rgPath, rgErr := exec.LookPath("rg")
	if rgErr == nil && rgPath != "" {
		output, err := t.runRipgrep(ctx, pattern, searchPath, include, contextLines, maxResults)
		if err != nil {
			// rg was found but returned an error (likely invalid regex)
			return tools.ErrorResult(fmt.Sprintf("grep error: %s", err)), nil
		}

		output = strings.TrimSpace(output)
		if output == "" {
			return tools.TextResult("No matches found"), nil
		}

		return tools.TextResult(output), nil
	}

	// Fall back to Go-native grep when rg is not installed
	return t.nativeGrep(ctx, pattern, searchPath, include, contextLines, maxResults)
}

// runRipgrep executes ripgrep with the given parameters.
func (t *GrepTool) runRipgrep(ctx context.Context, pattern, searchPath, include string, contextLines, maxResults int) (string, error) {
	args := []string{
		"-n",           // line numbers
		"--no-heading", // consistent output format
		"--color=never",
	}

	if include != "" {
		args = append(args, "--glob", include)
	}

	if contextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", contextLines))
	}

	if maxResults > 0 {
		args = append(args, fmt.Sprintf("-m%d", maxResults))
	}

	args = append(args, pattern, searchPath)

	cmd := exec.CommandContext(ctx, "rg", args...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		// Exit code 1 means no matches (not an error for us)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		// Exit code 2 means pattern error or other issue
		return "", fmt.Errorf("%s\n%s", err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

// nativeGrep is a Go-native fallback when ripgrep is not installed.
func (t *GrepTool) nativeGrep(ctx context.Context, pattern, searchPath, include string, contextLines, maxResults int) (*tools.ToolResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid regex pattern: %s", err)), nil
	}

	var results []string
	matchCount := 0

	walkErr := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't access
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply include filter if specified
		if include != "" {
			matched, _ := filepath.Match(include, info.Name())
			if !matched {
				return nil
			}
		}

		// Skip binary files (simple heuristic: skip files > 1MB or with null bytes)
		if info.Size() > 1<<20 {
			return nil
		}

		matches := t.searchFile(path, re, contextLines)
		for _, m := range matches {
			if maxResults > 0 && matchCount >= maxResults {
				return filepath.SkipAll
			}

			relPath, relErr := filepath.Rel(searchPath, path)
			if relErr != nil {
				relPath = path
			}
			results = append(results, fmt.Sprintf("%s:%s", relPath, m))
			matchCount++
		}

		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
		return tools.ErrorResult(fmt.Sprintf("search error: %s", walkErr)), nil
	}

	if len(results) == 0 {
		return tools.TextResult("No matches found"), nil
	}

	return tools.TextResult(strings.Join(results, "\n")), nil
}

// searchFile searches a single file for the pattern and returns matching lines with context.
func (t *GrepTool) searchFile(path string, re *regexp.Regexp, contextLines int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil
	}

	var results []string
	for i, line := range allLines {
		if re.MatchString(line) {
			if contextLines > 0 {
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines + 1
				if end > len(allLines) {
					end = len(allLines)
				}
				for j := start; j < end; j++ {
					sep := "-"
					if j == i {
						sep = ":"
					}
					results = append(results, fmt.Sprintf("%d%s%s", j+1, sep, allLines[j]))
				}
				// Add separator between match groups
				results = append(results, "--")
			} else {
				results = append(results, fmt.Sprintf("%d:%s", i+1, line))
			}
		}
	}

	return results
}

// CheckPermissions determines whether the tool should be allowed, denied, or require user prompt.
func (t *GrepTool) CheckPermissions(ctx context.Context, input json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission(t.Name(), t.IsReadOnly(), permCtx), nil
}

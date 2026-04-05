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
	"sort"
	"strconv"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const defaultHeadLimit = 250

// vcsDirectoriesToExclude mirrors the VCS_DIRECTORIES_TO_EXCLUDE list from
// Claude Code's GrepTool. These are always excluded via --glob negation.
var vcsDirectoriesToExclude = []string{
	".git",
	".svn",
	".hg",
	".bzr",
	".jj",
	".sl",
}

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
	glob := tools.OptionalString(data, "glob", "")
	fileType := tools.OptionalString(data, "type", "")
	outputMode := tools.OptionalString(data, "output_mode", "files_with_matches")
	contextC := tools.OptionalInt(data, "context", 0)
	afterA := tools.OptionalInt(data, "-A", 0)
	beforeB := tools.OptionalInt(data, "-B", 0)
	aliasC := tools.OptionalInt(data, "-C", 0)
	headLimit := tools.OptionalInt(data, "head_limit", defaultHeadLimit)
	offset := tools.OptionalInt(data, "offset", 0)

	// Boolean parameters use semantic coercion (string "true"/"false" -> bool)
	lineNumsN, err := tools.OptionalSemanticBool(data, "-n", true)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	caseInsI, err := tools.OptionalSemanticBool(data, "-i", false)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	multiline, err := tools.OptionalSemanticBool(data, "multiline", false)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// Resolve context: `context` takes precedence over `-C` alias
	effectiveContext := contextC
	if effectiveContext == 0 && aliasC > 0 {
		effectiveContext = aliasC
	}

	// Make path absolute if needed
	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(toolCtx.WorkingDir, searchPath)
	}

	params := &grepParams{
		pattern:          pattern,
		searchPath:       searchPath,
		glob:             glob,
		fileType:         fileType,
		outputMode:       outputMode,
		effectiveContext: effectiveContext,
		afterA:           afterA,
		beforeB:          beforeB,
		lineNumsN:        lineNumsN,
		caseInsI:         caseInsI,
		multiline:        multiline,
		headLimit:        headLimit,
		offset:           offset,
	}

	// Check if ripgrep is available
	rgPath, rgErr := exec.LookPath("rg")
	if rgErr == nil && rgPath != "" {
		return t.runRipgrep(ctx, params)
	}

	// Fall back to Go-native grep when rg is not installed
	return t.nativeGrep(ctx, params)
}

// grepParams holds all parsed parameters for a grep operation.
type grepParams struct {
	pattern          string
	searchPath       string
	glob             string
	fileType         string
	outputMode       string // "content", "files_with_matches", "count"
	effectiveContext int
	afterA           int
	beforeB          int
	lineNumsN        bool
	caseInsI         bool
	multiline        bool
	headLimit        int
	offset           int
}

// runRipgrep executes ripgrep with all 14 parameters mapped to rg flags.
func (t *GrepTool) runRipgrep(ctx context.Context, p *grepParams) (*tools.ToolResult, error) {
	args := []string{
		"--hidden",      // search hidden files
		"--color=never", // no ANSI colors
	}

	// Exclude VCS directories
	for _, dir := range vcsDirectoriesToExclude {
		args = append(args, "--glob", "!"+dir)
	}

	// Limit line length to prevent base64/minified content from cluttering output
	args = append(args, "--max-columns", "500")

	// Multiline mode
	if p.multiline {
		args = append(args, "-U", "--multiline-dotall")
	}

	// Case insensitive
	if p.caseInsI {
		args = append(args, "-i")
	}

	// Output mode flags
	switch p.outputMode {
	case "files_with_matches":
		args = append(args, "-l")
	case "count":
		args = append(args, "-c")
	case "content":
		// Line numbers in content mode
		if p.lineNumsN {
			args = append(args, "-n")
		} else {
			args = append(args, "--no-line-number")
		}

		// Context flags: context takes precedence over -B/-A individually
		if p.effectiveContext > 0 {
			args = append(args, fmt.Sprintf("-C%d", p.effectiveContext))
		} else {
			if p.beforeB > 0 {
				args = append(args, fmt.Sprintf("-B%d", p.beforeB))
			}
			if p.afterA > 0 {
				args = append(args, fmt.Sprintf("-A%d", p.afterA))
			}
		}

		// Consistent output format for content mode
		args = append(args, "--no-heading")
	}

	// Pattern -- if it starts with dash, use -e to prevent misinterpretation
	if strings.HasPrefix(p.pattern, "-") {
		args = append(args, "-e", p.pattern)
	} else {
		args = append(args, p.pattern)
	}

	// File type filter
	if p.fileType != "" {
		args = append(args, "--type", p.fileType)
	}

	// Glob filter -- split on whitespace, preserving brace patterns
	if p.glob != "" {
		globPatterns := splitGlobPatterns(p.glob)
		for _, gp := range globPatterns {
			args = append(args, "--glob", gp)
		}
	}

	args = append(args, p.searchPath)

	cmd := exec.CommandContext(ctx, "rg", args...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		// Exit code 1 means no matches (not an error for us)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return t.formatNoMatches(p.outputMode), nil
		}
		// Exit code 2 means pattern error or other issue
		return tools.ErrorResult(fmt.Sprintf("grep error: %s\n%s", err, strings.TrimSpace(string(out)))), nil
	}

	rawOutput := strings.TrimRight(string(out), "\n")
	if rawOutput == "" {
		return t.formatNoMatches(p.outputMode), nil
	}

	lines := strings.Split(rawOutput, "\n")

	return t.formatOutput(lines, p)
}

// splitGlobPatterns splits a glob string on whitespace, preserving brace patterns.
// Patterns with braces are kept intact; patterns without braces are further split on commas.
func splitGlobPatterns(glob string) []string {
	var patterns []string
	rawPatterns := strings.Fields(glob)
	for _, raw := range rawPatterns {
		if strings.Contains(raw, "{") && strings.Contains(raw, "}") {
			patterns = append(patterns, raw)
		} else {
			parts := strings.Split(raw, ",")
			for _, part := range parts {
				if part != "" {
					patterns = append(patterns, part)
				}
			}
		}
	}
	return patterns
}

// formatNoMatches returns the appropriate "no matches" message per output mode.
func (t *GrepTool) formatNoMatches(outputMode string) *tools.ToolResult {
	switch outputMode {
	case "content":
		return tools.TextResult("No matches found")
	case "count":
		return tools.TextResult("No matches found")
	default: // files_with_matches
		return tools.TextResult("No files found")
	}
}

// formatOutput applies head_limit/offset and formats the output based on output_mode.
func (t *GrepTool) formatOutput(lines []string, p *grepParams) (*tools.ToolResult, error) {
	switch p.outputMode {
	case "content":
		return t.formatContentOutput(lines, p), nil
	case "count":
		return t.formatCountOutput(lines, p), nil
	default: // files_with_matches
		return t.formatFilesOutput(lines, p), nil
	}
}

// formatContentOutput handles output_mode=content: matching lines with context.
func (t *GrepTool) formatContentOutput(lines []string, p *grepParams) *tools.ToolResult {
	limited, truncated := applyHeadLimit(lines, p.headLimit, p.offset)

	// Convert absolute paths to relative paths to save tokens
	for i, line := range limited {
		limited[i] = relativizeLine(line, p.searchPath)
	}

	content := strings.Join(limited, "\n")
	if content == "" {
		content = "No matches found"
	}

	if truncated {
		content += fmt.Sprintf("\n\n[Showing results with pagination = %s]", formatLimitInfo(p.headLimit, p.offset))
	}

	return tools.TextResult(content)
}

// formatCountOutput handles output_mode=count: per-file match counts with summary.
func (t *GrepTool) formatCountOutput(lines []string, p *grepParams) *tools.ToolResult {
	limited, truncated := applyHeadLimit(lines, p.headLimit, p.offset)

	// Parse counts and relativize paths
	totalMatches := 0
	fileCount := 0
	for i, line := range limited {
		colonIdx := strings.LastIndex(line, ":")
		if colonIdx > 0 {
			filePath := line[:colonIdx]
			countStr := line[colonIdx+1:]
			count, err := strconv.Atoi(countStr)
			if err == nil {
				totalMatches += count
				fileCount++
			}
			relPath := relativePath(filePath, p.searchPath)
			limited[i] = relPath + ":" + countStr
		}
	}

	content := strings.Join(limited, "\n")
	if content == "" {
		content = "No matches found"
	}

	// Append summary
	occurrences := "occurrences"
	if totalMatches == 1 {
		occurrences = "occurrence"
	}
	files := "files"
	if fileCount == 1 {
		files = "file"
	}
	content += fmt.Sprintf("\n\nFound %d total %s across %d %s.", totalMatches, occurrences, fileCount, files)

	if truncated {
		content += fmt.Sprintf(" with pagination = %s", formatLimitInfo(p.headLimit, p.offset))
	}

	return tools.TextResult(content)
}

// formatFilesOutput handles output_mode=files_with_matches: file paths sorted by mtime.
func (t *GrepTool) formatFilesOutput(lines []string, p *grepParams) *tools.ToolResult {
	// Sort by modification time (most recent first), filename as tiebreaker
	type fileEntry struct {
		path  string
		mtime int64
	}
	entries := make([]fileEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		info, err := os.Stat(line)
		var mtime int64
		if err == nil {
			mtime = info.ModTime().UnixMilli()
		}
		entries = append(entries, fileEntry{path: line, mtime: mtime})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].mtime != entries[j].mtime {
			return entries[i].mtime > entries[j].mtime // newest first
		}
		return entries[i].path < entries[j].path
	})

	sorted := make([]string, len(entries))
	for i, e := range entries {
		sorted[i] = e.path
	}

	// Apply head_limit/offset
	limited, truncated := applyHeadLimit(sorted, p.headLimit, p.offset)

	// Relativize paths
	for i, fp := range limited {
		limited[i] = relativePath(fp, p.searchPath)
	}

	if len(limited) == 0 {
		return tools.TextResult("No files found")
	}

	numFiles := len(limited)
	fileWord := "files"
	if numFiles == 1 {
		fileWord = "file"
	}
	header := fmt.Sprintf("Found %d %s", numFiles, fileWord)
	if truncated {
		header += fmt.Sprintf(" %s", formatLimitInfo(p.headLimit, p.offset))
	}

	return tools.TextResult(header + "\n" + strings.Join(limited, "\n"))
}

// applyHeadLimit applies offset and headLimit to a slice of items.
// Returns the limited slice and whether truncation occurred.
// headLimit 0 means unlimited.
func applyHeadLimit(items []string, headLimit, offset int) ([]string, bool) {
	// Skip offset entries
	if offset > 0 {
		if offset >= len(items) {
			return nil, false
		}
		items = items[offset:]
	}

	// headLimit 0 = unlimited
	if headLimit == 0 {
		return items, false
	}

	if len(items) > headLimit {
		return items[:headLimit], true
	}
	return items, false
}

// formatLimitInfo formats pagination information for display.
func formatLimitInfo(headLimit, offset int) string {
	var parts []string
	if headLimit > 0 {
		parts = append(parts, fmt.Sprintf("limit: %d", headLimit))
	}
	if offset > 0 {
		parts = append(parts, fmt.Sprintf("offset: %d", offset))
	}
	return strings.Join(parts, ", ")
}

// relativizeLine converts absolute file paths at the beginning of content-mode
// lines (format: /abs/path:rest) to relative paths.
func relativizeLine(line, basePath string) string {
	colonIdx := strings.Index(line, ":")
	if colonIdx > 0 {
		filePath := line[:colonIdx]
		rest := line[colonIdx:]
		relPath := relativePath(filePath, basePath)
		return relPath + rest
	}
	return line
}

// relativePath converts an absolute path to a relative path from basePath.
func relativePath(absPath, basePath string) string {
	rel, err := filepath.Rel(basePath, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// nativeGrep is a Go-native fallback when ripgrep is not installed.
func (t *GrepTool) nativeGrep(ctx context.Context, p *grepParams) (*tools.ToolResult, error) {
	re, err := regexp.Compile(p.pattern)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid regex pattern: %s", err)), nil
	}

	// For case-insensitive mode, recompile with (?i) prefix
	if p.caseInsI {
		re, err = regexp.Compile("(?i)" + p.pattern)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("invalid regex pattern: %s", err)), nil
		}
	}

	// For multiline mode, add (?s) prefix for dot-matches-newline
	if p.multiline {
		re, err = regexp.Compile("(?s)" + p.pattern)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("invalid regex pattern: %s", err)), nil
		}
	}

	type fileMatch struct {
		relPath string
		lines   []string // content mode: matching lines; count mode: not used
		count   int      // count mode: match count per file
	}

	var matches []fileMatch

	walkErr := filepath.Walk(p.searchPath, func(path string, info os.FileInfo, err error) error {
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
			name := info.Name()
			// Skip VCS directories
			for _, vcs := range vcsDirectoriesToExclude {
				if name == vcs {
					return filepath.SkipDir
				}
			}
			// Skip other hidden directories
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply glob filter
		if p.glob != "" {
			matched, _ := filepath.Match(p.glob, info.Name())
			if !matched {
				return nil
			}
		}

		// Apply type filter (simple mapping for common types)
		if p.fileType != "" {
			ext := strings.TrimPrefix(filepath.Ext(info.Name()), ".")
			if !matchFileType(ext, p.fileType) {
				return nil
			}
		}

		// Skip binary files (simple heuristic: skip files > 1MB)
		if info.Size() > 1<<20 {
			return nil
		}

		relPath, relErr := filepath.Rel(p.searchPath, path)
		if relErr != nil {
			relPath = path
		}

		if p.multiline {
			// Read entire file for multiline matching
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			if re.Match(content) {
				matchCount := len(re.FindAll(content, -1))
				matches = append(matches, fileMatch{
					relPath: relPath,
					count:   matchCount,
				})
			}
			return nil
		}

		fileLines := t.searchFileLines(path, re, p)
		if fileLines != nil {
			fm := fileMatch{
				relPath: relPath,
				lines:   fileLines.lines,
				count:   fileLines.count,
			}
			matches = append(matches, fm)
		}

		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
		return tools.ErrorResult(fmt.Sprintf("search error: %s", walkErr)), nil
	}

	if len(matches) == 0 {
		return t.formatNoMatches(p.outputMode), nil
	}

	// Build output based on output mode
	switch p.outputMode {
	case "content":
		var allLines []string
		for _, fm := range matches {
			for _, line := range fm.lines {
				allLines = append(allLines, fmt.Sprintf("%s:%s", fm.relPath, line))
			}
		}
		limited, truncated := applyHeadLimit(allLines, p.headLimit, p.offset)
		content := strings.Join(limited, "\n")
		if content == "" {
			content = "No matches found"
		}
		if truncated {
			content += fmt.Sprintf("\n\n[Showing results with pagination = %s]", formatLimitInfo(p.headLimit, p.offset))
		}
		return tools.TextResult(content), nil

	case "count":
		var countLines []string
		totalMatches := 0
		fileCount := 0
		for _, fm := range matches {
			countLines = append(countLines, fmt.Sprintf("%s:%d", fm.relPath, fm.count))
			totalMatches += fm.count
			fileCount++
		}
		limited, truncated := applyHeadLimit(countLines, p.headLimit, p.offset)
		content := strings.Join(limited, "\n")
		occurrences := "occurrences"
		if totalMatches == 1 {
			occurrences = "occurrence"
		}
		files := "files"
		if fileCount == 1 {
			files = "file"
		}
		content += fmt.Sprintf("\n\nFound %d total %s across %d %s.", totalMatches, occurrences, fileCount, files)
		if truncated {
			content += fmt.Sprintf(" with pagination = %s", formatLimitInfo(p.headLimit, p.offset))
		}
		return tools.TextResult(content), nil

	default: // files_with_matches
		var filePaths []string
		for _, fm := range matches {
			filePaths = append(filePaths, fm.relPath)
		}
		// Sort alphabetically for native fallback (no mtime info readily available without stat)
		sort.Strings(filePaths)
		limited, truncated := applyHeadLimit(filePaths, p.headLimit, p.offset)
		if len(limited) == 0 {
			return tools.TextResult("No files found"), nil
		}
		numFiles := len(limited)
		fileWord := "files"
		if numFiles == 1 {
			fileWord = "file"
		}
		header := fmt.Sprintf("Found %d %s", numFiles, fileWord)
		if truncated {
			header += fmt.Sprintf(" %s", formatLimitInfo(p.headLimit, p.offset))
		}
		return tools.TextResult(header + "\n" + strings.Join(limited, "\n")), nil
	}
}

// searchFileResult holds search results for a single file.
type searchFileResult struct {
	lines []string
	count int
}

// searchFileLines searches a single file for the pattern and returns results.
func (t *GrepTool) searchFileLines(path string, re *regexp.Regexp, p *grepParams) *searchFileResult {
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

	var matchIndices []int
	for i, line := range allLines {
		if re.MatchString(line) {
			matchIndices = append(matchIndices, i)
		}
	}

	if len(matchIndices) == 0 {
		return nil
	}

	result := &searchFileResult{count: len(matchIndices)}

	if p.outputMode == "count" || p.outputMode == "files_with_matches" {
		// No need to produce line content for these modes
		return result
	}

	// Content mode: produce lines with context
	used := make(map[int]bool)
	var resultLines []string

	for _, mi := range matchIndices {
		start := mi
		end := mi + 1

		// Apply context
		if p.effectiveContext > 0 {
			start = mi - p.effectiveContext
			end = mi + p.effectiveContext + 1
		} else {
			if p.beforeB > 0 {
				start = mi - p.beforeB
			}
			if p.afterA > 0 {
				end = mi + p.afterA + 1
			}
		}

		if start < 0 {
			start = 0
		}
		if end > len(allLines) {
			end = len(allLines)
		}

		for j := start; j < end; j++ {
			if used[j] {
				continue
			}
			used[j] = true
			sep := "-"
			if j == mi {
				sep = ":"
			}
			if p.lineNumsN {
				resultLines = append(resultLines, fmt.Sprintf("%d%s%s", j+1, sep, allLines[j]))
			} else {
				resultLines = append(resultLines, allLines[j])
			}
		}
	}

	result.lines = resultLines
	return result
}

// matchFileType checks whether a file extension matches a ripgrep-style file type.
func matchFileType(ext, fileType string) bool {
	typeMap := map[string][]string{
		"go":     {"go"},
		"py":     {"py"},
		"js":     {"js", "mjs", "cjs"},
		"ts":     {"ts", "tsx", "mts", "cts"},
		"rust":   {"rs"},
		"java":   {"java"},
		"c":      {"c", "h"},
		"cpp":    {"cpp", "cc", "cxx", "hpp", "hxx", "h"},
		"ruby":   {"rb"},
		"php":    {"php"},
		"swift":  {"swift"},
		"kotlin": {"kt", "kts"},
		"scala":  {"scala"},
		"lua":    {"lua"},
		"sh":     {"sh", "bash", "zsh"},
		"css":    {"css"},
		"html":   {"html", "htm"},
		"json":   {"json"},
		"yaml":   {"yaml", "yml"},
		"toml":   {"toml"},
		"xml":    {"xml"},
		"md":     {"md", "markdown"},
	}

	exts, ok := typeMap[fileType]
	if !ok {
		// Fallback: exact extension match
		return ext == fileType
	}
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

// CheckPermissions determines whether the tool should be allowed, denied, or require user prompt.
func (t *GrepTool) CheckPermissions(ctx context.Context, input json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission(t.Name(), t.IsReadOnly(), permCtx), nil
}

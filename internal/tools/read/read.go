// Package read implements the ReadTool for reading files from the filesystem.
// It returns file contents in cat -n format with line numbers, supports
// offset/limit for partial reads, and detects binary files.
// Also handles images (base64), PDFs (base64 with page ranges), Jupyter
// notebooks (cell extraction), and encoding detection (UTF-16 BOM).
package read

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/khaledmoayad/clawgo/internal/filestate"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const defaultLimit = 2000
const binaryCheckSize = 8192

// blockedDevicePaths contains device paths that would hang the process
// (infinite output, blocking input, or nonsensical to read).
// Matches the TypeScript BLOCKED_DEVICE_PATHS set.
var blockedDevicePaths = map[string]bool{
	// Infinite output -- never reach EOF
	"/dev/zero":   true,
	"/dev/random": true,
	"/dev/urandom": true,
	"/dev/full":   true,
	// Blocks waiting for input
	"/dev/stdin":   true,
	"/dev/tty":     true,
	"/dev/console": true,
	// Nonsensical to read
	"/dev/stdout": true,
	"/dev/stderr": true,
	// fd aliases for stdin/stdout/stderr
	"/dev/fd/0": true,
	"/dev/fd/1": true,
	"/dev/fd/2": true,
}

// isBlockedDevicePath checks if a file path is a blocked device path.
func isBlockedDevicePath(filePath string) bool {
	if blockedDevicePaths[filePath] {
		return true
	}
	// /proc/self/fd/0-2 and /proc/<pid>/fd/0-2 are Linux aliases for stdio
	if strings.HasPrefix(filePath, "/proc/") &&
		(strings.HasSuffix(filePath, "/fd/0") ||
			strings.HasSuffix(filePath, "/fd/1") ||
			strings.HasSuffix(filePath, "/fd/2")) {
		return true
	}
	return false
}

type input struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"` // 0-indexed line offset
	Limit    int    `json:"limit"`  // number of lines to read, 0 = default
	Pages    string `json:"pages"`  // page range for PDF files
}

// ReadTool reads files from the filesystem.
type ReadTool struct{}

// New creates a new ReadTool.
func New() *ReadTool { return &ReadTool{} }

func (t *ReadTool) Name() string                { return "Read" }
func (t *ReadTool) Description() string          { return toolDescription }
func (t *ReadTool) IsReadOnly() bool             { return true }
func (t *ReadTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because reading files is always safe for concurrent execution.
func (t *ReadTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *ReadTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("Read", true, permCtx), nil
}

func (t *ReadTool) Call(_ context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.FilePath) == "" {
		return tools.ErrorResult("required field \"file_path\" is missing or empty"), nil
	}

	// Block dangerous device paths that would hang or produce infinite output
	if isBlockedDevicePath(in.FilePath) {
		return tools.ErrorResult(fmt.Sprintf("Cannot read %s: this device path would block or produce infinite output", in.FilePath)), nil
	}

	// Dispatch based on file type before falling through to text reading
	if isImageFile(in.FilePath) {
		result, err := readImageFile(in.FilePath)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Failed to read image: %s", err.Error())), nil
		}
		// Record in file state cache with partial view (model sees processed version)
		if toolCtx != nil && toolCtx.FileStateCache != nil {
			toolCtx.FileStateCache.Set(in.FilePath, filestate.FileState{
				Content:       fmt.Sprintf("[image file: %s]", in.FilePath),
				Timestamp:     time.Now().UnixMilli(),
				IsPartialView: true,
			})
		}
		return result, nil
	}

	if isPDFFile(in.FilePath) {
		result, err := readPDFFile(in.FilePath, in.Pages)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Failed to read PDF: %s", err.Error())), nil
		}
		// Record in file state cache with partial view
		if toolCtx != nil && toolCtx.FileStateCache != nil {
			toolCtx.FileStateCache.Set(in.FilePath, filestate.FileState{
				Content:       fmt.Sprintf("[PDF file: %s]", in.FilePath),
				Timestamp:     time.Now().UnixMilli(),
				IsPartialView: true,
			})
		}
		return result, nil
	}

	if isNotebookFile(in.FilePath) {
		result, err := readNotebookFile(in.FilePath)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Failed to read notebook: %s", err.Error())), nil
		}
		// Record notebook read in file state cache
		if toolCtx != nil && toolCtx.FileStateCache != nil {
			text := ""
			if len(result.Content) > 0 {
				text = result.Content[0].Text
			}
			toolCtx.FileStateCache.Set(in.FilePath, filestate.FileState{
				Content:   text,
				Timestamp: time.Now().UnixMilli(),
			})
		}
		return result, nil
	}

	// Read the file
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to read file: %s", err.Error())), nil
	}

	// Encoding detection: handle UTF-16 BOM by decoding to UTF-8
	data = decodeIfUTF16(data)

	// Check for binary content (null bytes in first 8192 bytes)
	checkLen := len(data)
	if checkLen > binaryCheckSize {
		checkLen = binaryCheckSize
	}
	if bytes.ContainsRune(data[:checkLen], 0) {
		return tools.ErrorResult("File appears to be binary and cannot be displayed as text"), nil
	}

	// Handle empty file
	content := string(data)
	if content == "" {
		return tools.TextResult("(empty file)"), nil
	}

	// Split into lines
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Apply offset
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		return tools.TextResult("(offset beyond end of file)"), nil
	}

	// Apply limit
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	// Format in cat -n style: right-aligned 6-char line number + tab + content
	// Line numbers start at 1 + offset
	var output strings.Builder
	for i := offset; i < end; i++ {
		lineNum := i + 1 // 1-indexed line numbers
		fmt.Fprintf(&output, "%6d\t%s\n", lineNum, lines[i])
	}

	// Record the read in the file state cache for read-before-edit enforcement
	if toolCtx != nil && toolCtx.FileStateCache != nil {
		toolCtx.FileStateCache.Set(in.FilePath, filestate.FileState{
			Content:   content,
			Timestamp: time.Now().UnixMilli(),
			Offset:    offset,
			Limit:     limit,
		})
	}

	return tools.TextResult(output.String()), nil
}

// UTF-16 BOM markers
var (
	utf16LEBOM = []byte{0xFF, 0xFE}
	utf16BEBOM = []byte{0xFE, 0xFF}
	utf8BOM    = []byte{0xEF, 0xBB, 0xBF}
)

// decodeIfUTF16 checks for BOM markers and decodes UTF-16 to UTF-8.
// If a UTF-8 BOM is found, it is stripped. If no BOM is found, the data
// is returned as-is (Go strings are UTF-8 by default; non-UTF8 bytes
// become replacement characters when converted to string).
func decodeIfUTF16(data []byte) []byte {
	// UTF-8 BOM: strip it
	if bytes.HasPrefix(data, utf8BOM) {
		return data[3:]
	}

	// UTF-16 LE BOM
	if bytes.HasPrefix(data, utf16LEBOM) {
		return decodeUTF16LE(data[2:])
	}

	// UTF-16 BE BOM
	if bytes.HasPrefix(data, utf16BEBOM) {
		return decodeUTF16BE(data[2:])
	}

	return data
}

// decodeUTF16LE decodes UTF-16 Little Endian bytes to UTF-8.
func decodeUTF16LE(data []byte) []byte {
	if len(data)%2 != 0 {
		data = data[:len(data)-1] // trim incomplete final byte
	}
	u16s := make([]uint16, len(data)/2)
	for i := range u16s {
		u16s[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
	}
	runes := utf16.Decode(u16s)
	var buf bytes.Buffer
	b := make([]byte, 4)
	for _, r := range runes {
		n := utf8.EncodeRune(b, r)
		buf.Write(b[:n])
	}
	return buf.Bytes()
}

// decodeUTF16BE decodes UTF-16 Big Endian bytes to UTF-8.
func decodeUTF16BE(data []byte) []byte {
	if len(data)%2 != 0 {
		data = data[:len(data)-1] // trim incomplete final byte
	}
	u16s := make([]uint16, len(data)/2)
	for i := range u16s {
		u16s[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
	}
	runes := utf16.Decode(u16s)
	var buf bytes.Buffer
	b := make([]byte, 4)
	for _, r := range runes {
		n := utf8.EncodeRune(b, r)
		buf.Write(b[:n])
	}
	return buf.Bytes()
}

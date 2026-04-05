package read

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/tools"
)

// maxPDFSizeWithoutPages is the max PDF size (10MB) when no pages param is provided.
const maxPDFSizeWithoutPages = 10 * 1024 * 1024

// maxPDFPagesPerRequest is the maximum number of pages per read request.
const maxPDFPagesPerRequest = 20

// isPDFFile checks whether the given path has a .pdf extension.
func isPDFFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".pdf"
}

// parsePDFPageRange parses a page range string into start and end page numbers (1-indexed).
// Supported formats: "3" (single page), "1-5" (range), "10-20" (range).
// Returns an error if the range is invalid.
func parsePDFPageRange(pages string) (start, end int, err error) {
	pages = strings.TrimSpace(pages)
	if pages == "" {
		return 0, 0, fmt.Errorf("empty page range")
	}

	// Check for range format: "N-M"
	if idx := strings.Index(pages, "-"); idx >= 0 {
		startStr := pages[:idx]
		endStr := pages[idx+1:]

		start, err = strconv.Atoi(strings.TrimSpace(startStr))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start page: %q", startStr)
		}

		end, err = strconv.Atoi(strings.TrimSpace(endStr))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end page: %q", endStr)
		}
	} else {
		// Single page: "N"
		page, err := strconv.Atoi(pages)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid page number: %q", pages)
		}
		start = page
		end = page
	}

	// Validate
	if start < 1 {
		return 0, 0, fmt.Errorf("start page must be >= 1, got %d", start)
	}
	if end < start {
		return 0, 0, fmt.Errorf("end page (%d) must be >= start page (%d)", end, start)
	}
	if end-start+1 > maxPDFPagesPerRequest {
		return 0, 0, fmt.Errorf("maximum %d pages per request, requested %d", maxPDFPagesPerRequest, end-start+1)
	}

	return start, end, nil
}

// readPDFFile reads a PDF file and returns a ToolResult with base64-encoded data.
// If pages is non-empty, the page range is validated.
func readPDFFile(path string, pages string) (*tools.ToolResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF file: %w", err)
	}

	// If no pages param and file is too large, suggest using pages parameter
	if pages == "" && len(data) > maxPDFSizeWithoutPages {
		return tools.ErrorResult(
			fmt.Sprintf("PDF file is too large (%d bytes, max %d bytes without pages parameter). "+
				"Please provide the pages parameter to read specific page ranges (e.g., pages: \"1-5\").",
				len(data), maxPDFSizeWithoutPages),
		), nil
	}

	// Validate pages parameter if provided
	if pages != "" {
		_, _, err := parsePDFPageRange(pages)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Invalid pages parameter: %s", err.Error())), nil
		}
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	return &tools.ToolResult{
		Content: []tools.ContentBlock{
			{Type: "text", Text: fmt.Sprintf("PDF file: %s (%d bytes)", path, len(data))},
		},
		Metadata: map[string]any{
			"base64":     encoded,
			"media_type": "application/pdf",
			"file_size":  len(data),
			"pages":      pages,
		},
	}, nil
}

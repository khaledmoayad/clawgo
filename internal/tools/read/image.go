package read

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/tools"
)

// IMAGE_EXTENSIONS is the set of file extensions recognized as images.
// Matches the TypeScript IMAGE_EXTENSIONS set exactly.
var IMAGE_EXTENSIONS = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
}

// maxImageSize is the maximum image file size (20MB).
const maxImageSize = 20 * 1024 * 1024

// isImageFile checks whether the given path has an image extension.
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return IMAGE_EXTENSIONS[ext]
}

// mimeTypeForImageExt returns the MIME type for the given image extension.
func mimeTypeForImageExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// readImageFile reads an image file and returns a ToolResult with base64 metadata.
func readImageFile(path string) (*tools.ToolResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	if len(data) > maxImageSize {
		return tools.ErrorResult("Image file too large (max 20MB)"), nil
	}

	ext := filepath.Ext(path)
	mimeType := mimeTypeForImageExt(ext)
	encoded := base64.StdEncoding.EncodeToString(data)

	return &tools.ToolResult{
		Content: []tools.ContentBlock{
			{Type: "text", Text: fmt.Sprintf("Image file: %s (%d bytes, %s)", path, len(data), mimeType)},
		},
		Metadata: map[string]any{
			"base64":     encoded,
			"media_type": mimeType,
			"file_size":  len(data),
		},
	}, nil
}

package mcp

import (
	"encoding/base64"
	"fmt"
	"strings"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MaxMCPDescriptionLength caps tool descriptions sent to the model.
// OpenAPI-generated MCP servers can dump 15-60 KB of endpoint docs; this caps
// the p95 tail without losing the intent.
const MaxMCPDescriptionLength = 2048

// MaxMCPResultChars is the maximum number of characters allowed in a
// normalized MCP tool result text payload. Results exceeding this limit are
// truncated with a suffix explaining the truncation.
const MaxMCPResultChars = 50000

// truncationSuffix is appended to text that was truncated.
const truncationSuffix = "\n\n[OUTPUT TRUNCATED - exceeded character limit. " +
	"If this MCP server provides pagination or filtering tools, use them to " +
	"retrieve specific portions of the data.]"

// NormalizeCallToolResult takes a raw CallToolResult from an MCP server and
// returns a version that is safe and bounded for the conversation surface:
//
//  1. Text content is preserved as-is (up to MaxMCPResultChars).
//  2. _meta data on the result is preserved.
//  3. Unsupported content types are converted to human-readable text summaries
//     rather than silently dropped.
//  4. Oversized text payloads are capped at MaxMCPResultChars.
//  5. Image content is routed through NormalizeImageContent for
//     resizing/downsampling before returning.
func NormalizeCallToolResult(result *gomcp.CallToolResult) (*gomcp.CallToolResult, error) {
	if result == nil {
		return result, nil
	}

	normalized := &gomcp.CallToolResult{
		Meta:    result.Meta,
		IsError: result.IsError,
	}

	// Preserve structured content if present.
	if result.StructuredContent != nil {
		normalized.StructuredContent = result.StructuredContent
	}

	var content []gomcp.Content
	for _, c := range result.Content {
		switch v := c.(type) {
		case *gomcp.TextContent:
			text := v.Text
			if len(text) > MaxMCPResultChars {
				text = text[:MaxMCPResultChars] + truncationSuffix
			}
			content = append(content, &gomcp.TextContent{
				Text: text,
			})

		case *gomcp.ImageContent:
			img, err := NormalizeImageContent(v)
			if err != nil {
				// If image normalization fails, convert to a text summary.
				content = append(content, &gomcp.TextContent{
					Text: fmt.Sprintf("[Image content (%s): normalization failed: %v]",
						v.MIMEType, err),
				})
				continue
			}
			content = append(content, img)

		case *gomcp.AudioContent:
			// Audio is not directly renderable in the conversation surface --
			// convert to a text summary.
			content = append(content, &gomcp.TextContent{
				Text: fmt.Sprintf("[Audio content (%s, %d bytes base64)]",
					v.MIMEType, len(v.Data)),
			})

		default:
			// Unknown / unsupported content type -- produce a readable summary
			// instead of silently dropping.
			content = append(content, &gomcp.TextContent{
				Text: fmt.Sprintf("[Unsupported MCP content type: %T]", c),
			})
		}
	}

	normalized.Content = content
	return normalized, nil
}

// NormalizeImageContent validates and optionally resizes an MCP image content
// block so it stays within API dimension and size limits. Currently it passes
// the image through with basic validation; full resize/downsample support will
// be added when the image processing utilities are ported.
func NormalizeImageContent(img *gomcp.ImageContent) (gomcp.Content, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image content")
	}

	// Validate the base64 data is well-formed.
	if len(img.Data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}

	// Validate base64 encoding.
	if _, err := base64.StdEncoding.DecodeString(
		string(img.Data),
	); err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %w", err)
	}

	// Validate MIME type.
	if img.MIMEType == "" {
		img.MIMEType = "image/png" // default
	}
	if !isValidImageMIME(img.MIMEType) {
		return nil, fmt.Errorf("unsupported image MIME type: %s", img.MIMEType)
	}

	// Return as-is for now. When the image resizer is ported, this function
	// will enforce dimension and byte-size caps.
	return img, nil
}

// isValidImageMIME checks if the MIME type is a supported image format.
func isValidImageMIME(mime string) bool {
	mime = strings.ToLower(mime)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml":
		return true
	default:
		return false
	}
}

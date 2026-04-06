package api

import (
	"crypto/sha256"
	"fmt"
	"io"
)

// ComputeFingerprint produces a stable hash from the key elements of an API
// request: message roles, content types, text content, system prompt, and
// tool names. The fingerprint is used for request deduplication logging and
// diagnostic capture.
//
// Volatile fields (timestamps, IDs, message_id) are skipped to ensure
// stability across otherwise-identical requests. Image/document data is
// also skipped (too large and not semantically useful for dedup).
//
// Returns a SHA-256 hash truncated to 16 hex chars for compact representation.
func ComputeFingerprint(messages []Message, systemPrompt string, toolNames []string) string {
	h := sha256.New()

	// Include system prompt
	writeField(h, "system", systemPrompt)

	// Include tool names (sorted order not required -- caller should provide
	// consistent ordering from the registry)
	for _, name := range toolNames {
		writeField(h, "tool", name)
	}

	// Include message content
	for _, msg := range messages {
		writeField(h, "role", msg.Role)
		for _, block := range msg.Content {
			writeField(h, "block_type", string(block.Type))
			switch block.Type {
			case ContentText:
				writeField(h, "text", block.Text)
			case ContentToolUse:
				writeField(h, "tool_name", block.Name)
				if block.Input != nil {
					writeField(h, "tool_input", string(block.Input))
				}
			case ContentToolResult:
				writeField(h, "tool_result", block.Content)
			case ContentThinking:
				writeField(h, "thinking", block.Thinking)
			case ContentImage, ContentDocument:
				// Skip binary data -- not useful for dedup and too large.
				// Include type marker only.
				writeField(h, "media", string(block.Type))
			}
		}
	}

	sum := h.Sum(nil)
	return fmt.Sprintf("%x", sum[:8]) // 16 hex chars
}

// ComputeFingerprintShort is like ComputeFingerprint but returns only 8 hex
// chars (32 bits) for more compact logging.
func ComputeFingerprintShort(messages []Message, systemPrompt string, toolNames []string) string {
	full := ComputeFingerprint(messages, systemPrompt, toolNames)
	if len(full) >= 8 {
		return full[:8]
	}
	return full
}

// writeField writes a labeled field to the hash. Using labels prevents
// collisions between fields with the same content in different positions.
func writeField(h io.Writer, label, value string) {
	// Write label length + label + value length + value to prevent
	// ambiguous concatenation
	fmt.Fprintf(h, "%d:%s:%d:%s\n", len(label), label, len(value), value)
}

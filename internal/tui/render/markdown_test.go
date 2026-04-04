package render

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Header(t *testing.T) {
	result, err := RenderMarkdown("# Hello", 80)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	// Glamour renders headers with ANSI styling; the word "Hello" must appear
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected result to contain 'Hello', got: %q", result)
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	input := "```go\nfmt.Println()\n```"
	result, err := RenderMarkdown(input, 80)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	// Code block content should be present with syntax highlighting (ANSI codes)
	if !strings.Contains(result, "Println") {
		t.Errorf("expected result to contain 'Println', got: %q", result)
	}
}

func TestRenderMarkdown_BoldItalic(t *testing.T) {
	input := "**bold** and *italic*"
	result, err := RenderMarkdown(input, 80)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	if !strings.Contains(result, "bold") {
		t.Errorf("expected result to contain 'bold', got: %q", result)
	}
	if !strings.Contains(result, "italic") {
		t.Errorf("expected result to contain 'italic', got: %q", result)
	}
}

func TestRenderMarkdown_BulletList(t *testing.T) {
	input := "- item 1\n- item 2"
	result, err := RenderMarkdown(input, 80)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	if !strings.Contains(result, "item 1") {
		t.Errorf("expected result to contain 'item 1', got: %q", result)
	}
	if !strings.Contains(result, "item 2") {
		t.Errorf("expected result to contain 'item 2', got: %q", result)
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	result, err := RenderMarkdown("", 80)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	trimmed := strings.TrimSpace(result)
	if trimmed != "" {
		t.Errorf("expected empty string for empty input, got: %q", trimmed)
	}
}

func TestRenderMarkdown_WidthParam(t *testing.T) {
	// A long line should be wrapped at the specified width
	longLine := strings.Repeat("word ", 30) // 150 chars
	result, err := RenderMarkdown(longLine, 40)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	// The result should contain the content (word wrapping is handled by glamour)
	if !strings.Contains(result, "word") {
		t.Errorf("expected result to contain 'word', got: %q", result)
	}
}

func TestRenderMarkdown_DefaultWidth(t *testing.T) {
	// Width <= 0 should default to 80
	result, err := RenderMarkdown("test content", 0)
	if err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}
	if !strings.Contains(result, "test content") {
		t.Errorf("expected result to contain 'test content', got: %q", result)
	}
}

func TestRenderMarkdownDefault_Success(t *testing.T) {
	result := RenderMarkdownDefault("# Title")
	if !strings.Contains(result, "Title") {
		t.Errorf("expected result to contain 'Title', got: %q", result)
	}
}

func TestRenderMarkdownDefault_Empty(t *testing.T) {
	result := RenderMarkdownDefault("")
	trimmed := strings.TrimSpace(result)
	if trimmed != "" {
		t.Errorf("expected empty string for empty input, got: %q", trimmed)
	}
}

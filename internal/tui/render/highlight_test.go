package render

import (
	"strings"
	"testing"
)

func TestHighlightCode_Go(t *testing.T) {
	result, err := HighlightCode("fmt.Println(\"hi\")", "go")
	if err != nil {
		t.Fatalf("HighlightCode returned error: %v", err)
	}
	// Should contain the code with ANSI coloring
	if !strings.Contains(result, "Println") {
		t.Errorf("expected result to contain 'Println', got: %q", result)
	}
	// Should have ANSI escape codes for syntax highlighting
	if !strings.Contains(result, "\033[") {
		t.Errorf("expected ANSI escape codes in result, got: %q", result)
	}
}

func TestHighlightCode_Python(t *testing.T) {
	result, err := HighlightCode("print('hi')", "python")
	if err != nil {
		t.Fatalf("HighlightCode returned error: %v", err)
	}
	if !strings.Contains(result, "print") {
		t.Errorf("expected result to contain 'print', got: %q", result)
	}
	if !strings.Contains(result, "\033[") {
		t.Errorf("expected ANSI escape codes in result, got: %q", result)
	}
}

func TestHighlightCode_EmptyLanguage(t *testing.T) {
	input := "unknown code"
	result, err := HighlightCode(input, "")
	if err != nil {
		t.Fatalf("HighlightCode returned error: %v", err)
	}
	// With empty language, should return code as-is (no crash)
	if !strings.Contains(result, "unknown code") {
		t.Errorf("expected result to contain input text, got: %q", result)
	}
}

func TestHighlightCode_UnknownLanguage(t *testing.T) {
	input := "some code here"
	result, err := HighlightCode(input, "nonexistentlang")
	if err != nil {
		t.Fatalf("HighlightCode returned error: %v", err)
	}
	// Unknown language should return code as-is
	if !strings.Contains(result, "some code here") {
		t.Errorf("expected result to contain input text, got: %q", result)
	}
}

func TestHighlightCodeDefault_Go(t *testing.T) {
	result := HighlightCodeDefault("fmt.Println(\"hi\")", "go")
	if !strings.Contains(result, "Println") {
		t.Errorf("expected result to contain 'Println', got: %q", result)
	}
}

func TestHighlightCodeDefault_Empty(t *testing.T) {
	result := HighlightCodeDefault("plain text", "")
	if !strings.Contains(result, "plain text") {
		t.Errorf("expected result to contain 'plain text', got: %q", result)
	}
}

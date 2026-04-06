package api

import (
	"encoding/json"
	"testing"
)

func TestFingerprint_Stability(t *testing.T) {
	messages := []Message{
		{
			Role: "user",
			Content: []ContentBlock{
				{Type: ContentText, Text: "Hello, Claude!"},
			},
		},
		{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: ContentText, Text: "Hello! How can I help?"},
			},
		},
	}

	fp1 := ComputeFingerprint(messages, "You are helpful.", []string{"bash", "file_read"})
	fp2 := ComputeFingerprint(messages, "You are helpful.", []string{"bash", "file_read"})

	if fp1 != fp2 {
		t.Errorf("fingerprint should be stable: %s != %s", fp1, fp2)
	}

	// Verify length is 16 hex chars
	if len(fp1) != 16 {
		t.Errorf("expected 16 hex char fingerprint, got %d: %s", len(fp1), fp1)
	}
}

func TestFingerprint_DifferentMessages(t *testing.T) {
	msgs1 := []Message{
		{Role: "user", Content: []ContentBlock{{Type: ContentText, Text: "Hello"}}},
	}
	msgs2 := []Message{
		{Role: "user", Content: []ContentBlock{{Type: ContentText, Text: "Goodbye"}}},
	}

	fp1 := ComputeFingerprint(msgs1, "sys", nil)
	fp2 := ComputeFingerprint(msgs2, "sys", nil)

	if fp1 == fp2 {
		t.Error("different messages should produce different fingerprints")
	}
}

func TestFingerprint_DifferentSystemPrompt(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: []ContentBlock{{Type: ContentText, Text: "Hello"}}},
	}

	fp1 := ComputeFingerprint(msgs, "System A", nil)
	fp2 := ComputeFingerprint(msgs, "System B", nil)

	if fp1 == fp2 {
		t.Error("different system prompts should produce different fingerprints")
	}
}

func TestFingerprint_DifferentTools(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: []ContentBlock{{Type: ContentText, Text: "Hello"}}},
	}

	fp1 := ComputeFingerprint(msgs, "sys", []string{"bash"})
	fp2 := ComputeFingerprint(msgs, "sys", []string{"bash", "file_read"})

	if fp1 == fp2 {
		t.Error("different tool sets should produce different fingerprints")
	}
}

func TestFingerprint_IncludesToolUseContent(t *testing.T) {
	msgs1 := []Message{
		{Role: "assistant", Content: []ContentBlock{
			{Type: ContentToolUse, Name: "bash", Input: json.RawMessage(`{"cmd":"ls"}`)},
		}},
	}
	msgs2 := []Message{
		{Role: "assistant", Content: []ContentBlock{
			{Type: ContentToolUse, Name: "bash", Input: json.RawMessage(`{"cmd":"pwd"}`)},
		}},
	}

	fp1 := ComputeFingerprint(msgs1, "", nil)
	fp2 := ComputeFingerprint(msgs2, "", nil)

	if fp1 == fp2 {
		t.Error("different tool inputs should produce different fingerprints")
	}
}

func TestFingerprint_SkipsImageData(t *testing.T) {
	// Two messages with different image data should produce the same fingerprint
	// (image data is skipped for dedup purposes)
	msgs1 := []Message{
		{Role: "user", Content: []ContentBlock{
			{Type: ContentText, Text: "Look at this"},
			{Type: ContentImage, Source: &ImageSource{Data: "base64data1"}},
		}},
	}
	msgs2 := []Message{
		{Role: "user", Content: []ContentBlock{
			{Type: ContentText, Text: "Look at this"},
			{Type: ContentImage, Source: &ImageSource{Data: "base64data2"}},
		}},
	}

	fp1 := ComputeFingerprint(msgs1, "", nil)
	fp2 := ComputeFingerprint(msgs2, "", nil)

	if fp1 != fp2 {
		t.Error("fingerprints should be identical when only image data differs")
	}
}

func TestFingerprint_EmptyMessages(t *testing.T) {
	fp := ComputeFingerprint(nil, "", nil)
	if fp == "" {
		t.Error("empty input should still produce a valid fingerprint")
	}
	if len(fp) != 16 {
		t.Errorf("expected 16 hex char fingerprint, got %d: %s", len(fp), fp)
	}
}

func TestFingerprint_RoleDifference(t *testing.T) {
	msgs1 := []Message{
		{Role: "user", Content: []ContentBlock{{Type: ContentText, Text: "Hello"}}},
	}
	msgs2 := []Message{
		{Role: "assistant", Content: []ContentBlock{{Type: ContentText, Text: "Hello"}}},
	}

	fp1 := ComputeFingerprint(msgs1, "", nil)
	fp2 := ComputeFingerprint(msgs2, "", nil)

	if fp1 == fp2 {
		t.Error("different roles should produce different fingerprints")
	}
}

func TestFingerprintShort(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: []ContentBlock{{Type: ContentText, Text: "Hello"}}},
	}

	short := ComputeFingerprintShort(msgs, "sys", nil)
	if len(short) != 8 {
		t.Errorf("expected 8 char short fingerprint, got %d: %s", len(short), short)
	}

	full := ComputeFingerprint(msgs, "sys", nil)
	if short != full[:8] {
		t.Error("short fingerprint should be prefix of full fingerprint")
	}
}

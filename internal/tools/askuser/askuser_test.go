package askuser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/tools"
)

func callTool(t *testing.T, input any) (*tools.ToolResult, error) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	tool := New()
	return tool.Call(context.Background(), data, nil)
}

func mustSucceed(t *testing.T, result *tools.ToolResult, err error) *tools.ToolResult {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success but got error: %s", result.Content[0].Text)
	}
	return result
}

func mustFail(t *testing.T, result *tools.ToolResult, err error, substr string) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result but got success")
	}
	if substr != "" && !strings.Contains(result.Content[0].Text, substr) {
		t.Fatalf("expected error containing %q, got %q", substr, result.Content[0].Text)
	}
}

func makeQuestion(question, header string, opts []QuestionOption) Question {
	return Question{
		Question: question,
		Header:   header,
		Options:  opts,
	}
}

func twoOptions() []QuestionOption {
	return []QuestionOption{
		{Label: "Option A", Description: "First option"},
		{Label: "Option B", Description: "Second option"},
	}
}

func TestAskUserSingleQuestion(t *testing.T) {
	input := structuredInput{
		Questions: []Question{
			makeQuestion("Which approach?", "Approach", twoOptions()),
		},
	}
	res, err := callTool(t, input)
	result := mustSucceed(t, res, err)

	// Verify metadata signals.
	if result.Metadata["requires_user_input"] != true {
		t.Error("expected requires_user_input=true in metadata")
	}
	if result.Metadata["structured"] != true {
		t.Error("expected structured=true in metadata")
	}
	questions, ok := result.Metadata["questions"]
	if !ok {
		t.Fatal("expected questions in metadata")
	}
	qs, ok := questions.([]Question)
	if !ok {
		t.Fatalf("expected []Question in metadata, got %T", questions)
	}
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
}

func TestAskUserMultipleQuestions(t *testing.T) {
	input := structuredInput{
		Questions: []Question{
			makeQuestion("Question 1?", "Q1", twoOptions()),
			makeQuestion("Question 2?", "Q2", twoOptions()),
			makeQuestion("Question 3?", "Q3", twoOptions()),
		},
	}
	res, err := callTool(t, input)
	result := mustSucceed(t, res, err)
	qs := result.Metadata["questions"].([]Question)
	if len(qs) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(qs))
	}
}

func TestAskUserMaxQuestions(t *testing.T) {
	// 4 questions -- should succeed.
	input4 := structuredInput{
		Questions: []Question{
			makeQuestion("Q1?", "H1", twoOptions()),
			makeQuestion("Q2?", "H2", twoOptions()),
			makeQuestion("Q3?", "H3", twoOptions()),
			makeQuestion("Q4?", "H4", twoOptions()),
		},
	}
	res4, err4 := callTool(t, input4)
	mustSucceed(t, res4, err4)

	// 5 questions -- should fail.
	input5 := structuredInput{
		Questions: []Question{
			makeQuestion("Q1?", "H1", twoOptions()),
			makeQuestion("Q2?", "H2", twoOptions()),
			makeQuestion("Q3?", "H3", twoOptions()),
			makeQuestion("Q4?", "H4", twoOptions()),
			makeQuestion("Q5?", "H5", twoOptions()),
		},
	}
	res5, err5 := callTool(t, input5)
	mustFail(t, res5, err5, "at most 4 questions")
}

func TestAskUserMinOptions(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "Pick one?",
			Header:   "Pick",
			Options:  []QuestionOption{{Label: "Only", Description: "Sole choice"}},
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "at least 2 options")
}

func TestAskUserMaxOptions(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "Pick one?",
			Header:   "Pick",
			Options: []QuestionOption{
				{Label: "A", Description: "a"},
				{Label: "B", Description: "b"},
				{Label: "C", Description: "c"},
				{Label: "D", Description: "d"},
				{Label: "E", Description: "e"},
			},
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "at most 4 options")
}

func TestAskUserDuplicateQuestionText(t *testing.T) {
	input := structuredInput{
		Questions: []Question{
			makeQuestion("Same question?", "H1", twoOptions()),
			makeQuestion("Same question?", "H2", twoOptions()),
		},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "unique")
}

func TestAskUserDuplicateOptionLabels(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "Pick?",
			Header:   "Pick",
			Options: []QuestionOption{
				{Label: "Same", Description: "first"},
				{Label: "Same", Description: "second"},
			},
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "unique")
}

func TestAskUserMultiSelect(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question:    "Select features?",
			Header:      "Features",
			Options:     twoOptions(),
			MultiSelect: true,
		}},
	}
	res, err := callTool(t, input)
	result := mustSucceed(t, res, err)
	qs := result.Metadata["questions"].([]Question)
	if !qs[0].MultiSelect {
		t.Error("expected multiSelect=true to be preserved")
	}
}

func TestAskUserWithPreanswers(t *testing.T) {
	input := structuredInput{
		Questions: []Question{
			makeQuestion("Which approach?", "Approach", twoOptions()),
		},
		Answers: map[string]string{
			"Which approach?": "Option A",
		},
	}
	res, err := callTool(t, input)
	result := mustSucceed(t, res, err)

	// Pre-answered input should return answers directly, not requires_user_input.
	if result.Metadata["requires_user_input"] == true {
		t.Error("expected pre-answered result NOT to have requires_user_input")
	}
	answers, ok := result.Metadata["answers"]
	if !ok {
		t.Fatal("expected answers in metadata")
	}
	answersMap, ok := answers.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", answers)
	}
	if answersMap["Which approach?"] != "Option A" {
		t.Errorf("expected answer 'Option A', got %q", answersMap["Which approach?"])
	}

	// Content should be the JSON-marshaled answers.
	var parsed map[string]string
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("content should be valid JSON: %v", err)
	}
	if parsed["Which approach?"] != "Option A" {
		t.Errorf("expected parsed answer 'Option A', got %q", parsed["Which approach?"])
	}
}

func TestAskUserEmptyQuestion(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "",
			Header:   "H",
			Options:  twoOptions(),
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "must not be empty")
}

func TestAskUserEmptyHeader(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "What?",
			Header:   "",
			Options:  twoOptions(),
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "must not be empty")
}

func TestAskUserEmptyOptionLabel(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "What?",
			Header:   "H",
			Options: []QuestionOption{
				{Label: "", Description: "desc"},
				{Label: "B", Description: "desc"},
			},
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "must not be empty")
}

func TestAskUserEmptyOptionDescription(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "What?",
			Header:   "H",
			Options: []QuestionOption{
				{Label: "A", Description: ""},
				{Label: "B", Description: "desc"},
			},
		}},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "must not be empty")
}

func TestAskUserDisplayFormat(t *testing.T) {
	questions := []Question{
		{
			Question: "Which library?",
			Header:   "Library",
			Options: []QuestionOption{
				{Label: "React", Description: "UI framework"},
				{Label: "Vue", Description: "Progressive framework"},
			},
		},
		{
			Question: "Which style?",
			Header:   "Style",
			Options: []QuestionOption{
				{Label: "CSS", Description: "Plain CSS"},
				{Label: "Tailwind", Description: "Utility-first"},
				{Label: "Styled", Description: "CSS-in-JS"},
			},
		},
	}
	output := formatQuestionsForDisplay(questions)

	// Verify structure.
	if !strings.Contains(output, "[Library] Which library?") {
		t.Errorf("expected header+question format, got:\n%s", output)
	}
	if !strings.Contains(output, "[Style] Which style?") {
		t.Errorf("expected second header+question format, got:\n%s", output)
	}
	if !strings.Contains(output, "a) React - UI framework") {
		t.Errorf("expected option a format, got:\n%s", output)
	}
	if !strings.Contains(output, "b) Vue - Progressive framework") {
		t.Errorf("expected option b format, got:\n%s", output)
	}
	if !strings.Contains(output, "c) Styled - CSS-in-JS") {
		t.Errorf("expected option c format, got:\n%s", output)
	}
}

func TestAskUserNoQuestions(t *testing.T) {
	input := structuredInput{
		Questions: []Question{},
	}
	res, err := callTool(t, input)
	mustFail(t, res, err, "at least 1 question")
}

func TestAskUserPreviewPreserved(t *testing.T) {
	input := structuredInput{
		Questions: []Question{{
			Question: "Which?",
			Header:   "H",
			Options: []QuestionOption{
				{Label: "A", Description: "desc a", Preview: "preview content"},
				{Label: "B", Description: "desc b"},
			},
		}},
	}
	res, err := callTool(t, input)
	result := mustSucceed(t, res, err)
	qs := result.Metadata["questions"].([]Question)
	if qs[0].Options[0].Preview != "preview content" {
		t.Errorf("expected preview to be preserved, got %q", qs[0].Options[0].Preview)
	}
}

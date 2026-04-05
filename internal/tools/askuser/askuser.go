// Package askuser implements the AskUserQuestionTool for interactive user prompts.
// It supports 1-4 structured questions, each with 2-4 options (label/description/preview),
// multiSelect toggle, and uniqueness validation -- matching the TypeScript AskUserQuestionTool.
package askuser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// QuestionOption represents a single selectable choice within a question.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Preview     string `json:"preview,omitempty"`
}

// Question represents a single question with header, options, and optional multiSelect.
type Question struct {
	Question    string           `json:"question"`
	Header      string           `json:"header"`
	Options     []QuestionOption `json:"options"`
	MultiSelect bool             `json:"multiSelect,omitempty"`
}

// structuredInput is the full input payload for AskUserQuestion.
type structuredInput struct {
	Questions   []Question        `json:"questions"`
	Answers     map[string]string `json:"answers,omitempty"`
	Annotations map[string]any    `json:"annotations,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// Validate implements tools.Validatable for structuredInput.
func (in *structuredInput) Validate() error {
	// Validate question count: 1-4 questions.
	if len(in.Questions) == 0 {
		return fmt.Errorf("questions: must have at least 1 question")
	}
	if len(in.Questions) > 4 {
		return fmt.Errorf("questions: must have at most 4 questions, got %d", len(in.Questions))
	}

	// Track question texts for uniqueness.
	seenQuestions := make(map[string]bool, len(in.Questions))

	for i, q := range in.Questions {
		// Validate non-empty question text.
		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("questions[%d].question: must not be empty", i)
		}
		// Validate non-empty header.
		if strings.TrimSpace(q.Header) == "" {
			return fmt.Errorf("questions[%d].header: must not be empty", i)
		}

		// Enforce question text uniqueness.
		if seenQuestions[q.Question] {
			return fmt.Errorf("question texts must be unique, duplicate: %q", q.Question)
		}
		seenQuestions[q.Question] = true

		// Validate option count: 2-4 options per question.
		if len(q.Options) < 2 {
			return fmt.Errorf("questions[%d].options: must have at least 2 options, got %d", i, len(q.Options))
		}
		if len(q.Options) > 4 {
			return fmt.Errorf("questions[%d].options: must have at most 4 options, got %d", i, len(q.Options))
		}

		// Track option labels for uniqueness within each question.
		seenLabels := make(map[string]bool, len(q.Options))
		for j, opt := range q.Options {
			if strings.TrimSpace(opt.Label) == "" {
				return fmt.Errorf("questions[%d].options[%d].label: must not be empty", i, j)
			}
			if strings.TrimSpace(opt.Description) == "" {
				return fmt.Errorf("questions[%d].options[%d].description: must not be empty", i, j)
			}
			if seenLabels[opt.Label] {
				return fmt.Errorf("option labels must be unique within each question, duplicate: %q in question %d", opt.Label, i)
			}
			seenLabels[opt.Label] = true
		}
	}
	return nil
}

// AskUserTool pauses execution to prompt the user for structured input.
type AskUserTool struct{}

// New creates a new AskUserTool.
func New() *AskUserTool { return &AskUserTool{} }

func (t *AskUserTool) Name() string                { return "AskUserQuestion" }
func (t *AskUserTool) Description() string          { return toolDescription }
func (t *AskUserTool) IsReadOnly() bool             { return true }
func (t *AskUserTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because it blocks on user input.
func (t *AskUserTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions returns Allow -- asking the user is always permitted.
func (t *AskUserTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *AskUserTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in structuredInput
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// If answers are already populated (from permission component flow),
	// return them directly as the result.
	if len(in.Answers) > 0 {
		answersJSON, err := json.Marshal(in.Answers)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("failed to marshal answers: %v", err)), nil
		}
		return &tools.ToolResult{
			Content: []tools.ContentBlock{{Type: "text", Text: string(answersJSON)}},
			Metadata: map[string]any{
				"answers":     in.Answers,
				"annotations": in.Annotations,
			},
		}, nil
	}

	// Return the questions in metadata for the REPL to present.
	return &tools.ToolResult{
		Content: []tools.ContentBlock{{
			Type: "text",
			Text: formatQuestionsForDisplay(in.Questions),
		}},
		Metadata: map[string]any{
			"requires_user_input": true,
			"questions":           in.Questions,
			"structured":          true,
		},
	}, nil
}

// formatQuestionsForDisplay renders questions as readable text for terminal display.
func formatQuestionsForDisplay(questions []Question) string {
	var sb strings.Builder
	for i, q := range questions {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "[%s] %s\n", q.Header, q.Question)
		for j, opt := range q.Options {
			letter := string(rune('a' + j))
			fmt.Fprintf(&sb, "  %s) %s - %s\n", letter, opt.Label, opt.Description)
		}
	}
	return sb.String()
}

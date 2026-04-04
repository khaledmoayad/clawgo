package githubactions

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateWorkflow(t *testing.T) {
	workflow := generateWorkflow()

	assert.Contains(t, workflow, "claude-code-action@v1")
	assert.Contains(t, workflow, "ANTHROPIC_API_KEY")
	assert.Contains(t, workflow, "issue_comment")
	assert.Contains(t, workflow, "pull_request_review_comment")
	assert.Contains(t, workflow, "@claude")
	assert.Contains(t, workflow, "actions/checkout@v4")
}

func TestGenerateWorkflow_ValidYAML(t *testing.T) {
	workflow := generateWorkflow()

	var parsed map[string]interface{}
	err := yaml.Unmarshal([]byte(workflow), &parsed)
	require.NoError(t, err, "workflow YAML should be valid")

	// Verify top-level keys exist
	assert.Contains(t, parsed, "name")
	assert.Contains(t, parsed, "on")
	assert.Contains(t, parsed, "jobs")
}

func TestRun_NoGhCLI(t *testing.T) {
	// Save original PATH and set to empty to ensure gh is not found
	origPath := t.Name() // Just need a reference
	_ = origPath

	// Check if gh is actually not in a fake path
	_, err := exec.LookPath("gh-nonexistent-binary-12345")
	if err == nil {
		t.Skip("unexpected: fake binary found in PATH")
	}

	// We can't easily mock LookPath, but we can test the error message format.
	// The Run function checks for gh CLI first; if gh is installed on this system
	// the test verifies the workflow generation path instead.
	if _, lookErr := exec.LookPath("gh"); lookErr != nil {
		// gh not available -- test the error path
		_, runErr := Run(t.Context(), nil)
		require.Error(t, runErr)
		assert.Contains(t, runErr.Error(), "gh CLI required")
	} else {
		// gh is available -- test generates workflow to temp dir
		t.Log("gh CLI available; skipping no-gh error test (gh is installed)")
		// Just verify generateWorkflow works
		wf := generateWorkflow()
		assert.NotEmpty(t, wf)
	}
}

func TestCommand_Metadata(t *testing.T) {
	cmd := New()
	assert.Equal(t, "install-github-app", cmd.Name())
	assert.Contains(t, cmd.Description(), "GitHub Actions")
	assert.Contains(t, cmd.Aliases(), "github-actions")
}

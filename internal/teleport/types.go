package teleport

import "encoding/json"

const (
	// CCR_BYOC_BETA is the anthropic-beta header value for CCR BYOC features.
	CCR_BYOC_BETA = "ccr-byoc-2025-07-29"
)

// SessionStatus represents the status of a teleport session.
type SessionStatus string

const (
	// StatusRequiresAction means the session needs user input.
	StatusRequiresAction SessionStatus = "requires_action"
	// StatusRunning means the session is actively processing.
	StatusRunning SessionStatus = "running"
	// StatusIdle means the session is waiting.
	StatusIdle SessionStatus = "idle"
	// StatusArchived means the session has been archived.
	StatusArchived SessionStatus = "archived"
)

// GitSource represents a git repository source for a session context.
type GitSource struct {
	// Type is always "git_repository".
	Type string `json:"type"`
	// URL is the git repository URL.
	URL string `json:"url"`
	// Revision is the git revision (branch, tag, or commit).
	Revision string `json:"revision"`
	// AllowUnrestrictedGitPush permits unrestricted git push operations.
	AllowUnrestrictedGitPush bool `json:"allow_unrestricted_git_push"`
}

// KnowledgeBaseSource represents a knowledge base source for a session context.
type KnowledgeBaseSource struct {
	// Type is always "knowledge_base".
	Type string `json:"type"`
	// KnowledgeBaseID is the identifier of the knowledge base.
	KnowledgeBaseID string `json:"knowledge_base_id"`
}

// GithubPR represents a GitHub pull request reference.
type GithubPR struct {
	// Owner is the repository owner (user or org).
	Owner string `json:"owner"`
	// Repo is the repository name.
	Repo string `json:"repo"`
	// Number is the PR number.
	Number int `json:"number"`
}

// SessionContext contains the configuration context for a teleport session.
type SessionContext struct {
	// Sources is a list of source configurations (git repos, knowledge bases).
	Sources []json.RawMessage `json:"sources"`
	// Cwd is the working directory.
	Cwd string `json:"cwd"`
	// Outcomes is a list of outcome configurations.
	Outcomes []json.RawMessage `json:"outcomes"`
	// CustomSystemPrompt overrides the system prompt.
	CustomSystemPrompt string `json:"custom_system_prompt,omitempty"`
	// AppendSystemPrompt is appended to the default system prompt.
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`
	// Model overrides the model selection.
	Model *string `json:"model,omitempty"`
	// SeedBundleFileID is the file ID for a seed bundle.
	SeedBundleFileID string `json:"seed_bundle_file_id,omitempty"`
	// GithubPR links the session to a GitHub pull request.
	GithubPR *GithubPR `json:"github_pr,omitempty"`
	// ReuseOutcomeBranches reuses existing branches instead of creating new ones.
	ReuseOutcomeBranches bool `json:"reuse_outcome_branches,omitempty"`
}

// SessionResource represents a session as returned by the Teleport API.
// Matches the TS SessionResource type.
type SessionResource struct {
	// Type is always "session".
	Type string `json:"type"`
	// ID is the session identifier.
	ID string `json:"id"`
	// Title is the human-readable session title.
	Title *string `json:"title"`
	// SessionStatus is the current status.
	SessionStatus string `json:"session_status"`
	// EnvironmentID links to the environment.
	EnvironmentID string `json:"environment_id"`
	// CreatedAt is the ISO 8601 creation timestamp.
	CreatedAt string `json:"created_at"`
	// UpdatedAt is the ISO 8601 last update timestamp.
	UpdatedAt string `json:"updated_at"`
	// SessionContext contains source and outcome configuration.
	SessionContext SessionContext `json:"session_context"`
}

// ListSessionsResponse is the paginated response from GET /v1/sessions.
type ListSessionsResponse struct {
	// Data contains the session resources.
	Data []SessionResource `json:"data"`
	// HasMore indicates if more results are available.
	HasMore bool `json:"has_more"`
	// FirstID is the ID of the first item in this page.
	FirstID *string `json:"first_id"`
	// LastID is the ID of the last item in this page.
	LastID *string `json:"last_id"`
}

// OutcomeGitInfo describes git information within an outcome.
type OutcomeGitInfo struct {
	// Type is the git provider type (e.g., "github").
	Type string `json:"type"`
	// Repo is the repository identifier.
	Repo string `json:"repo"`
	// Branches lists the branches associated with this outcome.
	Branches []string `json:"branches"`
}

// GitRepositoryOutcome represents a git repository outcome from a session.
type GitRepositoryOutcome struct {
	// Type is always "git_repository".
	Type string `json:"type"`
	// GitInfo contains the git details for the outcome.
	GitInfo OutcomeGitInfo `json:"git_info"`
}

// ContentBlock represents a content block within a remote message.
type ContentBlock struct {
	// Type identifies the content block type (e.g., "text", "image").
	Type string `json:"type"`
	// Raw holds the full JSON content for extensibility.
	Raw json.RawMessage `json:"-"`
}

// SessionEvent represents an event sent to a remote session.
type SessionEvent struct {
	// UUID is the unique event identifier.
	UUID string `json:"uuid"`
	// SessionID links to the session.
	SessionID string `json:"session_id"`
	// Type is the event type (e.g., "user").
	Type string `json:"type"`
	// ParentToolUseID is the parent tool use, if any.
	ParentToolUseID *string `json:"parent_tool_use_id"`
	// Message contains the event message payload.
	Message SessionEventMessage `json:"message"`
}

// SessionEventMessage is the message payload within a session event.
type SessionEventMessage struct {
	// Role is the message role (e.g., "user").
	Role string `json:"role"`
	// Content is the message content (string or content blocks).
	Content interface{} `json:"content"`
}

// SendEventsRequest is the request body for POST /v1/sessions/{id}/events.
type SendEventsRequest struct {
	// Events is the list of events to send.
	Events []SessionEvent `json:"events"`
}

// UpdateSessionRequest is the request body for PATCH /v1/sessions/{id}.
type UpdateSessionRequest struct {
	// Title is the new title for the session.
	Title string `json:"title"`
}

package sdk

import (
	"os"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/session"
)

// SaveSession persists the QueryEngine's message history to a JSONL session file.
// It converts each api.Message to a session.TranscriptMessage and writes it via
// session.AppendTranscriptMessage. The session file path is derived from
// projectRoot using session.GetSessionDir.
func (e *QueryEngine) SaveSession() error {
	e.mu.Lock()
	msgs := make([]api.Message, len(e.messages))
	copy(msgs, e.messages)
	projectRoot := e.config.ProjectRoot
	sessionID := e.sessionID
	e.mu.Unlock()

	sessionPath := session.GetSessionPath(projectRoot, sessionID)

	ct := session.NewChainTracker()
	now := time.Now().UTC().Format(time.RFC3339)

	meta := session.SerializedMessage{
		SessionID: sessionID,
		Timestamp: now,
		Version:   "1.0.0",
	}

	for _, msg := range msgs {
		tm := session.TranscriptFromMessage(msg, ct, meta)
		if err := session.AppendTranscriptMessage(sessionPath, tm); err != nil {
			return err
		}
	}

	return nil
}

// LoadSDKSession reads a session file and returns the messages as an api.Message slice.
// Returns an empty slice (not an error) if the session file does not exist.
func LoadSDKSession(projectRoot, sessionID string) ([]api.Message, error) {
	sessionPath := session.GetSessionPath(projectRoot, sessionID)

	// Check if the file exists; return empty slice if not
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := session.LoadSession(sessionPath)
	if err != nil {
		return nil, err
	}

	return session.EntriesToMessages(entries), nil
}

// NewQueryEngineFromSession creates a QueryEngine pre-populated with messages
// from an existing session file. Useful for --resume and SDK session continuation.
// The engine's session ID is set to the provided sessionID so subsequent
// SaveSession calls append to the same file.
func NewQueryEngineFromSession(cfg QueryEngineConfig, projectRoot, sessionID string) (*QueryEngine, error) {
	messages, err := LoadSDKSession(projectRoot, sessionID)
	if err != nil {
		return nil, err
	}

	model := api.DefaultModel
	if cfg.Client != nil {
		model = cfg.Client.Model
	}

	return &QueryEngine{
		config:      cfg,
		messages:    messages,
		costTracker: cost.NewTracker(model),
		sessionID:   sessionID,
	}, nil
}

package uds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SessionKind identifies the type of Claude session.
type SessionKind string

const (
	// KindInteractive is a normal user-facing REPL session.
	KindInteractive SessionKind = "interactive"
	// KindBg is a background session (e.g., from Agent SDK).
	KindBg SessionKind = "bg"
	// KindDaemon is a daemon supervisor session.
	KindDaemon SessionKind = "daemon"
	// KindDaemonWorker is a worker spawned by the daemon.
	KindDaemonWorker SessionKind = "daemon-worker"
)

// SessionStatus indicates what a session is doing right now.
type SessionStatus string

const (
	// StatusBusy means the session is actively processing a query.
	StatusBusy SessionStatus = "busy"
	// StatusIdle means the session is waiting for user input.
	StatusIdle SessionStatus = "idle"
	// StatusWaiting means the session is waiting for an external event.
	StatusWaiting SessionStatus = "waiting"
)

// PeerInfo describes a running Claude session discovered via its PID file.
// Stored at ~/.claude/sessions/{pid}.json.
type PeerInfo struct {
	// PID is the operating system process ID.
	PID int `json:"pid"`
	// SessionID is the unique session identifier.
	SessionID string `json:"sessionId"`
	// Cwd is the working directory of the session.
	Cwd string `json:"cwd"`
	// StartedAt is the Unix timestamp (milliseconds) when the session started.
	StartedAt int64 `json:"startedAt"`
	// Kind is the session type (interactive, bg, daemon, daemon-worker).
	Kind SessionKind `json:"kind"`
	// Entrypoint describes how the session was launched (e.g., "cli", "sdk").
	Entrypoint string `json:"entrypoint"`
	// MessagingSocketPath is the UDS path for inter-session messaging.
	MessagingSocketPath string `json:"messagingSocketPath,omitempty"`
	// Name is an optional human-readable session name.
	Name string `json:"name,omitempty"`
	// LogPath is the path to the session's log file.
	LogPath string `json:"logPath,omitempty"`
	// Agent is the agent identifier if this is an agent session.
	Agent string `json:"agent,omitempty"`
}

// GetSessionsDir returns the path to the sessions directory within the
// given config directory. This is where PID files are stored.
func GetSessionsDir(configDir string) string {
	return filepath.Join(configDir, "sessions")
}

// RegisterSession writes a PID file for the current process to the sessions
// directory. The returned cleanup function removes the PID file and should
// be called when the session ends (typically via defer or shutdown hook).
func RegisterSession(configDir, sessionID, cwd string, kind SessionKind) (cleanup func(), err error) {
	sessionsDir := GetSessionsDir(configDir)
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("creating sessions dir: %w", err)
	}

	pid := os.Getpid()
	info := PeerInfo{
		PID:       pid,
		SessionID: sessionID,
		Cwd:       cwd,
		StartedAt: time.Now().UnixMilli(),
		Kind:      kind,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshalling peer info: %w", err)
	}

	pidFile := filepath.Join(sessionsDir, fmt.Sprintf("%d.json", pid))
	if err := os.WriteFile(pidFile, data, 0600); err != nil {
		return nil, fmt.Errorf("writing PID file: %w", err)
	}

	cleanup = func() {
		os.Remove(pidFile)
	}
	return cleanup, nil
}

// DiscoverPeers reads all PID files from the sessions directory, validates
// that each process is still alive, removes stale entries for dead processes,
// and returns the list of live peers.
func DiscoverPeers(configDir string) ([]PeerInfo, error) {
	sessionsDir := GetSessionsDir(configDir)
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions dir: %w", err)
	}

	var peers []PeerInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			// File may have been removed between ReadDir and ReadFile
			continue
		}

		var info PeerInfo
		if err := json.Unmarshal(data, &info); err != nil {
			// Corrupted file; remove it
			os.Remove(filePath)
			continue
		}

		// Validate the PID is still running
		if !isProcessAlive(info.PID) {
			// Stale PID file; clean it up
			os.Remove(filePath)
			continue
		}

		peers = append(peers, info)
	}

	return peers, nil
}

// IsBgSession checks the CLAUDE_CODE_SESSION_KIND environment variable
// to determine if the current process is a background session.
func IsBgSession() bool {
	return os.Getenv("CLAUDE_CODE_SESSION_KIND") == string(KindBg)
}

// isProcessAlive checks whether a process with the given PID is still running.
// Uses kill(pid, 0) which checks for the existence of a process without
// actually sending a signal.
func isProcessAlive(pid int) bool {
	// On Linux/macOS, sending signal 0 checks if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// pidFromFilename extracts a PID from a filename like "12345.json".
func pidFromFilename(name string) (int, bool) {
	base := strings.TrimSuffix(name, ".json")
	pid, err := strconv.Atoi(base)
	if err != nil {
		return 0, false
	}
	return pid, true
}

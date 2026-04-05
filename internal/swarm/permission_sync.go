// permission_sync.go implements synchronized permission prompts for agent swarms.
//
// When a swarm worker needs tool permission approval, it writes a request to the
// team's pending directory. The leader reads pending requests, presents them to
// the user, and writes resolutions. Workers poll the resolved directory for
// responses. This enables distributed permission management across independent
// worker goroutines.
//
// Directory structure:
//   ~/.claude/teams/{teamName}/permissions/pending/{requestId}.json
//   ~/.claude/teams/{teamName}/permissions/resolved/{requestId}.json
//   Lock file: pending/.lock
//
// Flow:
//   1. Worker calls WritePermissionRequest -> pending/{id}.json
//   2. Leader calls ReadPendingPermissions -> reads all pending
//   3. Leader calls ResolvePermission -> moves from pending/ to resolved/
//   4. Worker calls PollForResponse -> reads resolved/{id}.json
//   5. Worker calls DeleteResolvedPermission -> cleanup after processing
//   6. CleanupOldResolutions removes stale resolved files (default 1 hour)
package swarm

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/khaledmoayad/clawgo/internal/config"
)

// Permission request status constants.
const (
	PermissionStatusPending  = "pending"
	PermissionStatusApproved = "approved"
	PermissionStatusRejected = "rejected"
)

// Permission resolver identity constants.
const (
	ResolvedByWorker = "worker"
	ResolvedByLeader = "leader"
)

// DefaultMaxResolutionAge is the default maximum age for resolved permission
// files before cleanup (1 hour).
const DefaultMaxResolutionAge = time.Hour

// SwarmPermissionRequest represents a permission request from a worker to the
// leader. It is serialized as JSON and stored in the pending/resolved directories.
type SwarmPermissionRequest struct {
	ID                    string                 `json:"id"`
	WorkerID              string                 `json:"workerId"`
	WorkerName            string                 `json:"workerName"`
	WorkerColor           string                 `json:"workerColor,omitempty"`
	TeamName              string                 `json:"teamName"`
	ToolName              string                 `json:"toolName"`
	ToolUseID             string                 `json:"toolUseId"`
	Description           string                 `json:"description"`
	Input                 map[string]interface{} `json:"input"`
	PermissionSuggestions []interface{}          `json:"permissionSuggestions"`
	Status                string                 `json:"status"`
	ResolvedBy            string                 `json:"resolvedBy,omitempty"`
	ResolvedAt            int64                  `json:"resolvedAt,omitempty"`
	Feedback              string                 `json:"feedback,omitempty"`
	UpdatedInput          map[string]interface{} `json:"updatedInput,omitempty"`
	PermissionUpdates     []interface{}          `json:"permissionUpdates,omitempty"`
	CreatedAt             int64                  `json:"createdAt"`
}

// PermissionResolution contains the data needed to resolve a permission request.
type PermissionResolution struct {
	Decision          string                 `json:"decision"`
	ResolvedBy        string                 `json:"resolvedBy"`
	Feedback          string                 `json:"feedback,omitempty"`
	UpdatedInput      map[string]interface{} `json:"updatedInput,omitempty"`
	PermissionUpdates []interface{}          `json:"permissionUpdates,omitempty"`
}

// PermissionResponse is the simplified response format returned by PollForResponse.
// Workers use this to check the outcome of their permission request.
type PermissionResponse struct {
	RequestID         string                 `json:"requestId"`
	Decision          string                 `json:"decision"`
	Timestamp         string                 `json:"timestamp"`
	Feedback          string                 `json:"feedback,omitempty"`
	UpdatedInput      map[string]interface{} `json:"updatedInput,omitempty"`
	PermissionUpdates []interface{}          `json:"permissionUpdates,omitempty"`
}

// CreatePermissionRequestParams holds the parameters for CreatePermissionRequest.
type CreatePermissionRequestParams struct {
	ToolName              string
	ToolUseID             string
	Input                 map[string]interface{}
	Description           string
	PermissionSuggestions []interface{}
	TeamName              string
	WorkerID              string
	WorkerName            string
	WorkerColor           string
}

// --- Path helpers ---

// getTeamDir returns the base directory for a team: ~/.claude/teams/{teamName}.
func getTeamDir(teamName string) string {
	return filepath.Join(config.ConfigDir(), "teams", teamName)
}

// getPermissionDir returns the permissions directory for a team.
func getPermissionDir(teamName string) string {
	return filepath.Join(getTeamDir(teamName), "permissions")
}

// getPendingDir returns the pending permissions directory for a team.
func getPendingDir(teamName string) string {
	return filepath.Join(getPermissionDir(teamName), "pending")
}

// getResolvedDir returns the resolved permissions directory for a team.
func getResolvedDir(teamName string) string {
	return filepath.Join(getPermissionDir(teamName), "resolved")
}

// ensurePermissionDirs creates the permissions directory structure if it does
// not already exist.
func ensurePermissionDirs(teamName string) error {
	for _, dir := range []string{
		getPermissionDir(teamName),
		getPendingDir(teamName),
		getResolvedDir(teamName),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create permission dir %s: %w", dir, err)
		}
	}
	return nil
}

// --- File locking ---

// fileLock holds an open lock file descriptor. Call Release to unlock.
type fileLock struct {
	f *os.File
}

// acquireLock acquires an exclusive flock on the .lock file in the pending
// directory. The caller must call release() when done.
func acquireLock(teamName string) (*fileLock, error) {
	lockPath := filepath.Join(getPendingDir(teamName), ".lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("flock: %w", err)
	}

	return &fileLock{f: f}, nil
}

// Release releases the file lock and closes the file.
func (l *fileLock) Release() error {
	if l.f == nil {
		return nil
	}
	// Unlock then close. Ignore unlock errors since close will release anyway.
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}

// --- Request ID generation ---

// GenerateRequestID returns a unique permission request ID matching the TS
// format: perm-{timestampMs}-{random7alphanumeric}.
func GenerateRequestID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	// Hex encode gives 8 chars; take first 7 to match TS Math.random().toString(36).substring(2,9)
	random := hex.EncodeToString(b)[:7]
	return fmt.Sprintf("perm-%d-%s", time.Now().UnixMilli(), random)
}

// --- Request creation ---

// CreatePermissionRequest builds a new SwarmPermissionRequest in "pending" status.
func CreatePermissionRequest(params CreatePermissionRequestParams) SwarmPermissionRequest {
	suggestions := params.PermissionSuggestions
	if suggestions == nil {
		suggestions = []interface{}{}
	}
	input := params.Input
	if input == nil {
		input = map[string]interface{}{}
	}

	return SwarmPermissionRequest{
		ID:                    GenerateRequestID(),
		WorkerID:              params.WorkerID,
		WorkerName:            params.WorkerName,
		WorkerColor:           params.WorkerColor,
		TeamName:              params.TeamName,
		ToolName:              params.ToolName,
		ToolUseID:             params.ToolUseID,
		Description:           params.Description,
		Input:                 input,
		PermissionSuggestions: suggestions,
		Status:                PermissionStatusPending,
		CreatedAt:             time.Now().UnixMilli(),
	}
}

// --- Write / Read / Resolve operations ---

// WritePermissionRequest writes a permission request to the pending directory
// with file locking to prevent race conditions.
func WritePermissionRequest(request SwarmPermissionRequest) error {
	if err := ensurePermissionDirs(request.TeamName); err != nil {
		return err
	}

	lock, err := acquireLock(request.TeamName)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Release()

	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	path := filepath.Join(getPendingDir(request.TeamName), request.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write pending request: %w", err)
	}

	return nil
}

// ReadPendingPermissions reads all pending permission requests for a team,
// sorted by CreatedAt ascending (oldest first). Malformed files are skipped.
func ReadPendingPermissions(teamName string) ([]SwarmPermissionRequest, error) {
	pendingDir := getPendingDir(teamName)

	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pending dir: %w", err)
	}

	var requests []SwarmPermissionRequest
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || name == ".lock" {
			continue
		}

		path := filepath.Join(pendingDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			// Skip files we cannot read (race condition: deleted between readdir and read)
			continue
		}

		var req SwarmPermissionRequest
		if err := json.Unmarshal(data, &req); err != nil {
			// Skip malformed entries
			continue
		}
		requests = append(requests, req)
	}

	// Sort by CreatedAt ascending
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].CreatedAt < requests[j].CreatedAt
	})

	return requests, nil
}

// ReadResolvedPermission reads a resolved permission request by ID.
// Returns nil, nil if the file does not exist (not yet resolved).
func ReadResolvedPermission(requestID, teamName string) (*SwarmPermissionRequest, error) {
	path := filepath.Join(getResolvedDir(teamName), requestID+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read resolved permission: %w", err)
	}

	var req SwarmPermissionRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal resolved permission: %w", err)
	}

	return &req, nil
}

// ResolvePermission atomically moves a request from pending/ to resolved/.
// It reads the pending request, updates it with resolution data, writes to
// resolved/, and deletes the pending file. Returns false if the pending
// request was not found.
func ResolvePermission(requestID string, resolution PermissionResolution, teamName string) (bool, error) {
	if err := ensurePermissionDirs(teamName); err != nil {
		return false, err
	}

	lock, err := acquireLock(teamName)
	if err != nil {
		return false, fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Release()

	// Read the pending request
	pendingPath := filepath.Join(getPendingDir(teamName), requestID+".json")
	data, err := os.ReadFile(pendingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read pending request: %w", err)
	}

	var request SwarmPermissionRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return false, fmt.Errorf("unmarshal pending request: %w", err)
	}

	// Update with resolution data
	if resolution.Decision == PermissionStatusApproved {
		request.Status = PermissionStatusApproved
	} else {
		request.Status = PermissionStatusRejected
	}
	request.ResolvedBy = resolution.ResolvedBy
	request.ResolvedAt = time.Now().UnixMilli()
	request.Feedback = resolution.Feedback
	request.UpdatedInput = resolution.UpdatedInput
	request.PermissionUpdates = resolution.PermissionUpdates

	// Write to resolved directory
	resolvedData, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal resolved request: %w", err)
	}

	resolvedPath := filepath.Join(getResolvedDir(teamName), requestID+".json")
	if err := os.WriteFile(resolvedPath, resolvedData, 0o644); err != nil {
		return false, fmt.Errorf("write resolved request: %w", err)
	}

	// Delete from pending directory
	if err := os.Remove(pendingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove pending request: %w", err)
	}

	return true, nil
}

// --- Worker-side convenience functions ---

// PollForResponse checks if a permission request has been resolved and returns
// a simplified PermissionResponse. Returns nil, nil if not yet resolved.
// The decision field maps "approved" -> "approved", "rejected" -> "denied" to
// match the TS PermissionResponse type.
func PollForResponse(requestID, teamName string) (*PermissionResponse, error) {
	resolved, err := ReadResolvedPermission(requestID, teamName)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, nil
	}

	decision := "denied"
	if resolved.Status == PermissionStatusApproved {
		decision = "approved"
	}

	var timestamp string
	if resolved.ResolvedAt > 0 {
		timestamp = time.UnixMilli(resolved.ResolvedAt).UTC().Format(time.RFC3339)
	} else {
		timestamp = time.UnixMilli(resolved.CreatedAt).UTC().Format(time.RFC3339)
	}

	return &PermissionResponse{
		RequestID:         resolved.ID,
		Decision:          decision,
		Timestamp:         timestamp,
		Feedback:          resolved.Feedback,
		UpdatedInput:      resolved.UpdatedInput,
		PermissionUpdates: resolved.PermissionUpdates,
	}, nil
}

// DeleteResolvedPermission removes a resolved permission file after the worker
// has processed it.
func DeleteResolvedPermission(requestID, teamName string) (bool, error) {
	path := filepath.Join(getResolvedDir(teamName), requestID+".json")
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("delete resolved permission: %w", err)
	}
	return true, nil
}

// --- Cleanup ---

// CleanupOldResolutions removes resolved permission files older than maxAge.
// Returns the count of files cleaned up. Uses resolvedAt timestamp if available,
// falling back to createdAt.
func CleanupOldResolutions(teamName string, maxAge time.Duration) (int, error) {
	resolvedDir := getResolvedDir(teamName)

	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read resolved dir: %w", err)
	}

	now := time.Now().UnixMilli()
	cleaned := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(resolvedDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			// If we cannot read it, try to clean it up anyway
			if removeErr := os.Remove(path); removeErr == nil {
				cleaned++
			}
			continue
		}

		var req SwarmPermissionRequest
		if err := json.Unmarshal(data, &req); err != nil {
			// Malformed file -- clean it up
			if removeErr := os.Remove(path); removeErr == nil {
				cleaned++
			}
			continue
		}

		// Use resolvedAt if available, fall back to createdAt
		resolvedAt := req.ResolvedAt
		if resolvedAt == 0 {
			resolvedAt = req.CreatedAt
		}

		// Check age -- use >= to handle maxAge=0 (clean everything)
		if now-resolvedAt >= maxAge.Milliseconds() {
			if removeErr := os.Remove(path); removeErr == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// --- Role detection ---

// IsTeamLeader checks if the current agent is the team leader.
// Team leaders either have no agent ID set or have ID "team-lead".
func IsTeamLeader(teamName string) bool {
	if teamName == "" {
		return false
	}
	agentID := os.Getenv("CLAUDE_CODE_AGENT_ID")
	return agentID == "" || agentID == "team-lead"
}

// IsSwarmWorker checks if the current agent is a worker in a swarm.
// A worker has both a team name and an agent ID set, and is not the leader.
func IsSwarmWorker() bool {
	teamName := os.Getenv("CLAUDE_CODE_TEAM_NAME")
	agentID := os.Getenv("CLAUDE_CODE_AGENT_ID")
	return teamName != "" && agentID != "" && !IsTeamLeader(teamName)
}

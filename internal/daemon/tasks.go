package daemon

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CronTask represents a scheduled cron task matching the TS CronTask type.
// Tasks store a prompt string (not a shell command) and fire by calling onFire(prompt).
type CronTask struct {
	ID         string `json:"id"`                   // 8-hex-char UUID slice
	Cron       string `json:"cron"`                 // 5-field cron expression
	Prompt     string `json:"prompt"`               // prompt to inject into query loop (NOT a command)
	CreatedAt  int64  `json:"createdAt"`            // epoch milliseconds
	LastFiredAt *int64 `json:"lastFiredAt,omitempty"` // epoch ms of most recent fire
	Recurring  *bool  `json:"recurring,omitempty"`
	Permanent  *bool  `json:"permanent,omitempty"`
	Durable    *bool  `json:"-"` // runtime-only, never persisted
	AgentID    string `json:"-"` // runtime-only, for in-process teammate routing
}

// CronFile is the on-disk format for scheduled tasks.
// File path: .claude/scheduled_tasks.json
type CronFile struct {
	Tasks []CronTask `json:"tasks"`
}

// CronJitterConfig matches the TS CronJitterConfig defaults.
type CronJitterConfig struct {
	RecurringFrac    float64 // 0.1
	RecurringCapMs   int64   // 15*60*1000 = 900000
	OneShotMaxMs     int64   // 90*1000 = 90000
	OneShotFloorMs   int64   // 0
	OneShotMinuteMod int64   // 30
	RecurringMaxAgeMs int64  // 7*24*60*60*1000 = 604800000
}

// DefaultCronJitterConfig returns the default jitter config matching TS.
func DefaultCronJitterConfig() CronJitterConfig {
	return CronJitterConfig{
		RecurringFrac:     0.1,
		RecurringCapMs:    15 * 60 * 1000,
		OneShotMaxMs:      90 * 1000,
		OneShotFloorMs:    0,
		OneShotMinuteMod:  30,
		RecurringMaxAgeMs: 7 * 24 * 60 * 60 * 1000,
	}
}

// sessionTasks stores non-durable (session-scoped) tasks in memory.
var (
	sessionTasks   []CronTask
	sessionTasksMu sync.Mutex
)

// GetCronFilePath returns the path to the scheduled tasks JSON file.
func GetCronFilePath(dir string) string {
	return filepath.Join(dir, ".claude", "scheduled_tasks.json")
}

// ReadCronTasks reads scheduled tasks from the project directory.
// Returns an empty slice (not an error) if the file does not exist.
func ReadCronTasks(dir string) ([]CronTask, error) {
	path := GetCronFilePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []CronTask{}, nil
		}
		return nil, fmt.Errorf("read cron tasks file: %w", err)
	}

	var cf CronFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("unmarshal cron tasks: %w", err)
	}

	// Validate each task, skip malformed entries
	valid := make([]CronTask, 0, len(cf.Tasks))
	for _, t := range cf.Tasks {
		if t.ID == "" || t.Cron == "" || t.Prompt == "" || t.CreatedAt == 0 {
			log.Printf("daemon: skipping malformed cron task: id=%q cron=%q prompt=%q createdAt=%d", t.ID, t.Cron, t.Prompt, t.CreatedAt)
			continue
		}
		if _, err := parseCronExpression(t.Cron); err != nil {
			log.Printf("daemon: skipping cron task %s with invalid cron expression %q: %v", t.ID, t.Cron, err)
			continue
		}
		valid = append(valid, t)
	}

	return valid, nil
}

// WriteCronTasks writes scheduled tasks to the project directory.
// Runtime-only fields (Durable, AgentID) are automatically excluded via json:"-" tags.
func WriteCronTasks(tasks []CronTask, dir string) error {
	path := GetCronFilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	cf := CronFile{Tasks: tasks}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cron tasks: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// generateCronID generates an 8-char hex ID from random bytes (matching TS UUID slice).
func generateCronID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return fmt.Sprintf("%08x", b)
}

// AddCronTask creates a new cron task and adds it to file or session store.
// Returns the generated task ID.
func AddCronTask(cron, prompt string, recurring, durable bool, dir string) (string, error) {
	id := generateCronID()
	now := time.Now().UnixMilli()

	task := CronTask{
		ID:        id,
		Cron:      cron,
		Prompt:    prompt,
		CreatedAt: now,
	}
	if recurring {
		r := true
		task.Recurring = &r
	}

	if !durable {
		// Session-scoped: store in memory only
		sessionTasksMu.Lock()
		sessionTasks = append(sessionTasks, task)
		sessionTasksMu.Unlock()
		return id, nil
	}

	// Durable: read-modify-write file
	d := true
	task.Durable = &d

	existing, err := ReadCronTasks(dir)
	if err != nil {
		return "", fmt.Errorf("read existing tasks: %w", err)
	}

	existing = append(existing, task)
	if err := WriteCronTasks(existing, dir); err != nil {
		return "", fmt.Errorf("write tasks: %w", err)
	}

	return id, nil
}

// RemoveCronTasks removes tasks with the given IDs from the file.
func RemoveCronTasks(ids []string, dir string) error {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	tasks, err := ReadCronTasks(dir)
	if err != nil {
		return fmt.Errorf("read tasks: %w", err)
	}

	filtered := make([]CronTask, 0, len(tasks))
	for _, t := range tasks {
		if !idSet[t.ID] {
			filtered = append(filtered, t)
		}
	}

	return WriteCronTasks(filtered, dir)
}

// MarkCronTasksFired sets lastFiredAt on matching task IDs and writes back.
func MarkCronTasksFired(ids []string, firedAt int64, dir string) error {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	tasks, err := ReadCronTasks(dir)
	if err != nil {
		return fmt.Errorf("read tasks: %w", err)
	}

	for i := range tasks {
		if idSet[tasks[i].ID] {
			ts := firedAt
			tasks[i].LastFiredAt = &ts
		}
	}

	return WriteCronTasks(tasks, dir)
}

// GetSessionTasks returns a copy of the in-memory session-scoped tasks.
func GetSessionTasks() []CronTask {
	sessionTasksMu.Lock()
	defer sessionTasksMu.Unlock()
	result := make([]CronTask, len(sessionTasks))
	copy(result, sessionTasks)
	return result
}

// RemoveSessionTask removes a single session-scoped task by ID.
func RemoveSessionTask(id string) {
	sessionTasksMu.Lock()
	defer sessionTasksMu.Unlock()
	for i, t := range sessionTasks {
		if t.ID == id {
			sessionTasks = append(sessionTasks[:i], sessionTasks[i+1:]...)
			return
		}
	}
}

// ClearSessionTasks removes all session-scoped tasks.
func ClearSessionTasks() {
	sessionTasksMu.Lock()
	sessionTasks = nil
	sessionTasksMu.Unlock()
}

// cronFields holds parsed allowed values for each of the 5 cron fields.
type cronFields struct {
	minutes  map[int]bool
	hours    map[int]bool
	doms     map[int]bool
	months   map[int]bool
	dows     map[int]bool
}

// parseCronExpression parses a 5-field cron expression into allowed value sets.
func parseCronExpression(expr string) (*cronFields, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	minutes := parseCronField(fields[0], 0, 59)
	hours := parseCronField(fields[1], 0, 23)
	doms := parseCronField(fields[2], 1, 31)
	months := parseCronField(fields[3], 1, 12)
	dows := parseCronField(fields[4], 0, 6)

	return &cronFields{
		minutes: minutes,
		hours:   hours,
		doms:    doms,
		months:  months,
		dows:    dows,
	}, nil
}

// NextCronRunMs computes the next fire time (epoch ms) after fromMs for the given cron expression.
// Returns nil if invalid or no match in 366 days.
func NextCronRunMs(cron string, fromMs int64) *int64 {
	cf, err := parseCronExpression(cron)
	if err != nil {
		return nil
	}

	from := time.UnixMilli(fromMs)
	// Start from the next minute after from
	candidate := from.Truncate(time.Minute).Add(time.Minute)
	maxTime := from.Add(366 * 24 * time.Hour)

	for candidate.Before(maxTime) {
		if cf.months[int(candidate.Month())] &&
			cf.doms[candidate.Day()] &&
			cf.dows[int(candidate.Weekday())] &&
			cf.hours[candidate.Hour()] &&
			cf.minutes[candidate.Minute()] {
			ms := candidate.UnixMilli()
			return &ms
		}
		candidate = candidate.Add(time.Minute)
	}

	return nil
}

// jitterFractionFromID derives a deterministic jitter fraction [0, 1) from the task ID.
// Parses the first 8 hex chars as uint32 and divides by 0x100000000.
func jitterFractionFromID(taskID string) float64 {
	// Use first 8 hex chars of ID
	hexStr := taskID
	if len(hexStr) > 8 {
		hexStr = hexStr[:8]
	}
	val, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0
	}
	return float64(val) / float64(0x100000000)
}

// JitteredNextCronRunMs computes the next fire time with proportional jitter for recurring tasks.
// The jitter adds a forward delay: frac * min(interval * RecurringFrac, RecurringCapMs).
func JitteredNextCronRunMs(cron string, fromMs int64, taskID string, cfg CronJitterConfig) *int64 {
	nextMs := NextCronRunMs(cron, fromMs)
	if nextMs == nil {
		return nil
	}

	// Compute the interval (next - from) to determine jitter magnitude
	intervalMs := *nextMs - fromMs
	jitterMaxMs := math.Min(float64(intervalMs)*cfg.RecurringFrac, float64(cfg.RecurringCapMs))
	frac := jitterFractionFromID(taskID)
	jitterMs := int64(frac * jitterMaxMs)

	result := *nextMs + jitterMs
	return &result
}

// OneShotJitteredNextCronRunMs computes the next fire time with backward jitter for one-shot tasks.
// On minute boundaries matching OneShotMinuteMod, the jitter subtracts time before the fire point.
func OneShotJitteredNextCronRunMs(cron string, fromMs int64, taskID string, cfg CronJitterConfig) *int64 {
	nextMs := NextCronRunMs(cron, fromMs)
	if nextMs == nil {
		return nil
	}

	// Check if the next fire time is on a minute boundary matching OneShotMinuteMod
	nextTime := time.UnixMilli(*nextMs)
	minute := int64(nextTime.Minute())
	if cfg.OneShotMinuteMod > 0 && minute%cfg.OneShotMinuteMod == 0 {
		// Apply backward jitter: subtract a random amount up to OneShotMaxMs
		frac := jitterFractionFromID(taskID)
		jitterMs := int64(frac * float64(cfg.OneShotMaxMs-cfg.OneShotFloorMs))
		jitterMs += cfg.OneShotFloorMs
		result := *nextMs - jitterMs
		// Don't go before fromMs
		if result < fromMs {
			result = fromMs
		}
		return &result
	}

	return nextMs
}

// FindMissedTasks returns tasks whose next fire time from createdAt is in the past.
func FindMissedTasks(tasks []CronTask, nowMs int64) []CronTask {
	var missed []CronTask
	for _, t := range tasks {
		// Only consider non-recurring one-shot tasks
		if t.Recurring != nil && *t.Recurring {
			continue
		}
		nextMs := NextCronRunMs(t.Cron, t.CreatedAt)
		if nextMs != nil && *nextMs <= nowMs {
			missed = append(missed, t)
		}
	}
	return missed
}

// IsRecurringTaskAged returns true if a recurring (non-permanent) task has exceeded maxAgeMs.
func IsRecurringTaskAged(t CronTask, nowMs int64, maxAgeMs int64) bool {
	isRecurring := t.Recurring != nil && *t.Recurring
	isPermanent := t.Permanent != nil && *t.Permanent
	if !isRecurring || isPermanent {
		return false
	}
	return nowMs-t.CreatedAt >= maxAgeMs
}

// parseCronField parses a single cron field into a set of allowed values.
// Supports: *, */N, N, N-M, N-M/S, comma-separated combinations.
func parseCronField(field string, min, max int) map[int]bool {
	result := make(map[int]bool)

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// Handle */N (step from min)
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				continue
			}
			for i := min; i <= max; i += step {
				result[i] = true
			}
			continue
		}

		// Handle *
		if part == "*" {
			for i := min; i <= max; i++ {
				result[i] = true
			}
			continue
		}

		// Handle N-M or N-M/S
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "/", 2)
			bounds := strings.SplitN(rangeParts[0], "-", 2)
			if len(bounds) != 2 {
				continue
			}
			low, err1 := strconv.Atoi(bounds[0])
			high, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil {
				continue
			}
			step := 1
			if len(rangeParts) == 2 {
				s, err := strconv.Atoi(rangeParts[1])
				if err == nil && s > 0 {
					step = s
				}
			}
			for i := low; i <= high; i += step {
				if i >= min && i <= max {
					result[i] = true
				}
			}
			continue
		}

		// Handle plain number
		val, err := strconv.Atoi(part)
		if err == nil && val >= min && val <= max {
			result[val] = true
		}
	}

	// If nothing was parsed, allow everything (fallback for invalid fields)
	if len(result) == 0 {
		for i := min; i <= max; i++ {
			result[i] = true
		}
	}

	return result
}

// HumanReadableSchedule converts a 5-field cron expression to a human-readable string.
// Examples: "*/5 * * * *" -> "Every 5 minutes", "0 9 * * 1" -> "At 09:00 on Monday".
func HumanReadableSchedule(cron string) string {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return cron
	}
	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Every minute
	if minute == "*" && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		return "Every minute"
	}

	// */N patterns
	if strings.HasPrefix(minute, "*/") && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		return "Every " + strings.TrimPrefix(minute, "*/") + " minutes"
	}
	if minute == "0" && strings.HasPrefix(hour, "*/") && dom == "*" && month == "*" && dow == "*" {
		return "Every " + strings.TrimPrefix(hour, "*/") + " hours"
	}

	// Daily at specific time
	if minute != "*" && hour != "*" && dom == "*" && month == "*" && dow == "*" &&
		!strings.Contains(minute, "/") && !strings.Contains(hour, "/") &&
		!strings.Contains(minute, "-") && !strings.Contains(hour, "-") &&
		!strings.Contains(minute, ",") && !strings.Contains(hour, ",") {
		return fmt.Sprintf("Daily at %s:%s", zeroPad(hour), zeroPad(minute))
	}

	// Specific day of week
	if minute != "*" && hour != "*" && dom == "*" && month == "*" && dow != "*" &&
		!strings.Contains(minute, "/") && !strings.Contains(hour, "/") &&
		!strings.Contains(dow, "/") && !strings.Contains(dow, "-") {
		dayStr := dowName(dow)
		return fmt.Sprintf("At %s:%s on %s", zeroPad(hour), zeroPad(minute), dayStr)
	}

	// Specific day of month
	if minute != "*" && hour != "*" && dom != "*" && month == "*" && dow == "*" &&
		!strings.Contains(minute, "/") && !strings.Contains(hour, "/") &&
		!strings.Contains(dom, "/") && !strings.Contains(dom, "-") {
		return fmt.Sprintf("At %s:%s on day %s of every month", zeroPad(hour), zeroPad(minute), dom)
	}

	// Hourly at minute N
	if minute != "*" && hour == "*" && dom == "*" && month == "*" && dow == "*" &&
		!strings.Contains(minute, "/") && !strings.Contains(minute, "-") &&
		!strings.Contains(minute, ",") {
		return fmt.Sprintf("Hourly at :%s", zeroPad(minute))
	}

	// Fallback: return the raw expression
	return cron
}

// zeroPad adds a leading zero for single-digit time components.
func zeroPad(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

// dowName converts a cron day-of-week number or comma list to a readable name.
func dowName(dow string) string {
	names := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
		"4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday",
	}
	// Handle comma-separated days
	parts := strings.Split(dow, ",")
	var dayNames []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if n, ok := names[p]; ok {
			dayNames = append(dayNames, n)
		} else {
			dayNames = append(dayNames, p)
		}
	}
	return strings.Join(dayNames, ", ")
}

// MaxCronJobs is the maximum number of concurrent cron jobs allowed (matching TS MAX_JOBS).
const MaxCronJobs = 50

// CountAllTasks returns the total count of file-backed + session-scoped tasks.
func CountAllTasks(dir string) (int, error) {
	fileTasks, err := ReadCronTasks(dir)
	if err != nil {
		return 0, err
	}
	sessionTasks := GetSessionTasks()
	return len(fileTasks) + len(sessionTasks), nil
}

// CalculateNextRun computes the next execution time after from based on the cron expression.
// This is used internally -- prefer NextCronRunMs for the epoch-ms interface.
func CalculateNextRun(cronExpr string, from time.Time) time.Time {
	ms := from.UnixMilli()
	nextMs := NextCronRunMs(cronExpr, ms)
	if nextMs == nil {
		return from.Add(366 * 24 * time.Hour)
	}
	return time.UnixMilli(*nextMs)
}

// NowMs returns the current time in epoch milliseconds.
func NowMs() int64 {
	return time.Now().UnixMilli()
}

// int64ToBytes converts a uint64 to a byte slice for hashing.
func int64ToBytes(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

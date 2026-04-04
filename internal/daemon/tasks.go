package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const tasksFile = "scheduled_tasks.json"

// Task represents a scheduled cron task.
type Task struct {
	ID         string     `json:"id"`
	CronExpr   string     `json:"cron_expr"`   // 5-field cron: min hour dom mon dow
	Command    string     `json:"command"`      // command to execute
	ProjectDir string     `json:"project_dir"`  // working directory
	NextRun    time.Time  `json:"next_run"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	Enabled    bool       `json:"enabled"`
}

// LoadTasks reads scheduled tasks from the config directory.
// Returns an empty slice (not an error) if the file does not exist.
func LoadTasks(configDir string) ([]Task, error) {
	path := filepath.Join(configDir, tasksFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Task{}, nil
		}
		return nil, fmt.Errorf("read tasks file: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}
	return tasks, nil
}

// SaveTasks writes scheduled tasks to the config directory.
func SaveTasks(configDir string, tasks []Task) error {
	path := filepath.Join(configDir, tasksFile)
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// IsDue returns true if the task should be executed at or before the given time.
func (t *Task) IsDue(now time.Time) bool {
	return !now.Before(t.NextRun)
}

// CalculateNextRun computes the next execution time after `from` based on the cron expression.
// Supports 5-field cron: minute hour day-of-month month day-of-week.
// Uses simple minute-by-minute iteration (max 1 year lookahead).
func (t *Task) CalculateNextRun(from time.Time) time.Time {
	fields := strings.Fields(t.CronExpr)
	if len(fields) != 5 {
		// Invalid cron expression -- schedule far future
		return from.Add(365 * 24 * time.Hour)
	}

	// Parse each cron field into a set of allowed values
	minutes := parseCronField(fields[0], 0, 59)
	hours := parseCronField(fields[1], 0, 23)
	doms := parseCronField(fields[2], 1, 31)
	months := parseCronField(fields[3], 1, 12)
	dows := parseCronField(fields[4], 0, 6)

	// Start from the next minute after `from`
	candidate := from.Truncate(time.Minute).Add(time.Minute)
	maxTime := from.Add(366 * 24 * time.Hour)

	for candidate.Before(maxTime) {
		if months[int(candidate.Month())] &&
			doms[candidate.Day()] &&
			dows[int(candidate.Weekday())] &&
			hours[candidate.Hour()] &&
			minutes[candidate.Minute()] {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}

	// Fallback -- should not reach here for valid cron expressions
	return maxTime
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

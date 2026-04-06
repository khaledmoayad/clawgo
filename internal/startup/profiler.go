// Package startup implements the ordered initialization sequence for ClawGo.
// It mirrors the TypeScript entrypoints/init.ts pattern: load config, MDM,
// env vars, graceful shutdown, platform detection, and background tasks
// in strict dependency order with optional profiling.
package startup

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Profiler records named checkpoints for startup timing analysis.
// When enabled=false, Checkpoint is a no-op for zero overhead.
type Profiler struct {
	mu          sync.Mutex
	start       time.Time
	checkpoints []checkpoint
	enabled     bool
}

type checkpoint struct {
	name string
	at   time.Duration // relative to start
}

// NewProfiler creates a startup profiler.
// If enabled=false, Checkpoint calls are no-ops.
func NewProfiler(enabled bool) *Profiler {
	return &Profiler{
		start:   time.Now(),
		enabled: enabled,
	}
}

// Checkpoint records a named timing point.
// No-op if the profiler is disabled.
func (p *Profiler) Checkpoint(name string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkpoints = append(p.checkpoints, checkpoint{
		name: name,
		at:   time.Since(p.start),
	})
}

// Report returns a formatted string of all checkpoint timings.
// Returns an empty string if the profiler is disabled or no checkpoints exist.
func (p *Profiler) Report() string {
	if !p.enabled {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.checkpoints) == 0 {
		return ""
	}

	var b strings.Builder
	for i, cp := range p.checkpoints {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("%-25s %s", cp.name, cp.at.Round(time.Microsecond)))
	}
	return b.String()
}

// TotalDuration returns the time elapsed since profiler creation.
func (p *Profiler) TotalDuration() time.Duration {
	return time.Since(p.start)
}

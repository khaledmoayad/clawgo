package startup

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Profiler tests
// ---------------------------------------------------------------------------

func TestProfilerCheckpointRecordsTimestamps(t *testing.T) {
	p := NewProfiler(true)

	p.Checkpoint("step_a")
	time.Sleep(1 * time.Millisecond) // ensure non-zero delta
	p.Checkpoint("step_b")

	report := p.Report()
	if !strings.Contains(report, "step_a") {
		t.Errorf("report missing step_a: %s", report)
	}
	if !strings.Contains(report, "step_b") {
		t.Errorf("report missing step_b: %s", report)
	}
}

func TestProfilerDisabledIsNoOp(t *testing.T) {
	p := NewProfiler(false)

	p.Checkpoint("ignored")

	report := p.Report()
	if report != "" {
		t.Errorf("disabled profiler should produce empty report, got: %s", report)
	}
}

func TestProfilerTotalDuration(t *testing.T) {
	p := NewProfiler(true)
	time.Sleep(1 * time.Millisecond)
	d := p.TotalDuration()
	if d <= 0 {
		t.Errorf("TotalDuration should be > 0, got %v", d)
	}
}

func TestProfilerReportFormattedLines(t *testing.T) {
	p := NewProfiler(true)
	p.Checkpoint("alpha")
	p.Checkpoint("beta")

	report := p.Report()
	lines := strings.Split(strings.TrimSpace(report), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines in report, got %d: %s", len(lines), report)
	}
	// Each line should contain the checkpoint name and a duration
	for _, line := range lines {
		if !strings.Contains(line, "alpha") && !strings.Contains(line, "beta") {
			t.Errorf("unexpected line in report: %s", line)
		}
	}
}

// ---------------------------------------------------------------------------
// RunInitSequence tests
// ---------------------------------------------------------------------------

func TestRunInitSequenceReturnsAllPhases(t *testing.T) {
	result, err := RunInitSequence(context.Background(), InitOptions{
		ProfileStartup: true,
		WorkingDir:     t.TempDir(),
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("RunInitSequence error: %v", err)
	}

	report := result.Profiler.Report()
	expectedCheckpoints := []string{
		"config_loaded",
		"mdm_loaded",
		"env_applied",
		"shutdown_setup",
		"platform_detected",
		"background_started",
	}
	for _, cp := range expectedCheckpoints {
		if !strings.Contains(report, cp) {
			t.Errorf("profiler report missing checkpoint %q:\n%s", cp, report)
		}
	}
}

func TestRunInitSequenceConfigBeforeMDM(t *testing.T) {
	result, err := RunInitSequence(context.Background(), InitOptions{
		ProfileStartup: true,
		WorkingDir:     t.TempDir(),
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("RunInitSequence error: %v", err)
	}

	report := result.Profiler.Report()
	configIdx := strings.Index(report, "config_loaded")
	mdmIdx := strings.Index(report, "mdm_loaded")
	if configIdx < 0 || mdmIdx < 0 {
		t.Fatalf("missing checkpoints in report:\n%s", report)
	}
	if configIdx >= mdmIdx {
		t.Errorf("config_loaded should appear before mdm_loaded in report")
	}
}

func TestRunInitSequenceWithProfileDisabled(t *testing.T) {
	result, err := RunInitSequence(context.Background(), InitOptions{
		ProfileStartup: false,
		WorkingDir:     t.TempDir(),
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("RunInitSequence error: %v", err)
	}

	report := result.Profiler.Report()
	if report != "" {
		t.Errorf("expected empty report when profiling disabled, got: %s", report)
	}
}

func TestRunInitSequenceIdempotent(t *testing.T) {
	opts := InitOptions{
		ProfileStartup: true,
		WorkingDir:     t.TempDir(),
		Version:        "test",
	}

	// Call twice -- second call should not error
	_, err := RunInitSequence(context.Background(), opts)
	if err != nil {
		t.Fatalf("first RunInitSequence error: %v", err)
	}

	_, err = RunInitSequence(context.Background(), opts)
	if err != nil {
		t.Fatalf("second RunInitSequence error: %v", err)
	}
}

func TestRunInitSequenceReturnsResults(t *testing.T) {
	result, err := RunInitSequence(context.Background(), InitOptions{
		ProfileStartup: true,
		WorkingDir:     t.TempDir(),
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("RunInitSequence error: %v", err)
	}

	// Config should not be nil
	if result.Config == nil {
		t.Error("result.Config is nil")
	}
	// Settings should not be nil
	if result.Settings == nil {
		t.Error("result.Settings is nil")
	}
	// MDMSettings should not be nil
	if result.MDMSettings == nil {
		t.Error("result.MDMSettings is nil")
	}
	// PlatformInfo should have OS set
	if result.PlatformInfo.OS == "" {
		t.Error("result.PlatformInfo.OS is empty")
	}
	// Profiler should not be nil
	if result.Profiler == nil {
		t.Error("result.Profiler is nil")
	}
}

func TestRunInitSequenceProfileEnvVar(t *testing.T) {
	t.Setenv("CLAUDE_CODE_PROFILE_STARTUP", "1")

	result, err := RunInitSequence(context.Background(), InitOptions{
		WorkingDir: t.TempDir(),
		Version:    "test",
	})
	if err != nil {
		t.Fatalf("RunInitSequence error: %v", err)
	}

	// Profiling should be enabled via env var even though opts.ProfileStartup=false
	report := result.Profiler.Report()
	if !strings.Contains(report, "config_loaded") {
		t.Errorf("expected profiling enabled via env var, got empty report: %s", report)
	}
}

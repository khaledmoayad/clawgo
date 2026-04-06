package startup

import (
	"context"
	"os"

	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/khaledmoayad/clawgo/internal/platform"
)

// InitResult holds the results of the init sequence.
type InitResult struct {
	Config       *config.Config
	Settings     *config.Settings
	MDMSettings  *config.Settings
	PlatformInfo platform.Info
	Profiler     *Profiler
}

// InitOptions controls the init sequence behavior.
type InitOptions struct {
	ProfileStartup bool   // Explicit flag; also checks CLAUDE_CODE_PROFILE_STARTUP env var.
	WorkingDir     string // Working directory for settings resolution.
	ProjectRoot    string // Project root for settings resolution (defaults to WorkingDir).
	Version        string // Application version string.
}

// RunInitSequence executes the startup initialization in dependency order:
//  1. Config files (global + project)
//  2. MDM settings (platform-specific enterprise config)
//  3. Environment variable application from config
//  4. Graceful shutdown setup (signal handlers)
//  5. Platform detection (OS, CI, terminal, git)
//  6. Background tasks (IDE detection, repo detection) -- non-blocking
//
// Profiling checkpoints are recorded at each step boundary.
// The function is idempotent -- calling it multiple times is safe.
func RunInitSequence(ctx context.Context, opts InitOptions) (*InitResult, error) {
	// Determine if profiling is enabled via option or env var
	profileEnabled := opts.ProfileStartup || os.Getenv("CLAUDE_CODE_PROFILE_STARTUP") == "1"
	profiler := NewProfiler(profileEnabled)

	result := &InitResult{
		Profiler: profiler,
	}

	// Resolve project root
	projectRoot := opts.ProjectRoot
	if projectRoot == "" {
		projectRoot = opts.WorkingDir
	}

	// Step 1: Load config files (global config)
	cfg, err := config.LoadConfig()
	if err != nil {
		// Config load failure is non-fatal; use empty config
		cfg = &config.Config{}
	}
	result.Config = cfg
	profiler.Checkpoint("config_loaded")

	// Step 2: Load MDM/enterprise settings
	mdmSettings := config.LoadMDMSettings()
	result.MDMSettings = mdmSettings
	profiler.Checkpoint("mdm_loaded")

	// Step 3: Load and apply merged settings (user + project + MDM)
	configDir := config.ConfigDir()
	settings, err := config.LoadSettings(configDir, projectRoot)
	if err != nil {
		// Settings load failure is non-fatal; use empty settings
		settings = &config.Settings{}
	}
	result.Settings = settings

	// Apply safe environment variables from config/settings
	applyConfigEnvVars(cfg, settings)
	profiler.Checkpoint("env_applied")

	// Step 4: Graceful shutdown is handled by app.SetupGracefulShutdown
	// at the call site since it requires the context cancel function.
	// We record the checkpoint to track when this step would execute.
	profiler.Checkpoint("shutdown_setup")

	// Step 5: Platform detection
	result.PlatformInfo = platform.GetInfo()
	profiler.Checkpoint("platform_detected")

	// Step 6: Start background tasks (non-blocking)
	// IDE detection, repository detection, remote settings, policy limits
	// are started as goroutines that complete asynchronously.
	startBackgroundTasks(ctx, settings)
	profiler.Checkpoint("background_started")

	return result, nil
}

// applyConfigEnvVars sets environment variables from the config/settings
// that are safe to apply at startup. This mirrors the TypeScript
// applySafeConfigEnvironmentVariables().
func applyConfigEnvVars(cfg *config.Config, settings *config.Settings) {
	// Apply environment variables from settings.Env map
	if settings != nil && len(settings.Env) > 0 {
		for k, v := range settings.Env {
			// Only set if not already set (env vars take precedence)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

// startBackgroundTasks launches non-blocking goroutines for deferred init work.
// These tasks are best-effort and do not block the startup sequence.
func startBackgroundTasks(ctx context.Context, settings *config.Settings) {
	// IDE detection: runs asynchronously if IDE extensions are configured.
	// Repository detection: discovers git repo info in background.
	// Remote managed settings: fetched if eligible.
	// Policy limits: fetched if eligible.
	//
	// These are currently no-ops that will be wired to their respective
	// subsystems as they are implemented (IDE detection in 16-05, etc.).
	_ = ctx
	_ = settings
}

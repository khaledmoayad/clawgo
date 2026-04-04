package bridge

import "time"

const (
	defaultMaxConcurrentSessions = 5
	defaultPollInterval          = 5 * time.Second
)

// Config holds configuration for the bridge mode.
type Config struct {
	// APIBaseURL is the base URL for the bridge API (e.g., "https://api.anthropic.com").
	APIBaseURL string
	// GetToken returns the current auth token for API requests.
	GetToken func() string
	// EnvironmentName is the display name for this bridge environment.
	EnvironmentName string
	// MaxConcurrentSessions limits the number of simultaneous child sessions.
	// Defaults to 5 if zero.
	MaxConcurrentSessions int
	// PollInterval is the duration between work polling requests.
	// Defaults to 5s if zero.
	PollInterval time.Duration
}

// withDefaults returns a copy of the config with zero values filled with defaults.
func (c Config) withDefaults() Config {
	if c.MaxConcurrentSessions <= 0 {
		c.MaxConcurrentSessions = defaultMaxConcurrentSessions
	}
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}
	return c
}

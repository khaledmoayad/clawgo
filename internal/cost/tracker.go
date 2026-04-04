package cost

import "sync"

// Usage tracks token counts from a single API response.
// This mirrors the api.Usage type but is defined here to avoid circular
// dependencies between the api and cost packages.
type Usage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// Tracker accumulates token usage and cost across an entire session.
// It is safe for concurrent use.
type Tracker struct {
	mu                       sync.Mutex
	Model                    string
	TotalInputTokens         int
	TotalOutputTokens        int
	TotalCacheCreationTokens int
	TotalCacheReadTokens     int
	TurnCount                int
}

// NewTracker creates a new cost tracker for the given model.
func NewTracker(model string) *Tracker {
	return &Tracker{Model: model}
}

// Add accumulates usage from a single API response.
func (t *Tracker) Add(u Usage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalInputTokens += u.InputTokens
	t.TotalOutputTokens += u.OutputTokens
	t.TotalCacheCreationTokens += u.CacheCreationInputTokens
	t.TotalCacheReadTokens += u.CacheReadInputTokens
	t.TurnCount++
}

// Cost returns the total session cost in USD.
func (t *Tracker) Cost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.computeCost(
		t.TotalInputTokens,
		t.TotalOutputTokens,
		t.TotalCacheCreationTokens,
		t.TotalCacheReadTokens,
	)
}

// TurnCost returns the cost for a specific usage (for per-turn display).
func (t *Tracker) TurnCost(u Usage) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.computeCost(
		u.InputTokens,
		u.OutputTokens,
		u.CacheCreationInputTokens,
		u.CacheReadInputTokens,
	)
}

// Reset clears all accumulated usage.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalInputTokens = 0
	t.TotalOutputTokens = 0
	t.TotalCacheCreationTokens = 0
	t.TotalCacheReadTokens = 0
	t.TurnCount = 0
}

// computeCost calculates USD cost from token counts using the model's pricing.
// Must be called with t.mu held.
func (t *Tracker) computeCost(input, output, cacheCreation, cacheRead int) float64 {
	pricing := GetPricing(t.Model)
	cost := float64(input)*pricing.InputPerToken +
		float64(output)*pricing.OutputPerToken +
		float64(cacheCreation)*pricing.CacheCreationInputPerToken +
		float64(cacheRead)*pricing.CacheReadInputPerToken
	return cost
}

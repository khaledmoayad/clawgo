package cost

import (
	"math"
	"sync"
	"testing"
)

func TestTracker_Add(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{InputTokens: 100, OutputTokens: 50})

	if tracker.TotalInputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", tracker.TotalInputTokens)
	}
	if tracker.TotalOutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", tracker.TotalOutputTokens)
	}
	if tracker.TurnCount != 1 {
		t.Errorf("expected 1 turn, got %d", tracker.TurnCount)
	}
}

func TestTracker_AddMultiple(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{InputTokens: 100, OutputTokens: 50})
	tracker.Add(Usage{InputTokens: 200, OutputTokens: 100})
	tracker.Add(Usage{InputTokens: 300, OutputTokens: 150})

	if tracker.TotalInputTokens != 600 {
		t.Errorf("expected 600 input tokens, got %d", tracker.TotalInputTokens)
	}
	if tracker.TotalOutputTokens != 300 {
		t.Errorf("expected 300 output tokens, got %d", tracker.TotalOutputTokens)
	}
	if tracker.TurnCount != 3 {
		t.Errorf("expected 3 turns, got %d", tracker.TurnCount)
	}
}

func TestTracker_AddWithCache(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 500,
		CacheReadInputTokens:     200,
	})

	if tracker.TotalCacheCreationTokens != 500 {
		t.Errorf("expected 500 cache creation tokens, got %d", tracker.TotalCacheCreationTokens)
	}
	if tracker.TotalCacheReadTokens != 200 {
		t.Errorf("expected 200 cache read tokens, got %d", tracker.TotalCacheReadTokens)
	}
}

func TestTracker_Cost_Sonnet(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{InputTokens: 1000, OutputTokens: 500})

	// Expected: (1000 * 3/1e6) + (500 * 15/1e6) = 0.003 + 0.0075 = 0.0105
	expected := 0.0105
	cost := tracker.Cost()
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("expected cost $%.6f, got $%.6f", expected, cost)
	}
}

func TestTracker_Cost_Opus(t *testing.T) {
	tracker := NewTracker("claude-opus-4-20250514")
	tracker.Add(Usage{InputTokens: 1000, OutputTokens: 500})

	// Expected: (1000 * 15/1e6) + (500 * 75/1e6) = 0.015 + 0.0375 = 0.0525
	expected := 0.0525
	cost := tracker.Cost()
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("expected cost $%.6f, got $%.6f", expected, cost)
	}
}

func TestTracker_Cost_Haiku(t *testing.T) {
	tracker := NewTracker("claude-haiku-3-5-20241022")
	tracker.Add(Usage{InputTokens: 1000, OutputTokens: 500})

	// Expected: (1000 * 0.80/1e6) + (500 * 4/1e6) = 0.0008 + 0.002 = 0.0028
	expected := 0.0028
	cost := tracker.Cost()
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("expected cost $%.6f, got $%.6f", expected, cost)
	}
}

func TestTracker_Cost_WithCache(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{
		InputTokens:              1000,
		OutputTokens:             500,
		CacheCreationInputTokens: 2000,
		CacheReadInputTokens:     3000,
	})

	// Expected: (1000 * 3/1e6) + (500 * 15/1e6) + (2000 * 3.75/1e6) + (3000 * 0.30/1e6)
	// = 0.003 + 0.0075 + 0.0075 + 0.0009 = 0.0189
	expected := 0.0189
	cost := tracker.Cost()
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("expected cost $%.6f, got $%.6f", expected, cost)
	}
}

func TestTracker_TurnCost(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	u := Usage{InputTokens: 500, OutputTokens: 100}

	// Expected: (500 * 3/1e6) + (100 * 15/1e6) = 0.0015 + 0.0015 = 0.003
	expected := 0.003
	cost := tracker.TurnCost(u)
	if math.Abs(cost-expected) > 1e-9 {
		t.Errorf("expected turn cost $%.6f, got $%.6f", expected, cost)
	}
}

func TestTracker_Reset(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{InputTokens: 1000, OutputTokens: 500})
	tracker.Reset()

	if tracker.TotalInputTokens != 0 {
		t.Errorf("expected 0 input tokens after reset, got %d", tracker.TotalInputTokens)
	}
	if tracker.TotalOutputTokens != 0 {
		t.Errorf("expected 0 output tokens after reset, got %d", tracker.TotalOutputTokens)
	}
	if tracker.TurnCount != 0 {
		t.Errorf("expected 0 turns after reset, got %d", tracker.TurnCount)
	}
	if tracker.Cost() != 0 {
		t.Errorf("expected $0 cost after reset, got %f", tracker.Cost())
	}
}

func TestTracker_Concurrent(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	goroutines := 10
	addsPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < addsPerGoroutine; j++ {
				tracker.Add(Usage{InputTokens: 10, OutputTokens: 5})
			}
		}()
	}
	wg.Wait()

	expectedInput := goroutines * addsPerGoroutine * 10  // 10,000
	expectedOutput := goroutines * addsPerGoroutine * 5   // 5,000
	expectedTurns := goroutines * addsPerGoroutine         // 1,000

	if tracker.TotalInputTokens != expectedInput {
		t.Errorf("expected %d input tokens, got %d", expectedInput, tracker.TotalInputTokens)
	}
	if tracker.TotalOutputTokens != expectedOutput {
		t.Errorf("expected %d output tokens, got %d", expectedOutput, tracker.TotalOutputTokens)
	}
	if tracker.TurnCount != expectedTurns {
		t.Errorf("expected %d turns, got %d", expectedTurns, tracker.TurnCount)
	}
}

func TestGetPricing_Known(t *testing.T) {
	p := GetPricing("claude-sonnet-4-20250514")
	expectedInputPerToken := 3.0 / 1_000_000
	if math.Abs(p.InputPerToken-expectedInputPerToken) > 1e-15 {
		t.Errorf("expected input price %.10f, got %.10f", expectedInputPerToken, p.InputPerToken)
	}
	expectedOutputPerToken := 15.0 / 1_000_000
	if math.Abs(p.OutputPerToken-expectedOutputPerToken) > 1e-15 {
		t.Errorf("expected output price %.10f, got %.10f", expectedOutputPerToken, p.OutputPerToken)
	}
}

func TestGetPricing_Unknown(t *testing.T) {
	p := GetPricing("unknown-model-xyz")
	// Should return default pricing (same as Sonnet 4)
	expectedInputPerToken := 3.0 / 1_000_000
	if math.Abs(p.InputPerToken-expectedInputPerToken) > 1e-15 {
		t.Errorf("expected default input price %.10f, got %.10f", expectedInputPerToken, p.InputPerToken)
	}
}

func TestGetPricing_Opus(t *testing.T) {
	p := GetPricing("claude-opus-4-20250514")
	expectedInputPerToken := 15.0 / 1_000_000
	if math.Abs(p.InputPerToken-expectedInputPerToken) > 1e-15 {
		t.Errorf("expected input price %.10f, got %.10f", expectedInputPerToken, p.InputPerToken)
	}
	expectedOutputPerToken := 75.0 / 1_000_000
	if math.Abs(p.OutputPerToken-expectedOutputPerToken) > 1e-15 {
		t.Errorf("expected output price %.10f, got %.10f", expectedOutputPerToken, p.OutputPerToken)
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		usd      float64
		expected string
	}{
		{0.0, "$0.00"},
		{0.0042, "$0.0042"},
		{0.0105, "$0.0105"},
		{1.23, "$1.23"},
		{10.5, "$10.50"},
		{0.001, "$0.0010"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCost(tt.usd)
			if result != tt.expected {
				t.Errorf("FormatCost(%f) = %q, expected %q", tt.usd, result, tt.expected)
			}
		})
	}
}

func TestFormatUsage(t *testing.T) {
	tracker := NewTracker("claude-sonnet-4-20250514")
	tracker.Add(Usage{InputTokens: 1234, OutputTokens: 567})

	result := FormatUsage(tracker)
	// Expected cost: (1234 * 3/1e6) + (567 * 15/1e6) = 0.003702 + 0.008505 = 0.012207
	expected := "Tokens: 1234 in / 567 out | Cost: $0.0122"
	if result != expected {
		t.Errorf("FormatUsage = %q, expected %q", result, expected)
	}
}

func TestFormatTurnUsage(t *testing.T) {
	u := Usage{InputTokens: 456, OutputTokens: 123}
	result := FormatTurnUsage(3, u, "claude-sonnet-4-20250514")
	// Expected cost: (456 * 3/1e6) + (123 * 15/1e6) = 0.001368 + 0.001845 = 0.003213
	expected := "Turn 3: 456 in / 123 out ($0.0032)"
	if result != expected {
		t.Errorf("FormatTurnUsage = %q, expected %q", result, expected)
	}
}

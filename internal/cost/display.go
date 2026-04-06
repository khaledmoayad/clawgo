package cost

import "fmt"

// FormatCost formats a USD amount for display.
// Uses variable precision: "$0.0042", "$1.23", "$0.00" for zero.
func FormatCost(usd float64) string {
	if usd == 0 {
		return "$0.00"
	}
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	if usd < 1.0 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

// FormatCostUSD formats a USD cost with "$" prefix and appropriate precision.
// Uses 4 decimal places for sub-dollar amounts, 2 for larger amounts.
// This is an alias for FormatCost, named to match the TypeScript convention.
func FormatCostUSD(usd float64) string {
	return FormatCost(usd)
}

// FormatUsage returns a one-line session usage summary.
// Example: "Tokens: 1234 in / 567 out | Cost: $0.0185"
func FormatUsage(t *Tracker) string {
	t.mu.Lock()
	totalIn := t.TotalInputTokens
	totalOut := t.TotalOutputTokens
	t.mu.Unlock()

	cost := t.Cost()
	return fmt.Sprintf("Tokens: %d in / %d out | Cost: %s", totalIn, totalOut, FormatCost(cost))
}

// FormatTurnUsage returns per-turn usage summary.
// Example: "Turn 3: 456 in / 123 out ($0.0023)"
func FormatTurnUsage(turn int, u Usage, model string) string {
	pricing := GetPricing(model)
	cost := float64(u.InputTokens)*pricing.InputPerToken +
		float64(u.OutputTokens)*pricing.OutputPerToken +
		float64(u.CacheCreationInputTokens)*pricing.CacheCreationInputPerToken +
		float64(u.CacheReadInputTokens)*pricing.CacheReadInputPerToken
	return fmt.Sprintf("Turn %d: %d in / %d out (%s)", turn, u.InputTokens, u.OutputTokens, FormatCost(cost))
}

// Package cost provides token usage tracking and cost computation for
// Anthropic API usage. It maintains a pricing table for known models
// and accumulates usage across an entire session.
package cost

// ModelPricing contains per-token prices in USD.
type ModelPricing struct {
	InputPerToken              float64
	OutputPerToken             float64
	CacheCreationInputPerToken float64
	CacheReadInputPerToken     float64
}

// modelAliases maps short/latest model names to their dated versions.
// This mirrors the TypeScript behavior where aliases like "claude-sonnet-4"
// resolve to the specific dated version for pricing lookup.
var modelAliases = map[string]string{
	"claude-sonnet-4":          "claude-sonnet-4-20250514",
	"claude-opus-4":            "claude-opus-4-20250514",
	"claude-3-5-sonnet-latest": "claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-latest":  "claude-3-5-haiku-20241022",
	"claude-haiku-3-5":         "claude-haiku-3-5-20241022",
}

// Pricing table: values are in USD per million tokens.
// These are divided by 1,000,000 in GetPricing to return per-token prices.
var pricingTable = map[string]pricingEntry{
	// Claude Sonnet 4
	"claude-sonnet-4-20250514": {
		inputPerMTok:              3.0,
		outputPerMTok:             15.0,
		cacheCreationInputPerMTok: 3.75,
		cacheReadInputPerMTok:     0.30,
	},
	// Claude Opus 4
	"claude-opus-4-20250514": {
		inputPerMTok:              15.0,
		outputPerMTok:             75.0,
		cacheCreationInputPerMTok: 18.75,
		cacheReadInputPerMTok:     1.50,
	},
	// Claude Haiku 3.5 (also known as claude-3-5-haiku)
	"claude-haiku-3-5-20241022": {
		inputPerMTok:              0.80,
		outputPerMTok:             4.0,
		cacheCreationInputPerMTok: 1.0,
		cacheReadInputPerMTok:     0.08,
	},
	// Claude 3.5 Haiku (alternate naming convention, same model)
	"claude-3-5-haiku-20241022": {
		inputPerMTok:              0.80,
		outputPerMTok:             4.0,
		cacheCreationInputPerMTok: 1.0,
		cacheReadInputPerMTok:     0.08,
	},
	// Claude Sonnet 3.5 v2
	"claude-3-5-sonnet-20241022": {
		inputPerMTok:              3.0,
		outputPerMTok:             15.0,
		cacheCreationInputPerMTok: 3.75,
		cacheReadInputPerMTok:     0.30,
	},
	// Claude 3 Haiku (older, cheaper model)
	"claude-3-haiku-20240307": {
		inputPerMTok:              0.25,
		outputPerMTok:             1.25,
		cacheCreationInputPerMTok: 0.30,
		cacheReadInputPerMTok:     0.03,
	},
}

// Default pricing (same as Claude Sonnet 4) for unknown models.
var defaultPricing = pricingEntry{
	inputPerMTok:              3.0,
	outputPerMTok:             15.0,
	cacheCreationInputPerMTok: 3.75,
	cacheReadInputPerMTok:     0.30,
}

type pricingEntry struct {
	inputPerMTok              float64
	outputPerMTok             float64
	cacheCreationInputPerMTok float64
	cacheReadInputPerMTok     float64
}

const perMillion = 1_000_000.0

// GetPricing returns the per-token pricing for a model.
// Resolves aliases (e.g. "claude-sonnet-4" -> "claude-sonnet-4-20250514")
// before lookup. Falls back to default pricing (Sonnet 4 rates) for unknown models.
func GetPricing(model string) ModelPricing {
	// Resolve alias to dated version
	if resolved, ok := modelAliases[model]; ok {
		model = resolved
	}
	entry, ok := pricingTable[model]
	if !ok {
		entry = defaultPricing
	}
	return ModelPricing{
		InputPerToken:              entry.inputPerMTok / perMillion,
		OutputPerToken:             entry.outputPerMTok / perMillion,
		CacheCreationInputPerToken: entry.cacheCreationInputPerMTok / perMillion,
		CacheReadInputPerToken:     entry.cacheReadInputPerMTok / perMillion,
	}
}

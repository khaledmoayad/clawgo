package cost

import (
	"math"
	"testing"
)

func TestGetPricing_Sonnet4Dated(t *testing.T) {
	p := GetPricing("claude-sonnet-4-20250514")
	expectedInput := 3.0 / 1_000_000
	expectedOutput := 15.0 / 1_000_000

	if math.Abs(p.InputPerToken-expectedInput) > 1e-15 {
		t.Errorf("InputPerToken = %.10f, want %.10f", p.InputPerToken, expectedInput)
	}
	if math.Abs(p.OutputPerToken-expectedOutput) > 1e-15 {
		t.Errorf("OutputPerToken = %.10f, want %.10f", p.OutputPerToken, expectedOutput)
	}
}

func TestGetPricingAlias_Sonnet4(t *testing.T) {
	// "claude-sonnet-4" should resolve to same pricing as "claude-sonnet-4-20250514"
	aliased := GetPricing("claude-sonnet-4")
	dated := GetPricing("claude-sonnet-4-20250514")

	if aliased != dated {
		t.Errorf("claude-sonnet-4 pricing %+v != claude-sonnet-4-20250514 pricing %+v", aliased, dated)
	}
}

func TestGetPricingAlias_Opus4(t *testing.T) {
	// "claude-opus-4" should resolve to same pricing as "claude-opus-4-20250514"
	aliased := GetPricing("claude-opus-4")
	dated := GetPricing("claude-opus-4-20250514")

	if aliased != dated {
		t.Errorf("claude-opus-4 pricing %+v != claude-opus-4-20250514 pricing %+v", aliased, dated)
	}
}

func TestGetPricingAlias_Sonnet35Latest(t *testing.T) {
	// "claude-3-5-sonnet-latest" should resolve to Sonnet 3.5 v2 pricing
	aliased := GetPricing("claude-3-5-sonnet-latest")
	dated := GetPricing("claude-3-5-sonnet-20241022")

	if aliased != dated {
		t.Errorf("claude-3-5-sonnet-latest pricing %+v != claude-3-5-sonnet-20241022 pricing %+v", aliased, dated)
	}
}

func TestGetPricing_Haiku35(t *testing.T) {
	p := GetPricing("claude-3-5-haiku-20241022")
	expectedInput := 0.80 / 1_000_000
	expectedOutput := 4.0 / 1_000_000

	if math.Abs(p.InputPerToken-expectedInput) > 1e-15 {
		t.Errorf("InputPerToken = %.10f, want %.10f", p.InputPerToken, expectedInput)
	}
	if math.Abs(p.OutputPerToken-expectedOutput) > 1e-15 {
		t.Errorf("OutputPerToken = %.10f, want %.10f", p.OutputPerToken, expectedOutput)
	}
}

func TestGetPricing_Haiku3(t *testing.T) {
	p := GetPricing("claude-3-haiku-20240307")
	expectedInput := 0.25 / 1_000_000
	expectedOutput := 1.25 / 1_000_000

	if math.Abs(p.InputPerToken-expectedInput) > 1e-15 {
		t.Errorf("InputPerToken = %.10f, want %.10f", p.InputPerToken, expectedInput)
	}
	if math.Abs(p.OutputPerToken-expectedOutput) > 1e-15 {
		t.Errorf("OutputPerToken = %.10f, want %.10f", p.OutputPerToken, expectedOutput)
	}
}

func TestGetPricing_UnknownFallsBackToDefault(t *testing.T) {
	p := GetPricing("unknown-model")
	// Default pricing is Sonnet 4 rates
	expectedInput := 3.0 / 1_000_000
	if math.Abs(p.InputPerToken-expectedInput) > 1e-15 {
		t.Errorf("unknown model InputPerToken = %.10f, want %.10f (Sonnet 4 default)", p.InputPerToken, expectedInput)
	}
}

func TestGetPricingAlias_Haiku35Latest(t *testing.T) {
	// "claude-3-5-haiku-latest" should resolve to haiku 3.5 pricing
	aliased := GetPricing("claude-3-5-haiku-latest")
	dated := GetPricing("claude-3-5-haiku-20241022")

	if aliased != dated {
		t.Errorf("claude-3-5-haiku-latest pricing %+v != claude-3-5-haiku-20241022 pricing %+v", aliased, dated)
	}
}

func TestFormatCostUSD(t *testing.T) {
	tests := []struct {
		usd      float64
		expected string
	}{
		{0.0, "$0.00"},
		{0.0012345, "$0.0012"},
		{0.0042, "$0.0042"},
		{0.0105, "$0.0105"},
		{1.23, "$1.23"},
		{10.5, "$10.50"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatCostUSD(tt.usd)
			if result != tt.expected {
				t.Errorf("FormatCostUSD(%f) = %q, expected %q", tt.usd, result, tt.expected)
			}
		})
	}
}

func TestGetPricing_AllModelsHaveCachePricing(t *testing.T) {
	// Verify all models in the pricing table have cache pricing set
	models := []string{
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"claude-haiku-3-5-20241022",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-haiku-20240307",
	}

	for _, model := range models {
		p := GetPricing(model)
		if p.CacheCreationInputPerToken == 0 {
			t.Errorf("model %s: CacheCreationInputPerToken should not be zero", model)
		}
		if p.CacheReadInputPerToken == 0 {
			t.Errorf("model %s: CacheReadInputPerToken should not be zero", model)
		}
	}
}

package api

import (
	"testing"
)

func TestIsEnvTruthy(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		expected bool
	}{
		{"1 is truthy", "TEST_TRUTHY", "1", true},
		{"true is truthy", "TEST_TRUTHY", "true", true},
		{"TRUE is truthy", "TEST_TRUTHY", "TRUE", true},
		{"True is truthy", "TEST_TRUTHY", "True", true},
		{"yes is truthy", "TEST_TRUTHY", "yes", true},
		{"YES is truthy", "TEST_TRUTHY", "YES", true},
		{"Yes is truthy", "TEST_TRUTHY", "Yes", true},
		{"0 is falsy", "TEST_TRUTHY", "0", false},
		{"false is falsy", "TEST_TRUTHY", "false", false},
		{"empty is falsy", "TEST_TRUTHY", "", false},
		{"random string is falsy", "TEST_TRUTHY", "random", false},
		{"unset is falsy", "TEST_TRUTHY_UNSET", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv(tt.envKey, tt.envVal)
			}
			got := isEnvTruthy(tt.envKey)
			if got != tt.expected {
				t.Errorf("isEnvTruthy(%q) with value %q = %v, want %v", tt.envKey, tt.envVal, got, tt.expected)
			}
		})
	}
}

func TestGetProvider(t *testing.T) {
	t.Run("returns FirstParty when no env vars set", func(t *testing.T) {
		// Ensure all provider env vars are cleared
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")

		got := GetProvider()
		if got != ProviderFirstParty {
			t.Errorf("GetProvider() = %q, want %q", got, ProviderFirstParty)
		}
	})

	t.Run("returns Bedrock when CLAUDE_CODE_USE_BEDROCK=1", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")

		got := GetProvider()
		if got != ProviderBedrock {
			t.Errorf("GetProvider() = %q, want %q", got, ProviderBedrock)
		}
	})

	t.Run("returns Vertex when CLAUDE_CODE_USE_VERTEX=1", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")

		got := GetProvider()
		if got != ProviderVertex {
			t.Errorf("GetProvider() = %q, want %q", got, ProviderVertex)
		}
	})

	t.Run("returns Foundry when CLAUDE_CODE_USE_FOUNDRY=1", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")

		got := GetProvider()
		if got != ProviderFoundry {
			t.Errorf("GetProvider() = %q, want %q", got, ProviderFoundry)
		}
	})

	t.Run("Bedrock takes priority over Vertex when both set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")

		got := GetProvider()
		if got != ProviderBedrock {
			t.Errorf("GetProvider() = %q, want %q (Bedrock should have priority)", got, ProviderBedrock)
		}
	})

	t.Run("Bedrock takes priority over Foundry when both set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")

		got := GetProvider()
		if got != ProviderBedrock {
			t.Errorf("GetProvider() = %q, want %q (Bedrock should have priority)", got, ProviderBedrock)
		}
	})

	t.Run("Vertex takes priority over Foundry when both set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
		t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
		t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")

		got := GetProvider()
		if got != ProviderVertex {
			t.Errorf("GetProvider() = %q, want %q (Vertex should have priority over Foundry)", got, ProviderVertex)
		}
	})
}

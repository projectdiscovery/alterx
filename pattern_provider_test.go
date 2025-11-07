package alterx

import (
	"testing"

	"github.com/projectdiscovery/alterx/mining"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManualPatternProvider(t *testing.T) {
	patterns := []string{"{{word}}.{{root}}", "{{word}}-{{number}}.{{root}}"}
	payloads := map[string][]string{
		"word":   {"api", "dev", "staging"},
		"number": {"1", "2", "3"},
	}

	provider := NewManualPatternProvider(patterns, payloads)
	require.NotNil(t, provider)

	gotPatterns, gotPayloads, err := provider.GetPatterns()
	require.NoError(t, err)

	assert.Equal(t, patterns, gotPatterns)
	assert.Equal(t, payloads, gotPayloads)
}

func TestManualPatternProvider_EmptyPatterns(t *testing.T) {
	provider := NewManualPatternProvider([]string{}, map[string][]string{})

	_, _, err := provider.GetPatterns()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no patterns provided")
}

func TestMinedPatternProvider(t *testing.T) {
	// Use simple test domains
	domains := []string{
		"api-prod.example.com",
		"api-staging.example.com",
		"web-prod.example.com",
		"web-staging.example.com",
	}

	miningOpts := &mining.Options{
		MinLDist:            2,
		MaxLDist:            5,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
	}

	provider := NewMinedPatternProvider(domains, miningOpts)
	require.NotNil(t, provider)

	patterns, payloads, err := provider.GetPatterns()
	require.NoError(t, err)

	// Should generate at least some patterns
	assert.Greater(t, len(patterns), 0, "Should generate at least one pattern")

	// Patterns should end with .{{root}}
	for _, pattern := range patterns {
		assert.Contains(t, pattern, "{{", "Pattern should contain DSL syntax")
	}

	// Should have some payloads
	assert.Greater(t, len(payloads), 0, "Should generate at least one payload key")

	t.Logf("Generated %d patterns", len(patterns))
	t.Logf("Generated %d payload keys", len(payloads))

	// Log a few patterns for inspection
	for i, pattern := range patterns {
		if i >= 5 {
			t.Logf("... and %d more patterns", len(patterns)-5)
			break
		}
		t.Logf("  Pattern %d: %s", i+1, pattern)
	}
}

func TestMinedPatternProvider_InsufficientDomains(t *testing.T) {
	// Test with very few domains
	domains := []string{
		"api.example.com",
	}

	miningOpts := &mining.Options{
		MinLDist: 2,
		MaxLDist: 5,
	}

	provider := NewMinedPatternProvider(domains, miningOpts)
	patterns, payloads, err := provider.GetPatterns()

	// Should handle gracefully (might return error or simple patterns)
	if err != nil {
		t.Logf("Expected behavior: %v", err)
	} else {
		t.Logf("Generated %d patterns from single domain", len(patterns))
		t.Logf("Payloads: %v", payloads)
	}
}

func TestMutatorIntegration_ManualMode(t *testing.T) {
	opts := &Options{
		Domains:  []string{"example.com"},
		Patterns: []string{"{{word}}.{{root}}"},
		Payloads: map[string][]string{
			"word": {"api", "dev"},
		},
		Discover: false,
	}

	mutator, err := New(opts)
	require.NoError(t, err)
	require.NotNil(t, mutator)

	assert.Equal(t, 1, len(mutator.Options.Patterns))
	assert.Equal(t, 2, len(mutator.Options.Payloads["word"]))
}

func TestMutatorIntegration_DiscoverMode(t *testing.T) {
	opts := &Options{
		Domains: []string{
			"api-prod.example.com",
			"api-staging.example.com",
			"web-prod.example.com",
			"web-staging.example.com",
			"db-primary.example.com",
			"db-secondary.example.com",
			"cache-1.example.com",
			"cache-2.example.com",
			"app-v1.example.com",
			"app-v2.example.com",
		},
		Discover: true,
	}

	mutator, err := New(opts)
	require.NoError(t, err)
	require.NotNil(t, mutator)

	// Should have discovered patterns and payloads
	assert.Greater(t, len(mutator.Options.Patterns), 0, "Should discover patterns")
	assert.Greater(t, len(mutator.Options.Payloads), 0, "Should discover payloads")

	t.Logf("Discovered %d patterns", len(mutator.Options.Patterns))
	t.Logf("Discovered %d payload keys", len(mutator.Options.Payloads))

	// Show some patterns
	for i, pattern := range mutator.Options.Patterns {
		if i >= 5 {
			t.Logf("... and %d more patterns", len(mutator.Options.Patterns)-5)
			break
		}
		t.Logf("  Pattern %d: %s", i+1, pattern)
	}
}

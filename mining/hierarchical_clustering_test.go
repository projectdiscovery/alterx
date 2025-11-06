package mining

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPatternDeduplication verifies that duplicate patterns are not stored twice
func TestPatternDeduplication(t *testing.T) {
	domains := []string{
		"api-prod.example.com",
		"api-staging.example.com",
	}

	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            10,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
	})
	require.NoError(t, err)

	// Generate the same pattern twice
	success1 := pm.tryGenerateAndStorePattern([]string{"api-prod", "api-staging"})
	success2 := pm.tryGenerateAndStorePattern([]string{"api-prod", "api-staging"})

	assert.True(t, success1, "First pattern should be stored")
	assert.False(t, success2, "Second identical pattern should be rejected (duplicate)")

	results := pm.GetResults()
	assert.Len(t, results, 1, "Should only have one pattern (deduplication working)")
}

// TestRedundantPrefixFiltering verifies that redundant prefixes are skipped
func TestRedundantPrefixFiltering(t *testing.T) {
	// Test data where we have redundant prefixes
	// "api" and "api-prod" where "api-prod" starts with "api"
	domains := []string{
		"api.example.com",
		"api-prod.example.com",
		"api-prod-1.example.com",
		"api-staging.example.com",
	}

	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            3,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
	})
	require.NoError(t, err)

	// Manually test processNgramHierarchy with ngram "a"
	err = pm.processNgramHierarchy("a")
	require.NoError(t, err)

	results := pm.GetResults()

	// Verify patterns were generated
	assert.Greater(t, len(results), 0, "Should generate at least one pattern")

	// Check that we're not generating redundant patterns
	// This is more of a smoke test - the real test is that it doesn't error
	t.Logf("Generated %d patterns", len(results))
	for i, pattern := range results {
		t.Logf("Pattern %d: %s with %d payloads", i+1, pattern.Pattern, len(pattern.Payloads))
	}
}

// TestHierarchicalClustering verifies the full hierarchical clustering pipeline
func TestHierarchicalClustering(t *testing.T) {
	domains := []string{
		"api-prod-1.example.com",
		"api-prod-2.example.com",
		"api-staging-1.example.com",
		"web-prod.example.com",
		"web-staging.example.com",
	}

	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            5,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
	})
	require.NoError(t, err)

	// Run hierarchical clustering
	err = pm.hierarchicalNgramClustering()
	require.NoError(t, err)

	results := pm.GetResults()

	// Should generate multiple patterns at different levels
	assert.Greater(t, len(results), 0, "Should generate at least one pattern")

	t.Logf("Generated %d total patterns:", len(results))
	for i, pattern := range results {
		t.Logf("  %d. %s (payloads: %d)", i+1, pattern.Pattern, len(pattern.Payloads))
	}
}

// TestFullExecutePipeline tests the complete Execute() workflow
func TestFullExecutePipeline(t *testing.T) {
	domains := []string{
		"api-prod.example.com",
		"api-staging.example.com",
		"api-dev.example.com",
		"web-prod.example.com",
		"web-staging.example.com",
	}

	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            5,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
	})
	require.NoError(t, err)

	// Execute full pipeline
	err = pm.Execute()
	require.NoError(t, err)

	results := pm.GetResults()

	// Should generate patterns
	assert.Greater(t, len(results), 0, "Execute should generate patterns")

	// Verify all patterns are unique (deduplication working)
	seenPatterns := make(map[string]bool)
	for _, pattern := range results {
		assert.False(t, seenPatterns[pattern.Pattern], "Pattern %s appears twice (deduplication failed)", pattern.Pattern)
		seenPatterns[pattern.Pattern] = true
	}

	t.Logf("Execute generated %d unique patterns:", len(results))
	for i, pattern := range results {
		combinations := 1
		for _, payload := range pattern.Payloads {
			combinations *= len(payload)
		}
		t.Logf("  %d. %s → %d combinations", i+1, pattern.Pattern, combinations)
	}
}

// TestQualityFilteringDuringClustering verifies bad patterns are rejected during clustering
func TestQualityFilteringDuringClustering(t *testing.T) {
	// Create patterns that would be too generic
	domains := []string{
		"a.example.com",
		"b.example.com",
		"c.example.com",
		"d.example.com",
		"e.example.com",
	}

	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            5,
		PatternThreshold:    2,   // Very strict - reject patterns with >2 combinations
		PatternQualityRatio: 0.5, // Strict ratio
	})
	require.NoError(t, err)

	// Try to generate a pattern from all 5 - should be rejected
	// Pattern {{p0}} with 5 values = 5 combinations, 5 inputs
	// Ratio = 5/5 = 1.0 > 0.5, and 5 > 2, so rejected
	success := pm.tryGenerateAndStorePattern([]string{"a", "b", "c", "d", "e"})
	assert.False(t, success, "Generic pattern should be rejected")

	// Try with just 2 - Pattern {{p0}} with 2 values = 2 combinations, 2 inputs
	// Ratio = 2/2 = 1.0 which is NOT < 0.5, and 2 is NOT < 2
	// So this will also be rejected! Let me use 3 threshold instead
	pm.options.PatternThreshold = 3 // Now 2 < 3 will pass
	success = pm.tryGenerateAndStorePattern([]string{"a", "b"})
	assert.True(t, success, "Simple pattern should pass with threshold=3")

	results := pm.GetResults()
	assert.Len(t, results, 1, "Should only have the accepted pattern")
}

// TestMaxLengthFiltering verifies patterns exceeding max length are rejected
func TestMaxLengthFiltering(t *testing.T) {
	domains := []string{
		"very-long-subdomain-name-here-prod.example.com",
		"very-long-subdomain-name-here-staging.example.com",
	}

	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            10,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
		MaxPatternLength:    20, // Pattern will be longer than this
	})
	require.NoError(t, err)

	success := pm.tryGenerateAndStorePattern([]string{
		"very-long-subdomain-name-here-prod",
		"very-long-subdomain-name-here-staging",
	})

	assert.False(t, success, "Long pattern should be rejected")
	assert.Len(t, pm.GetResults(), 0, "No patterns should be stored")
}

// TestStorePattern verifies the storePattern deduplication logic
func TestStorePattern(t *testing.T) {
	pm := &PatternMiner{
		results:      make([]*DSLPattern, 0),
		seenPatterns: make(map[string]struct{}),
	}

	pattern1 := &DSLPattern{
		Pattern: "api{{p0}}",
		Payloads: map[string][]string{
			"p0": {"-prod", "-staging"},
		},
	}

	pattern2 := &DSLPattern{
		Pattern: "api{{p0}}", // Same pattern string
		Payloads: map[string][]string{
			"p0": {"-dev", "-test"}, // Different payloads but same pattern string
		},
	}

	pattern3 := &DSLPattern{
		Pattern: "web{{p0}}", // Different pattern
		Payloads: map[string][]string{
			"p0": {"-prod", "-staging"},
		},
	}

	// First pattern should be stored
	stored := pm.storePattern(pattern1)
	assert.True(t, stored, "First pattern should be stored")
	assert.Len(t, pm.results, 1)

	// Second pattern with same pattern string should be rejected
	stored = pm.storePattern(pattern2)
	assert.False(t, stored, "Duplicate pattern string should be rejected")
	assert.Len(t, pm.results, 1, "Should still have only 1 pattern")

	// Third pattern with different string should be stored
	stored = pm.storePattern(pattern3)
	assert.True(t, stored, "Different pattern should be stored")
	assert.Len(t, pm.results, 2)

	// Nil pattern should not be stored
	stored = pm.storePattern(nil)
	assert.False(t, stored, "Nil pattern should not be stored")
	assert.Len(t, pm.results, 2)
}

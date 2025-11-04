package inducer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrchestratorBasic tests the orchestrator with a simple example
func TestOrchestratorBasic(t *testing.T) {
	// Simple test case: api with different environments and numbers
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"api-staging-01.example.com",
	}

	orchestrator := NewOrchestrator(len(domains))

	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)
	assert.NotEmpty(t, patterns, "Expected at least one pattern")

	// Verify statistics
	stats := orchestrator.GetStats()
	assert.Equal(t, len(domains), stats.InputDomains)
	assert.Equal(t, "THOROUGH", stats.Mode) // <100 domains = THOROUGH
	assert.Greater(t, stats.FinalPatterns, 0)

	// Log patterns for inspection
	t.Logf("Found %d patterns:", len(patterns))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d, confidence: %.2f)",
			i+1, pattern.Template, pattern.Coverage, pattern.Confidence)
	}

	// Basic sanity check: should find at least one pattern covering multiple domains
	foundMultiDomainPattern := false
	for _, pattern := range patterns {
		if pattern.Coverage >= 2 {
			foundMultiDomainPattern = true
			break
		}
	}
	assert.True(t, foundMultiDomainPattern, "Expected at least one pattern covering multiple domains")
}

// TestOrchestratorEmpty tests empty input
func TestOrchestratorEmpty(t *testing.T) {
	orchestrator := NewOrchestrator(0)

	patterns, err := orchestrator.LearnPatterns([]string{})
	require.NoError(t, err, "Empty input should not error")
	assert.Empty(t, patterns, "Expected no patterns for empty input")
}

// TestOrchestratorSingleDomain tests single domain input
func TestOrchestratorSingleDomain(t *testing.T) {
	domains := []string{"api.example.com"}

	orchestrator := NewOrchestrator(len(domains))

	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)

	// Single domain should produce no patterns (no variations to learn)
	assert.Empty(t, patterns, "Single domain should produce no patterns (no variations)")

	if len(patterns) > 0 {
		t.Logf("Got %d patterns for single domain:", len(patterns))
		for i, pattern := range patterns {
			t.Logf("  Pattern %d: %s", i+1, pattern.Template)
		}
	}
}

// TestOrchestratorWebServices tests a more realistic scenario
func TestOrchestratorWebServices(t *testing.T) {
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"web-dev-01.example.com",
		"web-dev-02.example.com",
		"web-prod-01.example.com",
		"db-prod-01.example.com",
		"db-prod-02.example.com",
		"cache-prod-01.example.com",
	}

	orchestrator := NewOrchestrator(len(domains))

	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)
	assert.NotEmpty(t, patterns, "Expected patterns for realistic scenario")

	// Verify patterns cover all or most domains
	coveredDomains := make(map[string]bool)
	for _, pattern := range patterns {
		for _, d := range pattern.Domains {
			coveredDomains[d] = true
		}
	}

	coverageRatio := float64(len(coveredDomains)) / float64(len(domains))
	assert.GreaterOrEqual(t, coverageRatio, 0.7, "Expected at least 70%% domain coverage")

	t.Logf("Coverage: %d/%d domains (%.1f%%)", len(coveredDomains), len(domains), coverageRatio*100)
	t.Logf("Patterns generated: %d", len(patterns))
}

// TestOrchestratorModeDetection tests mode selection based on input size
func TestOrchestratorModeDetection(t *testing.T) {
	tests := []struct {
		name         string
		domainCount  int
		expectedMode string
	}{
		{"THOROUGH mode", 50, "THOROUGH"},
		{"BALANCED mode", 200, "BALANCED"},
		{"FAST mode", 1500, "FAST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate dummy domains
			domains := make([]string, tt.domainCount)
			for i := 0; i < tt.domainCount; i++ {
				domains[i] = "test.example.com"
			}

			orchestrator := NewOrchestrator(tt.domainCount)
			_, err := orchestrator.LearnPatterns(domains)
			require.NoError(t, err)

			stats := orchestrator.GetStats()
			assert.Equal(t, tt.expectedMode, stats.Mode, "Mode detection failed")
			t.Logf("✓ %d domains → %s mode", tt.domainCount, stats.Mode)
		})
	}
}

// TestOrchestratorDifferentPatterns tests various pattern types
func TestOrchestratorDifferentPatterns(t *testing.T) {
	tests := []struct {
		name            string
		domains         []string
		minPatterns     int
		minCoverage     float64
		expectVariables bool
	}{
		{
			name: "service-environment pattern",
			domains: []string{
				"api-dev.example.com",
				"api-staging.example.com",
				"api-prod.example.com",
				"web-dev.example.com",
				"web-prod.example.com",
			},
			minPatterns:     1,
			minCoverage:     0.8,
			expectVariables: true,
		},
		{
			name: "numbered services",
			domains: []string{
				"web-01.example.com",
				"web-02.example.com",
				"web-03.example.com",
				"api-01.example.com",
				"api-02.example.com",
			},
			minPatterns:     1,
			minCoverage:     0.8,
			expectVariables: true,
		},
		{
			name: "regional pattern",
			domains: []string{
				"us-east-api.example.com",
				"us-west-api.example.com",
				"eu-central-api.example.com",
				"ap-south-api.example.com",
			},
			minPatterns:     1,
			minCoverage:     0.7,
			expectVariables: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator := NewOrchestrator(len(tt.domains))

			patterns, err := orchestrator.LearnPatterns(tt.domains)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(patterns), tt.minPatterns,
				"Expected at least %d patterns", tt.minPatterns)

			// Check coverage
			coveredDomains := make(map[string]bool)
			for _, pattern := range patterns {
				for _, d := range pattern.Domains {
					coveredDomains[d] = true
				}
			}
			coverageRatio := float64(len(coveredDomains)) / float64(len(tt.domains))
			assert.GreaterOrEqual(t, coverageRatio, tt.minCoverage,
				"Expected at least %.0f%% coverage", tt.minCoverage*100)

			// Check for variables if expected
			if tt.expectVariables {
				foundVariables := false
				for _, pattern := range patterns {
					if len(pattern.Variables) > 0 {
						foundVariables = true
						break
					}
				}
				assert.True(t, foundVariables, "Expected patterns with variables")
			}

			t.Logf("Patterns: %d, Coverage: %.1f%%", len(patterns), coverageRatio*100)
		})
	}
}

// TestOrchestratorParallelization tests that parallelization doesn't break results
func TestOrchestratorParallelization(t *testing.T) {
	// Generate enough domains to trigger parallelization
	domains := make([]string, 0, 300)
	services := []string{"api", "web", "cdn", "db", "cache"}
	envs := []string{"dev", "staging", "prod"}

	for _, svc := range services {
		for _, env := range envs {
			for i := 1; i <= 20; i++ {
				domains = append(domains, ""+svc+"-"+env+".example.com")
			}
		}
	}

	orchestrator := NewOrchestrator(len(domains))

	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)
	assert.NotEmpty(t, patterns)

	// Run multiple times to check consistency (parallelization shouldn't cause non-determinism in pattern count)
	for i := 0; i < 3; i++ {
		orchestrator2 := NewOrchestrator(len(domains))
		patterns2, err2 := orchestrator2.LearnPatterns(domains)
		require.NoError(t, err2)

		// Pattern count should be consistent (allowing small variance due to AP clustering randomness)
		assert.InDelta(t, len(patterns), len(patterns2), 2.0,
			"Pattern count should be consistent across runs")
	}

	stats := orchestrator.GetStats()
	t.Logf("Mode: %s, Patterns: %d, Raw: %d", stats.Mode, stats.FinalPatterns, stats.RawPatterns)
}

// TestOrchestratorStatistics tests statistics tracking
func TestOrchestratorStatistics(t *testing.T) {
	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"web-dev.example.com",
		"web-prod.example.com",
	}

	orchestrator := NewOrchestrator(len(domains))
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)

	stats := orchestrator.GetStats()

	// Verify all statistics are populated
	assert.Equal(t, len(domains), stats.InputDomains)
	assert.Equal(t, len(domains), stats.FilteredDomains)
	assert.Greater(t, stats.LevelGroups, 0)
	assert.GreaterOrEqual(t, stats.Strategy1Patterns, 0)
	assert.GreaterOrEqual(t, stats.Strategy3Patterns, 0) // Strategy 3 always runs
	assert.Equal(t, len(patterns), stats.FinalPatterns)
	assert.NotEmpty(t, stats.Mode)

	// Verify pipeline flow: Raw → Dedup → AP → Final
	assert.GreaterOrEqual(t, stats.RawPatterns, stats.AfterDedup)
	assert.GreaterOrEqual(t, stats.AfterDedup, stats.AfterAP)
	assert.GreaterOrEqual(t, stats.AfterAP, stats.FinalPatterns)

	t.Logf("Statistics: %+v", stats)
}

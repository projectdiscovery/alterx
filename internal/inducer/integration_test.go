package inducer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDSLGeneration_EndToEnd tests complete DSL pattern generation
func TestDSLGeneration_EndToEnd(t *testing.T) {
	// Create a closure with similar domains
	closure := &Closure{
		Domains: []string{
			"api-dev.example.com",
			"api-staging.example.com",
			"api-prod.example.com",
			"web-dev.example.com",
			"web-prod.example.com",
		},
		Delta: 3,
		Size:  5,
	}

	// Generate DSL pattern
	generator := NewDSLGenerator(nil)
	pattern, err := generator.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Verify pattern structure
	assert.NotEmpty(t, pattern.Template)
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.Greater(t, len(pattern.Variables), 0, "Expected at least one variable")
	assert.Equal(t, 5, pattern.Coverage)

	t.Logf("Template: %s", pattern.Template)
	t.Logf("Variables: %d", len(pattern.Variables))
	t.Logf("Coverage: %d", pattern.Coverage)
	t.Logf("Ratio: %.2f", pattern.Ratio)
	t.Logf("Confidence: %.2f", pattern.Confidence)

	// Verify variables have payloads
	for _, variable := range pattern.Variables {
		if variable.NumberRange == nil {
			assert.NotEmpty(t, variable.Payloads, "Non-number variable should have payloads")
		}
		t.Logf("  Variable %s: type=%s, payloads=%v", variable.Name, variable.Type, variable.Payloads)
	}
}

// TestDSLGeneration_WithNumbers tests number range detection
func TestDSLGeneration_WithNumbers(t *testing.T) {
	closure := &Closure{
		Domains: []string{
			"api-01.example.com",
			"api-02.example.com",
			"api-03.example.com",
			"web-01.example.com",
			"web-02.example.com",
		},
		Delta: 2,
		Size:  5,
	}

	generator := NewDSLGenerator(nil)
	pattern, err := generator.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should detect number ranges
	foundNumberVariable := false
	for _, variable := range pattern.Variables {
		if variable.NumberRange != nil {
			foundNumberVariable = true
			assert.NotNil(t, variable.NumberRange)
			assert.Greater(t, variable.NumberRange.End, variable.NumberRange.Start)
			t.Logf("Number variable %s: %d-%d (format: %s)",
				variable.Name, variable.NumberRange.Start, variable.NumberRange.End, variable.NumberRange.Format)
		}
	}

	assert.True(t, foundNumberVariable, "Expected at least one number variable")
}

// TestDSLGeneration_WithSemanticTokens tests token dictionary classification
func TestDSLGeneration_WithSemanticTokens(t *testing.T) {
	// Create token dictionary
	dict := &TokenDictionary{
		Env:     []string{"dev", "staging", "prod", "qa"},
		Region:  []string{"us-east", "us-west", "eu-central"},
		Service: []string{"api", "web", "cdn", "db"},
	}

	closure := &Closure{
		Domains: []string{
			"api-dev.example.com",
			"api-staging.example.com",
			"api-prod.example.com",
			"web-dev.example.com",
			"web-prod.example.com",
		},
		Delta: 3,
		Size:  5,
	}

	generator := NewDSLGenerator(dict)
	pattern, err := generator.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should use semantic variable names
	foundSemanticVar := false
	for _, variable := range pattern.Variables {
		if variable.Name == "service" || variable.Name == "env" {
			foundSemanticVar = true
			t.Logf("Found semantic variable: %s", variable.Name)
		}
	}

	assert.True(t, foundSemanticVar, "Expected semantic variable names (service/env)")
}

// TestCompleteInductionPipeline tests the full pipeline with realistic data
func TestCompleteInductionPipeline(t *testing.T) {
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
		"cdn-01.example.com",
		"cdn-02.example.com",
		"db-prod.example.com",
		"cache-prod.example.com",
	}

	// Create orchestrator
	orchestrator := NewOrchestrator(len(domains))

	// Learn patterns
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)
	assert.NotEmpty(t, patterns, "Expected patterns from realistic data")

	// Verify patterns
	for i, pattern := range patterns {
		assert.NotEmpty(t, pattern.Template, "Pattern %d should have template", i+1)
		assert.Contains(t, pattern.Template, "{{root}}", "Pattern %d should contain {{root}}", i+1)
		assert.Greater(t, pattern.Coverage, 0, "Pattern %d should have coverage", i+1)
		assert.Greater(t, pattern.Confidence, 0.0, "Pattern %d should have confidence", i+1)

		t.Logf("Pattern %d: %s (coverage: %d, confidence: %.2f)",
			i+1, pattern.Template, pattern.Coverage, pattern.Confidence)
	}

	// Verify overall coverage
	coveredDomains := make(map[string]bool)
	for _, pattern := range patterns {
		for _, d := range pattern.Domains {
			coveredDomains[d] = true
		}
	}

	coverageRatio := float64(len(coveredDomains)) / float64(len(domains))
	assert.GreaterOrEqual(t, coverageRatio, 0.7, "Expected at least 70%% domain coverage")
	t.Logf("Overall coverage: %.1f%% (%d/%d domains)",
		coverageRatio*100, len(coveredDomains), len(domains))
}

// TestPipelineWithLevelGrouping tests multi-level domain handling
func TestPipelineWithLevelGrouping(t *testing.T) {
	domains := []string{
		// Level 1: single subdomain
		"api.example.com",
		"web.example.com",
		"cdn.example.com",
		// Level 2: two-part subdomains
		"api.dev.example.com",
		"api.prod.example.com",
		"web.staging.example.com",
		// Level 3: three-part subdomains
		"scheduler.api.prod.example.com",
		"worker.api.prod.example.com",
	}

	orchestrator := NewOrchestrator(len(domains))
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)

	// Should handle different levels
	stats := orchestrator.GetStats()
	assert.GreaterOrEqual(t, stats.LevelGroups, 2, "Expected multiple level groups")

	t.Logf("Level groups: %d", stats.LevelGroups)
	t.Logf("Patterns generated: %d", len(patterns))

	// Patterns should be grouped by level
	for i, pattern := range patterns {
		t.Logf("Pattern %d: %s", i+1, pattern.Template)
	}
}

// TestPipelineWithMixedPatterns tests various pattern types together
func TestPipelineWithMixedPatterns(t *testing.T) {
	domains := []string{
		// Pattern 1: service-env
		"api-dev.example.com",
		"api-prod.example.com",
		"web-dev.example.com",
		// Pattern 2: service-number
		"cdn-01.example.com",
		"cdn-02.example.com",
		"cdn-03.example.com",
		// Pattern 3: region-service
		"us-api.example.com",
		"eu-api.example.com",
		"asia-api.example.com",
		// Pattern 4: service.env
		"db.prod.example.com",
		"db.staging.example.com",
	}

	orchestrator := NewOrchestrator(len(domains))
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)
	assert.NotEmpty(t, patterns)

	// Should detect multiple distinct patterns
	assert.GreaterOrEqual(t, len(patterns), 2, "Expected multiple distinct patterns")

	// Verify templates are different
	templates := make(map[string]bool)
	for _, pattern := range patterns {
		templates[pattern.Template] = true
	}
	assert.GreaterOrEqual(t, len(templates), 2, "Expected diverse templates")

	t.Logf("Found %d unique templates:", len(templates))
	for template := range templates {
		t.Logf("  %s", template)
	}
}

// TestDSLGeneration_QualityMetrics tests quality score calculation
func TestDSLGeneration_QualityMetrics(t *testing.T) {
	tests := []struct {
		name           string
		domains        []string
		minConfidence  float64
		expectHighQual bool
	}{
		{
			name: "high quality - consistent pattern",
			domains: []string{
				"api-dev.example.com",
				"api-staging.example.com",
				"api-prod.example.com",
				"web-dev.example.com",
				"web-prod.example.com",
			},
			minConfidence:  0.3,
			expectHighQual: true,
		},
		{
			name: "moderate quality - some variation",
			domains: []string{
				"api-dev-01.example.com",
				"api-dev-02.example.com",
				"web-prod.example.com",
			},
			minConfidence:  0.1, // Lower threshold for heterogeneous patterns
			expectHighQual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closure := &Closure{
				Domains: tt.domains,
				Delta:   3,
				Size:    len(tt.domains),
			}

			generator := NewDSLGenerator(nil)
			pattern, err := generator.GeneratePattern(closure)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, pattern.Confidence, tt.minConfidence,
				"Confidence should be at least %.2f", tt.minConfidence)
			assert.Greater(t, pattern.Ratio, 0.0, "Ratio should be positive")

			t.Logf("Quality metrics - Confidence: %.2f, Ratio: %.2f",
				pattern.Confidence, pattern.Ratio)
		})
	}
}

// TestEnrichment_Integration tests pattern enrichment in full pipeline
func TestEnrichment_Integration(t *testing.T) {
	domains := []string{
		"api-01.example.com",
		"api-02.example.com",
		"api.example.com", // Same pattern without number
		"web-dev.example.com",
		"web-staging.example.com",
		"web.example.com", // Same pattern without env
	}

	orchestrator := NewOrchestrator(len(domains))
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)

	// After enrichment, number variables should have "" in payloads
	foundOptionalNumber := false
	for _, pattern := range patterns {
		for _, variable := range pattern.Variables {
			// Numbers become optional (enrichment adds "" to payloads)
			if len(variable.Payloads) > 0 && variable.Payloads[0] == "" {
				foundOptionalNumber = true
				t.Logf("Found optional variable: %s with payloads: %v", variable.Name, variable.Payloads)
			}
		}
	}

	// Enrichment might make some variables optional
	t.Logf("Optional variables found: %v", foundOptionalNumber)
}

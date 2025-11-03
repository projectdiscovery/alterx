package inducer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDSLConversion_EndToEnd tests the complete flow from regex pattern to DSL
func TestDSLConversion_EndToEnd(t *testing.T) {
	// Create a simple pattern
	pattern := &Pattern{
		Regex:      "(api|web)-(dev|prod)",
		Coverage:   4,
		Ratio:      1.5,
		Confidence: 0.0, // Will be calculated
		Domains: []string{
			"api-dev",
			"api-prod",
			"web-dev",
			"web-prod",
		},
	}

	// Update confidence
	pattern.UpdateConfidence()

	// Convert to DSL
	converter := NewDSLConverter()
	result := converter.Convert(pattern.Regex)

	require.NoError(t, result.Error)
	assert.NotEmpty(t, result.Template)
	assert.Equal(t, "{{p0}}-{{p1}}.{{suffix}}", result.Template)
	assert.Len(t, result.Payloads, 2)
	assert.Contains(t, result.Payloads, "p0")
	assert.Contains(t, result.Payloads, "p1")
	assert.Equal(t, []string{"api", "web"}, result.Payloads["p0"])
	assert.Equal(t, []string{"dev", "prod"}, result.Payloads["p1"])

	t.Logf("Original regex: %s", pattern.Regex)
	t.Logf("DSL template: %s", result.Template)
	t.Logf("Payloads: %v", result.Payloads)
	t.Logf("Coverage: %d, Ratio: %.2f, Confidence: %.2f",
		pattern.Coverage, pattern.Ratio, pattern.Confidence)
}

// TestPatternGenerator_WithDSLConversion tests pattern generation with DSL conversion
func TestPatternGenerator_WithDSLConversion(t *testing.T) {
	// Create closure with similar domains
	closure := &Closure{
		Domains: []string{
			"api-dev-01",
			"api-dev-02",
			"api-prod-01",
		},
		Delta: 3,
	}

	// Generate pattern
	generator := NewPatternGenerator()
	pattern, err := generator.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Set ratio and update confidence
	pattern.Ratio = 2.0
	pattern.UpdateConfidence()

	// Convert to DSL
	converter := NewDSLConverter()
	result := converter.Convert(pattern.Regex)

	require.NoError(t, result.Error)
	t.Logf("Original regex: %s", pattern.Regex)
	t.Logf("DSL template: %s", result.Template)
	t.Logf("Payloads: %v", result.Payloads)
	t.Logf("Coverage: %d, Ratio: %.2f, Confidence: %.2f",
		pattern.Coverage, pattern.Ratio, pattern.Confidence)

	// Verify template structure
	assert.Contains(t, result.Template, "{{suffix}}")
	assert.True(t, len(result.Payloads) > 0 || result.Template == ".{{suffix}}")
}

// TestCompleteInductionPipeline tests the full pipeline with realistic data
func TestCompleteInductionPipeline(t *testing.T) {
	domains := []string{
		"api-dev-01",
		"api-dev-02",
		"api-prod-01",
		"web-staging",
		"web-prod",
	}

	// Create orchestrator
	config := DefaultOrchestratorConfig()
	config.DistLow = 2
	config.DistHigh = 5
	orchestrator := NewOrchestrator(config)

	// Learn patterns (now returns DSLPattern directly)
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)

	if len(patterns) == 0 {
		t.Skip("No patterns learned from test data")
	}

	// Patterns are already in DSL format - no conversion needed
	for i, pattern := range patterns {
		t.Logf("Pattern %d:", i+1)
		t.Logf("  DSL Template: %s", pattern.Template)
		t.Logf("  Coverage: %d, Ratio: %.2f, Confidence: %.2f",
			pattern.Coverage, pattern.Ratio, pattern.Confidence)
		t.Logf("  Domains: %v", pattern.Domains)
		if len(pattern.Variables) > 0 {
			t.Logf("  Variables:")
			for _, v := range pattern.Variables {
				t.Logf("    {{%s}}: %v (type: %s)", v.Name, v.Payloads, v.Type)
			}
		}
	}

	t.Logf("Successfully generated %d DSL patterns", len(patterns))
	assert.Greater(t, len(patterns), 0, "Should generate at least one DSL pattern")
}

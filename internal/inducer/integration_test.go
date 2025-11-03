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

	// Learn patterns
	patterns, err := orchestrator.LearnPatterns(domains)
	require.NoError(t, err)

	if len(patterns) == 0 {
		t.Skip("No patterns learned from test data")
	}

	// Convert all patterns to DSL
	converter := NewDSLConverter()
	dslTemplates := make([]string, 0)

	for i, pattern := range patterns {
		t.Logf("Pattern %d before conversion:", i+1)
		t.Logf("  Regex: '%s' (len=%d)", pattern.Regex, len(pattern.Regex))
		t.Logf("  Coverage: %d, Ratio: %.2f, Confidence: %.2f",
			pattern.Coverage, pattern.Ratio, pattern.Confidence)
		t.Logf("  Domains: %v", pattern.Domains)

		result := converter.Convert(pattern.Regex)
		if result.Error != nil {
			t.Logf("Warning: Failed to convert pattern %d: %v", i+1, result.Error)
			continue
		}

		err := converter.ValidateTemplate(result.Template, result.Payloads)
		if err != nil {
			t.Logf("Warning: Invalid template %d: %v", i+1, err)
			continue
		}

		dslTemplates = append(dslTemplates, result.Template)

		t.Logf("Pattern %d:", i+1)
		t.Logf("  Regex: %s", pattern.Regex)
		t.Logf("  DSL: %s", result.Template)
		t.Logf("  Coverage: %d, Ratio: %.2f, Confidence: %.2f",
			pattern.Coverage, pattern.Ratio, pattern.Confidence)
		if len(result.Payloads) > 0 {
			t.Logf("  Payloads: %v", result.Payloads)
		}
	}

	t.Logf("Successfully converted %d/%d patterns to DSL", len(dslTemplates), len(patterns))
	assert.Greater(t, len(dslTemplates), 0, "Should convert at least one pattern to DSL")
}

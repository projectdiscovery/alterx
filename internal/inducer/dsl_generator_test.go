package inducer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// LEVEL-BASED GROUPING TESTS
// ============================================================================

func TestDSLGenerator_OneLevelPattern(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api.projectdiscovery.io",
		"cdn.projectdiscovery.io",
		"dev.projectdiscovery.io",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   3,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should generate: {{p0}}.{{root}}
	assert.Equal(t, "{{p0}}.{{root}}", pattern.Template)
	assert.Equal(t, 1, pattern.LevelCount)
	assert.NotContains(t, pattern.Template, "{{suffix}}", "Should not use {{suffix}}")

	// Check p0 variable
	var p0Var *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "p0" {
			p0Var = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, p0Var, "Should have p0 variable")
	assert.ElementsMatch(t, []string{"api", "cdn", "dev"}, p0Var.Payloads)

	// Check quality metrics
	assert.Equal(t, 3, pattern.Coverage)
	assert.InDelta(t, 1.0, pattern.Ratio, 0.1, "Ratio should be ~1.0 for perfect pattern")
}

func TestDSLGenerator_TwoLevelPattern(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"scheduler.api.projectdiscovery.io",
		"scheduler.dev.projectdiscovery.io",
		"webhook.api.projectdiscovery.io",
		"webhook.dev.projectdiscovery.io",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should generate: {{p0}}.{{p1}}.{{root}}
	assert.Equal(t, "{{p0}}.{{p1}}.{{root}}", pattern.Template)
	assert.Equal(t, 2, pattern.LevelCount)
	assert.NotContains(t, pattern.Template, "{{suffix}}", "Should not use {{suffix}}")

	// Check p0 variable (leftmost level)
	var p0Var *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "p0" {
			p0Var = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, p0Var, "Should have p0 variable")
	assert.ElementsMatch(t, []string{"scheduler", "webhook"}, p0Var.Payloads)

	// Check p1 variable (second level)
	var p1Var *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "p1" {
			p1Var = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, p1Var, "Should have p1 variable")
	assert.ElementsMatch(t, []string{"api", "dev"}, p1Var.Payloads)

	// Check quality metrics - should be perfect (2 x 2 = 4 generations, 4 observed)
	assert.Equal(t, 4, pattern.Coverage)
	assert.InDelta(t, 1.0, pattern.Ratio, 0.1, "Ratio should be ~1.0 for perfect pattern")
}

func TestDSLGenerator_ThreeLevelPattern(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"scheduler.alpha.api.projectdiscovery.io",
		"scheduler.alpha.dev.projectdiscovery.io",
		"scheduler.beta.api.projectdiscovery.io",
		"webhook.alpha.api.projectdiscovery.io",
		"webhook.beta.dev.projectdiscovery.io",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   10,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should generate: {{p0}}.{{p1}}.{{p2}}.{{root}}
	assert.Equal(t, "{{p0}}.{{p1}}.{{p2}}.{{root}}", pattern.Template)
	assert.Equal(t, 3, pattern.LevelCount)
	assert.NotContains(t, pattern.Template, "{{suffix}}", "Should not use {{suffix}}")

	// Check p0 variable
	var p0Var *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "p0" {
			p0Var = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, p0Var, "Should have p0 variable")
	assert.ElementsMatch(t, []string{"scheduler", "webhook"}, p0Var.Payloads)

	// Check p1 variable
	var p1Var *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "p1" {
			p1Var = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, p1Var, "Should have p1 variable")
	assert.ElementsMatch(t, []string{"alpha", "beta"}, p1Var.Payloads)

	// Check p2 variable
	var p2Var *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "p2" {
			p2Var = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, p2Var, "Should have p2 variable")
	assert.ElementsMatch(t, []string{"api", "dev"}, p2Var.Payloads)

	// Check quality metrics
	assert.Equal(t, 5, pattern.Coverage)
	// Cartesian product: 2 x 2 x 2 = 8 generations, 5 observed → ratio ~1.6
	assert.Less(t, pattern.Ratio, 3.0, "Ratio should be reasonable")
}

func TestDSLGenerator_SemanticClassificationWithRoot(t *testing.T) {
	dictionary := &TokenDictionary{
		Service: []string{"scheduler", "webhook"},
		Env:     []string{"dev", "prod", "staging", "qa"},
	}

	gen := NewDSLGenerator(dictionary)

	domains := []string{
		"scheduler.dev.projectdiscovery.io",
		"scheduler.prod.projectdiscovery.io",
		"webhook.dev.projectdiscovery.io",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should generate: {{service}}.{{env}}.{{root}} (semantic classification)
	assert.Equal(t, "{{service}}.{{env}}.{{root}}", pattern.Template)
	assert.Equal(t, 2, pattern.LevelCount)
	assert.NotContains(t, pattern.Template, "{{suffix}}", "Should not use {{suffix}}")

	// Check service variable
	var serviceVar *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "service" {
			serviceVar = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, serviceVar)
	assert.ElementsMatch(t, []string{"scheduler", "webhook"}, serviceVar.Payloads)

	// Check env variable
	var envVar *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "env" {
			envVar = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, envVar)
	assert.ElementsMatch(t, []string{"dev", "prod"}, envVar.Payloads)
}

// ============================================================================
// ORIGINAL TESTS - UPDATED TO USE {{root}}
// ============================================================================

func TestDSLGenerator_SimpleNumberVariation(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   3,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should generate: {{p0}}-{{p1}}-{{number}}.{{root}}
	// Without semantic dictionary, "api" and "dev" become positional variables (p0, p1)
	// This prevents hardcoded literals and makes patterns more general
	assert.Contains(t, pattern.Template, "{{p0}}")
	assert.Contains(t, pattern.Template, "{{p1}}")
	assert.Contains(t, pattern.Template, "{{number}}")
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.NotContains(t, pattern.Template, "{{suffix}}")

	// Should have number variable with structured NumberRange (not literal values)
	var numberVar *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "number" {
			numberVar = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, numberVar, "Should have number variable")
	require.NotNil(t, numberVar.NumberRange, "Should have structured NumberRange")
	assert.Equal(t, 0, numberVar.NumberRange.Start, "Start should be 0")
	// End is max observed value + buffer (3 observed: 01, 02, 03 → end=4)
	assert.GreaterOrEqual(t, numberVar.NumberRange.End, 3, "End should be >= max observed")
	assert.Equal(t, "%02d", numberVar.NumberRange.Format, "Format should preserve leading zeros")
	assert.Equal(t, TokenTypeNumber, numberVar.Type)

	// Check quality metrics
	assert.Equal(t, 3, pattern.Coverage)
	assert.Greater(t, pattern.Ratio, 0.0)
	assert.Greater(t, pattern.Confidence, 0.0)
	assert.LessOrEqual(t, pattern.Confidence, 1.0)
}

func TestDSLGenerator_SemanticClassification(t *testing.T) {
	dictionary := &TokenDictionary{
		Env:     []string{"dev", "prod", "staging", "qa"},
		Service: []string{"api", "web", "cdn", "db"},
		Region:  []string{"us-east-1", "us-west-2", "eu-central-1"},
	}

	gen := NewDSLGenerator(dictionary)

	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"web-staging.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Should generate: {{service}}-{{env}}.{{root}}
	// With dictionary, tokens are semantically classified
	assert.Equal(t, "{{service}}-{{env}}.{{root}}", pattern.Template)
	assert.NotContains(t, pattern.Template, "{{suffix}}")

	// Check service variable
	var serviceVar *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "service" {
			serviceVar = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, serviceVar)
	assert.ElementsMatch(t, []string{"api", "web"}, serviceVar.Payloads)

	// Check env variable
	var envVar *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "env" {
			envVar = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, envVar)
	assert.ElementsMatch(t, []string{"dev", "prod", "staging"}, envVar.Payloads)
}

func TestDSLGenerator_HybridClassification(t *testing.T) {
	// Partial dictionary - some tokens will match, others won't
	dictionary := &TokenDictionary{
		Env: []string{"dev", "prod"},
	}

	gen := NewDSLGenerator(dictionary)

	domains := []string{
		"api-dev-01.example.com",
		"web-prod-02.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   10,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)

	// "dev" and "prod" should match dictionary → {{env}}
	// "api" and "web" don't match → {{p0}}
	// "01" and "02" are numbers → {{number}}
	assert.Equal(t, "{{p0}}-{{env}}-{{number}}.{{root}}", pattern.Template)
	assert.NotContains(t, pattern.Template, "{{suffix}}")

	// Verify each variable has correct payloads
	varMap := make(map[string][]string)
	for _, v := range pattern.Variables {
		varMap[v.Name] = v.Payloads
	}

	assert.ElementsMatch(t, []string{"api", "web"}, varMap["p0"])
	assert.ElementsMatch(t, []string{"dev", "prod"}, varMap["env"])

	// Number should be a structured NumberRange now
	var numberVar *DSLVariable
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == "number" {
			numberVar = &pattern.Variables[i]
			break
		}
	}
	require.NotNil(t, numberVar, "Should have number variable")
	require.NotNil(t, numberVar.NumberRange, "Should have structured NumberRange")
}

func TestDSLGenerator_MultiLevel(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api.staging.example.com",
		"api.prod.example.com",
		"web.staging.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)

	// Should generate pattern with multiple levels
	// {{p0}}.{{p1}}.{{root}}
	assert.Contains(t, pattern.Template, ".")
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.NotContains(t, pattern.Template, "{{suffix}}")

	// Should have alternations for both levels
	hasApiOrWeb := false
	hasStagingOrProd := false

	for _, v := range pattern.Variables {
		if containsInSlice(v.Payloads, "api") || containsInSlice(v.Payloads, "web") {
			hasApiOrWeb = true
		}
		if containsInSlice(v.Payloads, "staging") || containsInSlice(v.Payloads, "prod") {
			hasStagingOrProd = true
		}
	}

	assert.True(t, hasApiOrWeb, "Should have first level alternation")
	assert.True(t, hasStagingOrProd, "Should have second level alternation")
}

func TestDSLGenerator_QualityCalculation(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api-dev.example.com",
		"api-prod.example.com",
		"web-dev.example.com",
		"web-prod.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)

	// Pattern should be: {{p0}}-{{p0}}.{{root}}
	// Estimated generations: 2 × 2 = 4
	// Observed: 4
	// Ratio: 4 / 4 = 1.0 (perfect)
	assert.Equal(t, 4, pattern.Coverage)
	assert.InDelta(t, 1.0, pattern.Ratio, 0.1)

	// High quality ratio → high confidence
	assert.Greater(t, pattern.Confidence, 0.8)
}

func TestDSLGenerator_RatioTest(t *testing.T) {
	gen := NewDSLGenerator(nil)

	// Test ratio filtering threshold (ratio < 25)
	domains := make([]string, 10)
	for i := 0; i < 10; i++ {
		domains[i] = "web01.example.com" // Same domain repeated
	}

	closure := &Closure{
		Domains: domains,
		Delta:   1,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)

	// Should either succeed with low ratio or be filtered
	if err == nil {
		// If pattern generated, ratio should be reasonable
		assert.LessOrEqual(t, pattern.Ratio, 25.0)
	}
}

func TestDSLGenerator_DashHandling(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api-v1.example.com",
		"api-v2.example.com",
		"web-v1.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)

	// Should preserve dash structure
	assert.Contains(t, pattern.Template, "-")
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.NotContains(t, pattern.Template, "{{suffix}}")
}

func TestDSLGenerator_EmptyClosure(t *testing.T) {
	gen := NewDSLGenerator(nil)

	closure := &Closure{
		Domains: []string{},
		Delta:   0,
		Size:    0,
	}

	_, err := gen.GeneratePattern(closure)
	assert.Error(t, err, "Should fail for empty closure")
}

func TestDSLGenerator_SingleDomain(t *testing.T) {
	gen := NewDSLGenerator(nil)

	closure := &Closure{
		Domains: []string{"api.example.com"},
		Delta:   0,
		Size:    1,
	}

	_, err := gen.GeneratePattern(closure)
	assert.Error(t, err, "Should fail for single domain")
}

func TestDSLGenerator_ComplexPattern(t *testing.T) {
	dictionary := &TokenDictionary{
		Service: []string{"api", "web", "cdn"},
		Env:     []string{"dev", "prod", "staging"},
		Region:  []string{"us-east-1", "us-west-2"},
	}

	gen := NewDSLGenerator(dictionary)

	domains := []string{
		"api-dev-us-east-1-01.example.com",
		"api-prod-us-west-2-02.example.com",
		"web-staging-us-east-1-03.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   20,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)

	// Should generate: {{service}}-{{env}}-{{region}}-{{number}}.{{root}}
	// or similar with proper semantic classification
	assert.Contains(t, pattern.Template, "{{")
	assert.Contains(t, pattern.Template, "}}")
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.NotContains(t, pattern.Template, "{{suffix}}")

	// Should have at least 3 variables (service/word, env, number)
	assert.GreaterOrEqual(t, len(pattern.Variables), 3)
}

func TestDSLGenerator_ConfidenceScoring(t *testing.T) {
	gen := NewDSLGenerator(nil)

	testCases := []struct {
		name          string
		domains       []string
		minConfidence float64
		maxConfidence float64
	}{
		{
			name: "perfect pattern - high confidence",
			domains: []string{
				"api-dev.example.com",
				"api-prod.example.com",
				"web-dev.example.com",
				"web-prod.example.com",
			},
			minConfidence: 0.7,
			maxConfidence: 1.0,
		},
		{
			name: "varied pattern - lower confidence due to range expansion",
			domains: []string{
				"api01.example.com",
				"web02.example.com",
			},
			minConfidence: 0.1, // Lower due to range expansion (generates more than observed)
			maxConfidence: 0.5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			closure := &Closure{
				Domains: tc.domains,
				Delta:   10,
				Size:    len(tc.domains),
			}

			pattern, err := gen.GeneratePattern(closure)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, pattern.Confidence, tc.minConfidence)
			assert.LessOrEqual(t, pattern.Confidence, tc.maxConfidence)
		})
	}
}

func TestDSLGenerator_LiteralTokens(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"prefix-api-suffix.example.com",
		"prefix-web-suffix.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   10,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)

	// NEW BEHAVIOR: Single tokens become variables to prevent hardcoded literals
	// Template should be: {{p0}}-{{p1}}-{{p2}}.{{root}} where:
	//   p0 = [prefix], p1 = [api, web], p2 = [suffix]
	// This makes patterns more general and reusable
	assert.Contains(t, pattern.Template, "{{p0}}")
	assert.Contains(t, pattern.Template, "{{p1}}")
	assert.Contains(t, pattern.Template, "{{p2}}")
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.NotContains(t, pattern.Template, "{{suffix}}")

	// Verify that "prefix" and "suffix" are in payloads, not hardcoded in template
	hasPrefix := false
	hasSuffix := false
	for _, v := range pattern.Variables {
		for _, payload := range v.Payloads {
			if payload == "prefix" {
				hasPrefix = true
			}
			if payload == "suffix" {
				hasSuffix = true
			}
		}
	}
	assert.True(t, hasPrefix, "prefix should be in payloads")
	assert.True(t, hasSuffix, "suffix should be in payloads")
}

func TestCalculateQuality(t *testing.T) {
	tests := []struct {
		name          string
		coverage      int
		estimatedGens int
		expectedRatio float64
		minConfidence float64
		maxRatio      float64
	}{
		{
			name:          "perfect pattern",
			coverage:      100,
			estimatedGens: 100,
			expectedRatio: 1.0,
			minConfidence: 0.8,
			maxRatio:      1.0,
		},
		{
			name:          "moderate pattern",
			coverage:      50,
			estimatedGens: 100,
			expectedRatio: 2.0,
			minConfidence: 0.4,
			maxRatio:      2.0,
		},
		{
			name:          "low quality pattern - should be filtered",
			coverage:      10,
			estimatedGens: 300,
			expectedRatio: 30.0,
			minConfidence: 0.0,
			maxRatio:      30.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := float64(tt.estimatedGens) / float64(tt.coverage)
			assert.InDelta(t, tt.expectedRatio, ratio, 0.01)
			assert.LessOrEqual(t, ratio, tt.maxRatio)

			// Calculate confidence
			confidence := calculateConfidence(tt.coverage, ratio)
			assert.GreaterOrEqual(t, confidence, tt.minConfidence)
			assert.LessOrEqual(t, confidence, 1.0)
		})
	}
}

func TestDSLGenerator_GeneratePatternsFromClosures(t *testing.T) {
	gen := NewDSLGenerator(nil)

	closures := []*Closure{
		{
			Domains: []string{
				"api-dev-01.example.com",
				"api-dev-02.example.com",
			},
			Delta: 2,
			Size:  2,
		},
		{
			Domains: []string{
				"web-prod.example.com",
				"web-staging.example.com",
			},
			Delta: 5,
			Size:  2,
		},
		{
			Domains: []string{
				"single.example.com", // Should be skipped
			},
			Delta: 1,
			Size:  1,
		},
	}

	patterns := gen.GeneratePatternsFromClosures(closures)

	// Should generate 2 patterns (third closure has single domain)
	assert.GreaterOrEqual(t, len(patterns), 2)

	// Each pattern should have reasonable coverage
	for _, pattern := range patterns {
		assert.GreaterOrEqual(t, pattern.Coverage, 2)
		assert.NotEmpty(t, pattern.Template)
		assert.Contains(t, pattern.Template, "{{root}}")
		assert.NotContains(t, pattern.Template, "{{suffix}}")
	}
}

// Helper function for tests
func containsInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TestDesignDocExample verifies the exact example from LEVEL_BASED_GROUPING.md
func TestDesignDocExample(t *testing.T) {
	gen := NewDSLGenerator(nil)

	// Example from design doc
	domains := []string{
		"api.projectdiscovery.io",
		"cdn.projectdiscovery.io",
		"scheduler.api.projectdiscovery.io",
		"scheduler.dev.projectdiscovery.io",
		"webhook.api.projectdiscovery.io",
		"webhook.dev.projectdiscovery.io",
	}

	// Group by level count
	levelGroups := make(map[int][]string)
	for _, domain := range domains {
		td, err := Tokenize(domain)
		if err != nil {
			continue
		}
		levelCount := len(td.Levels)
		levelGroups[levelCount] = append(levelGroups[levelCount], domain)
	}

	// Test 1-level pattern
	if oneLevelDomains, ok := levelGroups[1]; ok && len(oneLevelDomains) >= 2 {
		closure := &Closure{
			Domains: oneLevelDomains,
			Delta:   5,
			Size:    len(oneLevelDomains),
		}

		pattern, err := gen.GeneratePattern(closure)
		require.NoError(t, err)
		assert.Equal(t, "{{p0}}.{{root}}", pattern.Template)
		assert.Equal(t, 1, pattern.LevelCount)

		t.Logf("1-level pattern: %s", pattern.Template)
		t.Logf("  Variables: %+v", pattern.Variables)
	}

	// Test 2-level pattern
	if twoLevelDomains, ok := levelGroups[2]; ok && len(twoLevelDomains) >= 2 {
		closure := &Closure{
			Domains: twoLevelDomains,
			Delta:   5,
			Size:    len(twoLevelDomains),
		}

		pattern, err := gen.GeneratePattern(closure)
		require.NoError(t, err)
		assert.Equal(t, "{{p0}}.{{p1}}.{{root}}", pattern.Template)
		assert.Equal(t, 2, pattern.LevelCount)

		// Verify cross-cutting pattern discovery
		var p0Var, p1Var *DSLVariable
		for i := range pattern.Variables {
			if pattern.Variables[i].Name == "p0" {
				p0Var = &pattern.Variables[i]
			}
			if pattern.Variables[i].Name == "p1" {
				p1Var = &pattern.Variables[i]
			}
		}

		require.NotNil(t, p0Var)
		require.NotNil(t, p1Var)

		// Key insight: "api" and "dev" are now VARIABLE payloads, not hardcoded!
		assert.ElementsMatch(t, []string{"scheduler", "webhook"}, p0Var.Payloads)
		assert.ElementsMatch(t, []string{"api", "dev"}, p1Var.Payloads)

		t.Logf("2-level pattern: %s", pattern.Template)
		t.Logf("  p0 (service): %v", p0Var.Payloads)
		t.Logf("  p1 (env): %v", p1Var.Payloads)
		t.Logf("  ✓ Discovered cross-cutting pattern: both 'api' and 'dev' are variable!")
	}
}

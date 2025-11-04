package inducer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntropyBasedSelection_BasicScenario tests entropy selection with simple data
func TestEntropyBasedSelection_BasicScenario(t *testing.T) {
	// Create patterns with varying coverage
	patterns := []*DSLPattern{
		{
			Template:   "{{service}}-{{env}}.{{root}}",
			Coverage:   20,
			Confidence: 0.85,
			Domains:    generateDomains("pattern1", 20),
		},
		{
			Template:   "{{service}}-{{number}}.{{root}}",
			Coverage:   15,
			Confidence: 0.80,
			Domains:    generateDomains("pattern2", 15),
		},
		{
			Template:   "{{region}}-{{service}}.{{root}}",
			Coverage:   10,
			Confidence: 0.75,
			Domains:    generateDomains("pattern3", 10),
		},
		{
			Template:   "{{service}}.{{env}}.{{root}}",
			Coverage:   5,
			Confidence: 0.70,
			Domains:    generateDomains("pattern4", 5),
		},
		{
			Template:   "internal-{{service}}.{{root}}",
			Coverage:   3,
			Confidence: 0.65,
			Domains:    generateDomains("pattern5", 3),
		},
	}

	// Create orchestrator with BALANCED mode config
	orchestrator := NewOrchestrator(100)

	// All domains (no overlap for simplicity)
	allDomains := make([]string, 0, 53)
	for _, p := range patterns {
		allDomains = append(allDomains, p.Domains...)
	}

	// Run entropy selection
	selected := orchestrator.selectPatternsByEntropy(patterns, allDomains)

	// Verify selection
	require.NotEmpty(t, selected, "Should select at least one pattern")
	assert.LessOrEqual(t, len(selected), orchestrator.modeConfig.MaxPatterns,
		"Should not exceed max patterns")
	assert.GreaterOrEqual(t, len(selected), orchestrator.modeConfig.MinPatterns,
		"Should meet minimum patterns")

	// Patterns should be ordered by coverage (highest first)
	for i := 1; i < len(selected); i++ {
		assert.GreaterOrEqual(t, selected[i-1].Coverage, selected[i].Coverage,
			"Patterns should be ordered by coverage")
	}

	t.Logf("Selected %d/%d patterns", len(selected), len(patterns))
	for i, p := range selected {
		t.Logf("  %d. %s (coverage: %d)", i+1, p.Template, p.Coverage)
	}
}

// TestEntropyBasedSelection_DiminishingReturns tests stopping on diminishing returns
func TestEntropyBasedSelection_DiminishingReturns(t *testing.T) {
	// Create many patterns with decreasing marginal value
	patterns := make([]*DSLPattern, 0, 30)

	// First pattern covers 50% of domains
	patterns = append(patterns, &DSLPattern{
		Template:   "pattern-01.{{root}}",
		Coverage:   50,
		Confidence: 0.90,
		Domains:    generateDomains("p01", 50),
	})

	// Next few patterns add significant coverage
	for i := 2; i <= 5; i++ {
		patterns = append(patterns, &DSLPattern{
			Template:   fmt.Sprintf("pattern-%02d.{{root}}", i),
			Coverage:   10,
			Confidence: 0.85,
			Domains:    generateDomains(fmt.Sprintf("p%02d", i), 10),
		})
	}

	// Remaining patterns have low marginal value (overlap)
	for i := 6; i <= 30; i++ {
		patterns = append(patterns, &DSLPattern{
			Template:   fmt.Sprintf("pattern-%02d.{{root}}", i),
			Coverage:   2,
			Confidence: 0.70,
			Domains:    generateDomains(fmt.Sprintf("p%02d", i), 2),
		})
	}

	orchestrator := NewOrchestrator(100)
	allDomains := make([]string, 0, 100)
	for _, p := range patterns {
		allDomains = append(allDomains, p.Domains...)
	}

	selected := orchestrator.selectPatternsByEntropy(patterns, allDomains)

	// Should stop early due to diminishing returns, not select all 30
	assert.Less(t, len(selected), 15, "Should stop on diminishing returns")
	t.Logf("Selected %d/%d patterns (stopped on diminishing returns)", len(selected), len(patterns))
}

// TestEntropyBasedSelection_CoverageTarget tests stopping at coverage goal
func TestEntropyBasedSelection_CoverageTarget(t *testing.T) {
	// Create patterns that together exceed coverage target
	patterns := make([]*DSLPattern, 0, 20)

	// Each pattern covers unique domains
	for i := 0; i < 20; i++ {
		patterns = append(patterns, &DSLPattern{
			Template:   fmt.Sprintf("pattern-%02d.{{root}}", i),
			Coverage:   10,
			Confidence: 0.80,
			Domains:    generateDomains(fmt.Sprintf("p%02d", i), 10),
		})
	}

	orchestrator := NewOrchestrator(100)
	allDomains := make([]string, 0, 200)
	for _, p := range patterns {
		allDomains = append(allDomains, p.Domains...)
	}

	selected := orchestrator.selectPatternsByEntropy(patterns, allDomains)

	// Calculate actual coverage
	covered := make(map[string]bool)
	for _, p := range selected {
		for _, d := range p.Domains {
			covered[d] = true
		}
	}

	coverageRatio := float64(len(covered)) / float64(len(allDomains))

	// Should stop around target coverage (90%)
	assert.GreaterOrEqual(t, coverageRatio, 0.85, "Should reach near target coverage")
	t.Logf("Selected %d patterns, coverage: %.1f%%", len(selected), coverageRatio*100)
}

// TestEntropyBasedSelection_ModeVariations tests different mode configurations
func TestEntropyBasedSelection_ModeVariations(t *testing.T) {
	tests := []struct {
		name            string
		inputSize       int
		expectedMode    string
		maxPatterns     int
		minPatterns     int
		elbowThreshold  float64
	}{
		{"THOROUGH mode", 50, "THOROUGH", 30, 8, 0.01},
		{"BALANCED mode", 200, "BALANCED", 25, 5, 0.02},
		{"FAST mode", 1500, "FAST", 20, 3, 0.03},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator := NewOrchestrator(tt.inputSize)

			// Verify mode configuration
			assert.Equal(t, tt.expectedMode, orchestrator.modeConfig.Mode.String())
			assert.Equal(t, tt.maxPatterns, orchestrator.modeConfig.MaxPatterns)
			assert.Equal(t, tt.minPatterns, orchestrator.modeConfig.MinPatterns)
			assert.Equal(t, tt.elbowThreshold, orchestrator.modeConfig.ElbowSensitivity)

			t.Logf("✓ Mode: %s, MinPatterns: %d, MaxPatterns: %d, Elbow: %.2f%%",
				tt.expectedMode, tt.minPatterns, tt.maxPatterns, tt.elbowThreshold*100)
		})
	}
}

// TestEntropyBasedSelection_SafetyFloor tests minimum pattern enforcement
func TestEntropyBasedSelection_SafetyFloor(t *testing.T) {
	// Create only a few patterns with low marginal value
	patterns := []*DSLPattern{
		{
			Template:   "pattern-01.{{root}}",
			Coverage:   50,
			Confidence: 0.90,
			Domains:    generateDomains("p1", 50),
		},
		{
			Template:   "pattern-02.{{root}}",
			Coverage:   2,
			Confidence: 0.60,
			Domains:    generateDomains("p2", 2),
		},
		{
			Template:   "pattern-03.{{root}}",
			Coverage:   1,
			Confidence: 0.55,
			Domains:    generateDomains("p3", 1),
		},
	}

	orchestrator := NewOrchestrator(100) // BALANCED mode: MinPatterns = 5
	allDomains := make([]string, 0, 53)
	for _, p := range patterns {
		allDomains = append(allDomains, p.Domains...)
	}

	selected := orchestrator.selectPatternsByEntropy(patterns, allDomains)

	// Should select all 3 patterns (even though marginal value is low)
	// because we don't have enough to reach MinPatterns
	assert.Equal(t, len(patterns), len(selected), "Should keep all patterns if below minimum")

	t.Logf("Safety floor: kept all %d patterns (MinPatterns=%d)",
		len(selected), orchestrator.modeConfig.MinPatterns)
}

// TestEntropyBasedSelection_OverlappingDomains tests handling of domain overlap
func TestEntropyBasedSelection_OverlappingDomains(t *testing.T) {
	// Create patterns with overlapping domain coverage
	sharedDomains := generateDomains("shared", 10)

	patterns := []*DSLPattern{
		{
			Template:   "{{service}}-{{env}}.{{root}}",
			Coverage:   15,
			Confidence: 0.85,
			Domains:    append(sharedDomains, generateDomains("p1-unique", 5)...),
		},
		{
			Template:   "{{service}}-{{number}}.{{root}}",
			Coverage:   15,
			Confidence: 0.80,
			Domains:    append(sharedDomains, generateDomains("p2-unique", 5)...),
		},
		{
			Template:   "{{region}}-{{service}}.{{root}}",
			Coverage:   10,
			Confidence: 0.75,
			Domains:    generateDomains("p3-unique", 10),
		},
	}

	orchestrator := NewOrchestrator(100)

	// All unique domains (30 total: 10 shared + 5 + 5 + 10)
	allDomains := append(sharedDomains,
		append(generateDomains("p1-unique", 5),
			append(generateDomains("p2-unique", 5),
				generateDomains("p3-unique", 10)...)...)...)

	selected := orchestrator.selectPatternsByEntropy(patterns, allDomains)

	// Entropy selection should handle overlap correctly
	// Second pattern should have lower marginal value due to overlap
	require.NotEmpty(t, selected)

	// Calculate effective coverage
	covered := make(map[string]bool)
	for _, p := range selected {
		for _, d := range p.Domains {
			covered[d] = true
		}
	}

	t.Logf("Selected %d patterns, effective coverage: %d/%d domains",
		len(selected), len(covered), len(allDomains))
}

// Helper function to generate dummy domains
func generateDomains(prefix string, count int) []string {
	domains := make([]string, count)
	for i := 0; i < count; i++ {
		domains[i] = fmt.Sprintf("%s-%02d.example.com", prefix, i)
	}
	return domains
}

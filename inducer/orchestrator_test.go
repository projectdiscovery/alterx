package inducer

import (
	"testing"
)

func TestOrchestrator_LearnPatterns_SmallDataset(t *testing.T) {
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
	}

	config := DefaultOrchestratorConfig("example.com")
	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected to learn at least one pattern")
	}

	// All patterns should have coverage >= 2
	for _, pattern := range patterns {
		if pattern.Coverage < 2 {
			t.Errorf("Pattern %s has coverage %d; want >= 2", pattern.Regex, pattern.Coverage)
		}
	}

	// Patterns should contain expected terms
	hasApiPattern := false
	hasWebPattern := false

	for _, pattern := range patterns {
		if contains(pattern.Regex, "api") {
			hasApiPattern = true
		}
		if contains(pattern.Regex, "web") {
			hasWebPattern = true
		}
	}

	if !hasApiPattern {
		t.Error("Expected to find pattern containing 'api'")
	}

	if !hasWebPattern {
		t.Error("Expected to find pattern containing 'web'")
	}
}

func TestOrchestrator_LearnPatterns_NumberVariations(t *testing.T) {
	domains := []string{
		"server-01.example.com",
		"server-02.example.com",
		"server-03.example.com",
		"server-04.example.com",
		"server-05.example.com",
	}

	config := DefaultOrchestratorConfig("example.com")
	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected to learn at least one pattern")
	}

	// Should find a pattern with number range
	hasNumberRange := false
	for _, pattern := range patterns {
		if contains(pattern.Regex, "[") && contains(pattern.Regex, "]") {
			hasNumberRange = true
		}
	}

	if !hasNumberRange {
		t.Error("Expected to find pattern with number range")
	}
}

func TestOrchestrator_LearnPatterns_EnvironmentVariations(t *testing.T) {
	domains := []string{
		"api.dev.example.com",
		"api.staging.example.com",
		"api.prod.example.com",
		"web.dev.example.com",
		"web.staging.example.com",
		"web.prod.example.com",
	}

	config := DefaultOrchestratorConfig("example.com")
	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected to learn at least one pattern")
	}

	// Should find patterns with alternations for environments
	hasAlternation := false
	for _, pattern := range patterns {
		if contains(pattern.Regex, "|") {
			hasAlternation = true
		}
	}

	if !hasAlternation {
		t.Error("Expected to find pattern with alternation (|)")
	}
}

func TestOrchestrator_LearnPatterns_EmptyInput(t *testing.T) {
	config := DefaultOrchestratorConfig("example.com")
	orchestrator := NewOrchestrator(config)

	_, err := orchestrator.LearnPatterns([]string{})
	if err == nil {
		t.Error("Expected error for empty input")
	}
}

func TestOrchestrator_LearnPatterns_SingleDomain(t *testing.T) {
	domains := []string{"api.example.com"}

	config := DefaultOrchestratorConfig("example.com")
	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	// Single domain should produce no patterns (need at least 2 for a pattern)
	if len(patterns) != 0 {
		t.Errorf("Expected 0 patterns for single domain, got %d", len(patterns))
	}
}

func TestOrchestrator_LearnPatterns_WithCompression(t *testing.T) {
	domains := []string{
		"api-01.example.com",
		"api-02.example.com",
		"api-03.example.com",
		"api-04.example.com",
		"api-05.example.com",
	}

	config := DefaultOrchestratorConfig("example.com")
	config.EnableCompression = true
	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	// Check that compression was applied
	hasCompressedRange := false
	for _, pattern := range patterns {
		// Look for compact range notation like [0-5] or 0[1-5]
		if contains(pattern.Regex, "[") && contains(pattern.Regex, "-") && contains(pattern.Regex, "]") {
			hasCompressedRange = true
		}
	}

	if !hasCompressedRange {
		t.Error("Expected to find compressed number range")
	}
}

func TestOrchestrator_LearnPatterns_WithoutCompression(t *testing.T) {
	domains := []string{
		"api-01.example.com",
		"api-02.example.com",
		"api-03.example.com",
	}

	config := DefaultOrchestratorConfig("example.com")
	config.EnableCompression = false
	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	// Should still generate patterns, just without compression
	if len(patterns) == 0 {
		t.Error("Expected to learn patterns even without compression")
	}
}

func TestOrchestrator_LearnPatterns_QualityFiltering(t *testing.T) {
	domains := []string{
		"a.example.com",
		"b.example.com",
	}

	config := DefaultOrchestratorConfig("example.com")
	// Set strict quality thresholds
	config.QualityConfig.MaxRatio = 5.0
	config.QualityConfig.MinCoverage = 2

	orchestrator := NewOrchestrator(config)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	// All patterns should meet quality thresholds
	for _, pattern := range patterns {
		if pattern.Coverage < 2 {
			t.Errorf("Pattern has coverage %d, below min 2", pattern.Coverage)
		}

		if pattern.Ratio > 5.0 {
			t.Errorf("Pattern has ratio %.2f, above max 5.0", pattern.Ratio)
		}
	}
}

func TestOrchestrator_DeduplicatePatterns(t *testing.T) {
	orchestrator := NewOrchestrator(nil)
	orchestrator.config.EnableDedupe = true

	// Create duplicate patterns
	patterns := []*Pattern{
		{Regex: "api-(dev|prod).example.com", Coverage: 2},
		{Regex: "api-(dev|prod).example.com", Coverage: 2}, // Duplicate
		{Regex: "web-(staging|prod).example.com", Coverage: 2},
	}

	unique := orchestrator.deduplicatePatterns(patterns)

	if len(unique) != 2 {
		t.Errorf("Expected 2 unique patterns, got %d", len(unique))
	}
}

func TestOrchestrator_CustomConfig(t *testing.T) {
	config := &OrchestratorConfig{
		MaxGroupSize: 1000, // Smaller groups
		RootDomain:   "test.com",
		ClusteringConfig: &ClusteringConfig{
			MinDelta: 1,
			MaxDelta: 5,
		},
		QualityConfig: &QualityConfig{
			MaxRatio:          10.0,
			AbsoluteThreshold: 100,
			MinCoverage:       3,
		},
		EnableCompression: true,
		EnableDedupe:      true,
	}

	orchestrator := NewOrchestrator(config)

	if orchestrator.config.MaxGroupSize != 1000 {
		t.Errorf("MaxGroupSize = %d; want 1000", orchestrator.config.MaxGroupSize)
	}

	if orchestrator.config.RootDomain != "test.com" {
		t.Errorf("RootDomain = %s; want test.com", orchestrator.config.RootDomain)
	}
}

func BenchmarkOrchestrator_LearnPatterns_100Domains(b *testing.B) {
	domains := make([]string, 100)
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			domains[i] = "api-dev-" + string(rune('0'+i%10)) + ".example.com"
		} else {
			domains[i] = "web-prod-" + string(rune('0'+i%10)) + ".example.com"
		}
	}

	config := DefaultOrchestratorConfig("example.com")
	orchestrator := NewOrchestrator(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = orchestrator.LearnPatterns(domains)
	}
}

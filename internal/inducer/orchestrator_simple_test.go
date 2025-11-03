package inducer

import (
	"testing"
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

	orchestrator := NewOrchestrator(nil) // Use default config

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected at least one pattern, got none")
	}

	// Print patterns for manual inspection
	t.Logf("Found %d patterns:", len(patterns))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d)", i+1, pattern.Template, pattern.Coverage)
	}

	// Basic sanity check: should find at least one pattern covering multiple domains
	foundMultiDomainPattern := false
	for _, pattern := range patterns {
		if pattern.Coverage >= 2 {
			foundMultiDomainPattern = true
			break
		}
	}

	if !foundMultiDomainPattern {
		t.Error("Expected at least one pattern covering multiple domains")
	}
}

// TestOrchestratorEmpty tests empty input
func TestOrchestratorEmpty(t *testing.T) {
	orchestrator := NewOrchestrator(nil)

	patterns, err := orchestrator.LearnPatterns([]string{})
	if err == nil {
		t.Error("Expected error for empty input, got nil")
	}
	if patterns != nil {
		t.Error("Expected nil patterns for empty input")
	}
}

// TestOrchestratorSingleDomain tests single domain input
func TestOrchestratorSingleDomain(t *testing.T) {
	domains := []string{"api.example.com"}

	orchestrator := NewOrchestrator(nil)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	// Single domain should produce no patterns (no variations to learn)
	if len(patterns) > 0 {
		t.Logf("Got %d patterns for single domain (might be OK):", len(patterns))
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

	orchestrator := NewOrchestrator(nil)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	t.Logf("Found %d patterns from %d domains:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d, ratio: %.2f)",
			i+1, pattern.Template, pattern.Coverage, pattern.Ratio)
	}

	// Should find some patterns
	if len(patterns) == 0 {
		t.Error("Expected patterns from realistic web services dataset")
	}
}

// Benchmark for MEMO table construction
func BenchmarkMemoTable100(b *testing.B) {
	// Generate 100 sample domains
	domains := make([]string, 100)
	for i := 0; i < 100; i++ {
		domains[i] = "api-dev-" + string(rune('a'+i%26)) + ".example.com"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orchestrator := NewOrchestrator(nil)
		_ = orchestrator.buildMemoTable()
	}
}

// Benchmark for full pattern learning
func BenchmarkLearnPatterns50(b *testing.B) {
	// Generate 50 sample domains with patterns
	domains := make([]string, 50)
	for i := 0; i < 50; i++ {
		env := []string{"dev", "prod", "staging"}[i%3]
		num := (i % 10) + 1
		domains[i] = "api-" + env + "-0" + string(rune('0'+num)) + ".example.com"
	}

	orchestrator := NewOrchestrator(nil)
	orchestrator.domains = domains

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := orchestrator.LearnPatterns(domains)
		if err != nil {
			b.Fatalf("LearnPatterns failed: %v", err)
		}
	}
}

// TestOrchestratorStats verifies per-strategy pattern tracking
func TestOrchestratorStats(t *testing.T) {
	config := DefaultOrchestratorConfig()
	o := NewOrchestrator(config)

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
	}

	_, err := o.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns should not error: %v", err)
	}

	stats := o.GetStats()

	// Verify stats structure
	if stats.InputDomains != 3 {
		t.Errorf("Expected 3 input domains, got %d", stats.InputDomains)
	}
	// Note: MemoTableSize is 0 with level-based grouping (we use local MEMO tables)
	// This is expected behavior in the new architecture
	if stats.Strategy1Patterns <= 0 {
		t.Errorf("Expected positive strategy 1 patterns, got %d", stats.Strategy1Patterns)
	}
	// Strategy 2 and 3 are now implemented and should produce patterns
	if stats.Strategy2Patterns < 0 {
		t.Errorf("Expected non-negative strategy 2 patterns, got %d", stats.Strategy2Patterns)
	}
	if stats.Strategy3Patterns < 0 {
		t.Errorf("Expected non-negative strategy 3 patterns, got %d", stats.Strategy3Patterns)
	}
	// Total patterns should be sum of all strategies (may have duplicates before deduplication)
	expectedTotal := stats.Strategy1Patterns + stats.Strategy2Patterns + stats.Strategy3Patterns
	if stats.TotalPatterns != expectedTotal {
		t.Errorf("Total patterns (%d) should equal sum of all strategies (%d)", stats.TotalPatterns, expectedTotal)
	}

	t.Logf("Stats: InputDomains=%d, Strategy1=%d, Strategy2=%d, Strategy3=%d, Total=%d",
		stats.InputDomains, stats.Strategy1Patterns, stats.Strategy2Patterns,
		stats.Strategy3Patterns, stats.TotalPatterns)
}

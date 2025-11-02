package inducer

import (
	"fmt"
	"testing"
)

func TestClusterer_EditClosures(t *testing.T) {
	clusterer := NewClusterer(nil)

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
		"api-prod-01.example.com",
		"web-staging.example.com",
	}

	// Precompute distances
	clusterer.memo.PrecomputeDistances(domains)

	// Test with delta=2 (should group api-dev-* together)
	closures := clusterer.editClosures(domains, 2)

	if len(closures) == 0 {
		t.Error("Expected at least one closure with delta=2")
	}

	// Find the api-dev closure
	found := false
	for _, closure := range closures {
		if closure.Size >= 3 {
			// Should contain the three api-dev domains
			found = true
			if closure.Delta != 2 {
				t.Errorf("Closure delta = %d; want 2", closure.Delta)
			}
		}
	}

	if !found {
		t.Error("Expected to find closure with 3+ api-dev domains")
	}
}

func TestClusterer_StrategyGlobal(t *testing.T) {
	clusterer := NewClusterer(nil)

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
	}

	// Precompute distances
	clusterer.memo.PrecomputeDistances(domains)

	closures := clusterer.strategyGlobal(domains)

	if len(closures) == 0 {
		t.Error("Expected at least one closure from global strategy")
	}

	// All closures should have size > 1
	for _, closure := range closures {
		if closure.Size < 2 {
			t.Errorf("Found closure with size %d; want >= 2", closure.Size)
		}
	}
}

func TestClusterer_StrategyTokenLevel(t *testing.T) {
	clusterer := NewClusterer(nil)

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"web-prod-01.example.com",
		"web-prod-02.example.com",
	}

	// Precompute distances
	clusterer.memo.PrecomputeDistances(domains)

	closures := clusterer.strategyTokenLevel(domains)

	// Should find closures for both "api" and "web" prefixes
	if len(closures) == 0 {
		t.Error("Expected closures from token-level strategy")
	}

	// Check that we have separate groups
	apiCount := 0
	webCount := 0

	for _, closure := range closures {
		for _, domain := range closure.Domains {
			if len(domain) >= 3 && domain[:3] == "api" {
				apiCount++
			}
			if len(domain) >= 3 && domain[:3] == "web" {
				webCount++
			}
		}
	}

	if apiCount == 0 {
		t.Error("Expected to find api domains in closures")
	}

	if webCount == 0 {
		t.Error("Expected to find web domains in closures")
	}
}

func TestClusterer_ClusterGroup(t *testing.T) {
	clusterer := NewClusterer(nil)

	group := &DomainGroup{
		Prefix: "api",
		Domains: []string{
			"api-dev-01.example.com",
			"api-dev-02.example.com",
			"api-dev-03.example.com",
			"api-prod-01.example.com",
			"api-prod-02.example.com",
		},
		Size: 5,
	}

	closures := clusterer.ClusterGroup(group)

	if len(closures) == 0 {
		t.Error("Expected to find closures")
	}

	// All closures should have size >= 2
	for _, closure := range closures {
		if closure.Size < 2 {
			t.Errorf("Closure has size %d; want >= 2", closure.Size)
		}

		// All domains in closure should be non-empty
		for _, domain := range closure.Domains {
			if domain == "" {
				t.Error("Found empty domain in closure")
			}
		}
	}
}

func TestClusterer_DeduplicateClosures(t *testing.T) {
	clusterer := NewClusterer(nil)

	// Create duplicate closures
	closures := []*Closure{
		{
			Domains: []string{"api-dev-01", "api-dev-02"},
			Delta:   2,
			Size:    2,
		},
		{
			Domains: []string{"api-dev-01", "api-dev-02"},
			Delta:   3,
			Size:    2,
		},
		{
			Domains: []string{"web-prod-01", "web-prod-02"},
			Delta:   2,
			Size:    2,
		},
	}

	unique := clusterer.deduplicateClosures(closures)

	// Should have only 2 unique closures (first two are duplicates)
	if len(unique) != 2 {
		t.Errorf("Expected 2 unique closures, got %d", len(unique))
	}
}

func TestMakeClosureKey(t *testing.T) {
	closure1 := &Closure{
		Domains: []string{"api-dev-01", "api-dev-02"},
	}

	closure2 := &Closure{
		Domains: []string{"api-dev-02", "api-dev-01"}, // Reversed order
	}

	key1 := makeClosureKey(closure1)
	key2 := makeClosureKey(closure2)

	// Keys should be identical (order-independent)
	if key1 != key2 {
		t.Errorf("Keys differ: %q vs %q", key1, key2)
	}
}

func TestExtractFirstToken(t *testing.T) {
	tests := []struct {
		domain   string
		expected string
	}{
		{"api-dev-01.example.com", "api"},
		{"web.staging.example.com", "web"},
		{"cdn", "cdn"},
		{"api", "api"},
		{"test-foo-bar", "test"},
	}

	for _, tt := range tests {
		result := extractFirstToken(tt.domain)
		if result != tt.expected {
			t.Errorf("extractFirstToken(%q) = %q; want %q", tt.domain, result, tt.expected)
		}
	}
}

func TestClusterer_SingleDomain(t *testing.T) {
	clusterer := NewClusterer(nil)

	group := &DomainGroup{
		Prefix:  "api",
		Domains: []string{"api.example.com"},
		Size:    1,
	}

	closures := clusterer.ClusterGroup(group)

	// Single domain should produce no closures (need at least 2 for a pattern)
	if len(closures) != 0 {
		t.Errorf("Expected 0 closures for single domain, got %d", len(closures))
	}
}

func TestClusterer_CustomConfig(t *testing.T) {
	config := &ClusteringConfig{
		MinDelta: 1,
		MaxDelta: 5,
	}

	clusterer := NewClusterer(config)

	if clusterer.config.MinDelta != 1 {
		t.Errorf("MinDelta = %d; want 1", clusterer.config.MinDelta)
	}

	if clusterer.config.MaxDelta != 5 {
		t.Errorf("MaxDelta = %d; want 5", clusterer.config.MaxDelta)
	}
}

func BenchmarkClusterer_ClusterGroup(b *testing.B) {
	clusterer := NewClusterer(nil)

	domains := make([]string, 100)
	for i := 0; i < 100; i++ {
		domains[i] = fmt.Sprintf("api-dev-%03d.example.com", i)
	}

	group := &DomainGroup{
		Prefix:  "api",
		Domains: domains,
		Size:    100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clusterer.ClusterGroup(group)
	}
}

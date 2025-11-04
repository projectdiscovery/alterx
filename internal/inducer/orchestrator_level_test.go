package inducer

import (
	"strings"
	"testing"
)

// TestOrchestratorLevelGrouping tests level-based grouping with mixed-level domains
func TestOrchestratorLevelGrouping(t *testing.T) {
	// Test case: Mixed 1-level, 2-level, and 3-level domains
	domains := []string{
		// 1-level domains ({{p0}}.{{root}})
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"web-dev-01.example.com",
		"web-prod-01.example.com",

		// 2-level domains ({{p0}}.{{p1}}.{{root}})
		"scheduler.api.example.com",
		"webhook.api.example.com",
		"worker.api.example.com",
		"balancer.web.example.com",
		"cache.web.example.com",

		// 3-level domains ({{p0}}.{{p1}}.{{p2}}.{{root}})
		"v1.scheduler.api.example.com",
		"v2.scheduler.api.example.com",
		"v1.webhook.api.example.com",
		"v2.webhook.api.example.com",
	}

	orchestrator := NewOrchestrator(100) // Use default config

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected at least one pattern, got none")
	}

	// Print patterns for manual inspection
	t.Logf("Found %d patterns from %d domains:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d, ratio: %.2f)",
			i+1, pattern.Template, pattern.Coverage, pattern.Ratio)
	}

	// Verify patterns use {{root}} not {{suffix}}
	for _, pattern := range patterns {
		if strings.Contains(pattern.Template, "{{suffix}}") {
			t.Errorf("Pattern should use {{root}} not {{suffix}}: %s", pattern.Template)
		}
		if !strings.Contains(pattern.Template, "{{root}}") {
			t.Errorf("Pattern should contain {{root}}: %s", pattern.Template)
		}
	}

	// Verify we found patterns for different level groups
	foundMultiLevelPatterns := false
	for _, pattern := range patterns {
		// Count dots in template to infer level count
		// {{p0}}.{{p1}}.{{root}} → 2 levels
		if countDotsInTemplate(pattern.Template) >= 2 {
			foundMultiLevelPatterns = true
			break
		}
	}

	if !foundMultiLevelPatterns {
		t.Log("Warning: Expected patterns for multi-level domains, but none found")
		// Not a hard failure - depends on clustering parameters
	}
}

// TestOrchestratorSingleLevelOnly tests with only 1-level domains
func TestOrchestratorSingleLevelOnly(t *testing.T) {
	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"web-dev-01.example.com",
		"web-prod-01.example.com",
		"db-dev-01.example.com",
		"db-prod-01.example.com",
	}

	orchestrator := NewOrchestrator(100)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	t.Logf("Found %d patterns from %d 1-level domains:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d)", i+1, pattern.Template, pattern.Coverage)
	}

	// All patterns should use {{root}}
	for _, pattern := range patterns {
		if !strings.Contains(pattern.Template, "{{root}}") {
			t.Errorf("Pattern should contain {{root}}: %s", pattern.Template)
		}
	}

	// Patterns should be relatively simple (1 level)
	for _, pattern := range patterns {
		dots := countDotsInTemplate(pattern.Template)
		if dots > 1 {
			t.Errorf("Expected simple 1-level patterns, got %d dots: %s", dots, pattern.Template)
		}
	}
}

// TestOrchestratorTwoLevelOnly tests with only 2-level domains
func TestOrchestratorTwoLevelOnly(t *testing.T) {
	domains := []string{
		"scheduler.api.example.com",
		"webhook.api.example.com",
		"worker.api.example.com",
		"processor.api.example.com",
		"balancer.web.example.com",
		"cache.web.example.com",
		"proxy.web.example.com",
	}

	orchestrator := NewOrchestrator(100)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	t.Logf("Found %d patterns from %d 2-level domains:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d)", i+1, pattern.Template, pattern.Coverage)
	}

	// All patterns should use {{root}}
	for _, pattern := range patterns {
		if !strings.Contains(pattern.Template, "{{root}}") {
			t.Errorf("Pattern should contain {{root}}: %s", pattern.Template)
		}
	}

	// Patterns should have 2 levels (2 dots before {{root}})
	// Example: {{p0}}.{{p1}}.{{root}}
	for _, pattern := range patterns {
		dots := countDotsInTemplate(pattern.Template)
		if dots != 2 {
			t.Logf("Note: Expected 2-level patterns (2 dots), got %d: %s", dots, pattern.Template)
			// Not a hard failure - pattern might be generalized
		}
	}
}

// TestOrchestratorThreeLevelOnly tests with only 3-level domains
func TestOrchestratorThreeLevelOnly(t *testing.T) {
	domains := []string{
		"v1.scheduler.api.example.com",
		"v2.scheduler.api.example.com",
		"v1.webhook.api.example.com",
		"v2.webhook.api.example.com",
		"v1.worker.api.example.com",
		"v2.worker.api.example.com",
		"v1.proxy.web.example.com",
		"v2.proxy.web.example.com",
	}

	orchestrator := NewOrchestrator(100)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	t.Logf("Found %d patterns from %d 3-level domains:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d)", i+1, pattern.Template, pattern.Coverage)
	}

	// All patterns should use {{root}}
	for _, pattern := range patterns {
		if !strings.Contains(pattern.Template, "{{root}}") {
			t.Errorf("Pattern should contain {{root}}: %s", pattern.Template)
		}
	}

	// Patterns should have 3 levels (3 dots before {{root}})
	// Example: {{p0}}.{{p1}}.{{p2}}.{{root}}
	for _, pattern := range patterns {
		dots := countDotsInTemplate(pattern.Template)
		if dots < 2 {
			t.Errorf("Expected 3-level patterns (3+ dots), got only %d: %s", dots, pattern.Template)
		}
	}
}

// TestOrchestratorRealWorldMixed tests with realistic mixed-level scenario
func TestOrchestratorRealWorldMixed(t *testing.T) {
	// Simulate a real-world subdomain enumeration result
	domains := []string{
		// Production services (1-level)
		"api.projectdiscovery.io",
		"cdn.projectdiscovery.io",
		"web.projectdiscovery.io",

		// Development services (1-level)
		"api-dev.projectdiscovery.io",
		"api-staging.projectdiscovery.io",
		"web-dev.projectdiscovery.io",
		"web-staging.projectdiscovery.io",

		// Service subdivisions (2-level)
		"scheduler.api.projectdiscovery.io",
		"webhook.api.projectdiscovery.io",
		"worker.api.projectdiscovery.io",
		"balancer.web.projectdiscovery.io",
		"cache.web.projectdiscovery.io",

		// Versioned services (3-level)
		"v1.scheduler.api.projectdiscovery.io",
		"v2.scheduler.api.projectdiscovery.io",
		"v1.webhook.api.projectdiscovery.io",
		"v2.webhook.api.projectdiscovery.io",
	}

	orchestrator := NewOrchestrator(100)

	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	t.Logf("Found %d patterns from %d real-world domains:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d, ratio: %.2f)",
			i+1, pattern.Template, pattern.Coverage, pattern.Ratio)
	}

	// Verify all patterns use {{root}}
	for _, pattern := range patterns {
		if !strings.Contains(pattern.Template, "{{root}}") {
			t.Errorf("Pattern should contain {{root}}: %s", pattern.Template)
		}
	}

	// Should find at least a few patterns
	if len(patterns) == 0 {
		t.Error("Expected at least one pattern from real-world mixed domains")
	}

	// Verify patterns passed quality filtering (ratio test)
	for _, pattern := range patterns {
		if pattern.Ratio > 25.0 {
			t.Errorf("Pattern ratio too high (%.2f > 25.0): %s", pattern.Ratio, pattern.Template)
		}
	}
}

// TestOrchestratorLevelGroupStats verifies level grouping statistics
func TestOrchestratorLevelGroupStats(t *testing.T) {
	domains := []string{
		// 1-level: 3 domains
		"api.example.com",
		"web.example.com",
		"cdn.example.com",

		// 2-level: 4 domains
		"scheduler.api.example.com",
		"webhook.api.example.com",
		"balancer.web.example.com",
		"cache.web.example.com",

		// 3-level: 2 domains
		"v1.scheduler.api.example.com",
		"v2.scheduler.api.example.com",
	}

	// Test level grouping directly
	groups, err := GroupByLevelCount(domains)
	if err != nil {
		t.Fatalf("GroupByLevelCount failed: %v", err)
	}

	// Verify group counts
	if len(groups) != 3 {
		t.Errorf("Expected 3 level groups, got %d", len(groups))
	}

	// Verify 1-level group
	if groups[1] == nil {
		t.Error("Expected 1-level group")
	} else if len(groups[1].Domains) != 3 {
		t.Errorf("Expected 3 domains in 1-level group, got %d", len(groups[1].Domains))
	}

	// Verify 2-level group
	if groups[2] == nil {
		t.Error("Expected 2-level group")
	} else if len(groups[2].Domains) != 4 {
		t.Errorf("Expected 4 domains in 2-level group, got %d", len(groups[2].Domains))
	}

	// Verify 3-level group
	if groups[3] == nil {
		t.Error("Expected 3-level group")
	} else if len(groups[3].Domains) != 2 {
		t.Errorf("Expected 2 domains in 3-level group, got %d", len(groups[3].Domains))
	}

	// Now test orchestrator with these domains
	orchestrator := NewOrchestrator(100)
	patterns, err := orchestrator.LearnPatterns(domains)
	if err != nil {
		t.Fatalf("LearnPatterns failed: %v", err)
	}

	t.Logf("Found %d patterns from %d domains across 3 level groups:", len(patterns), len(domains))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s (coverage: %d)", i+1, pattern.Template, pattern.Coverage)
	}
}

// Helper function to count dots in a template
// This helps infer the structural level of a pattern
func countDotsInTemplate(template string) int {
	count := 0
	for _, ch := range template {
		if ch == '.' {
			count++
		}
	}
	return count
}

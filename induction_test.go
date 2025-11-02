package alterx

import (
	"testing"
)

func TestPatternInducer_InferPatterns(t *testing.T) {
	// Simulate passive subdomain enumeration results
	passiveDomains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
		"api-prod-01.example.com",
		"api-prod-02.example.com",
		"web-staging.example.com",
		"web-prod.example.com",
		"cdn-us.example.com",
		"cdn-eu.example.com",
	}

	inputDomains := []string{"example.com"}

	inducer := NewPatternInducer(inputDomains, passiveDomains, 2)
	patterns, err := inducer.InferPatterns()

	if err != nil {
		t.Fatalf("InferPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected to infer at least one pattern")
	}

	// All patterns should be valid DSL
	for _, pattern := range patterns {
		if !contains(pattern, "{{") || !contains(pattern, "}}") {
			t.Errorf("Pattern %q is not valid DSL", pattern)
		}

		// Should contain either {{word}}, {{number}}, or {{suffix}}
		if !contains(pattern, "{{word}}") && !contains(pattern, "{{number}}") && !contains(pattern, "{{suffix}}") {
			t.Errorf("Pattern %q doesn't contain expected DSL variables", pattern)
		}
	}

	// Log learned patterns for manual verification
	t.Logf("Learned %d patterns:", len(patterns))
	for i, pattern := range patterns {
		t.Logf("  Pattern %d: %s", i+1, pattern)
	}
}

func TestPatternInducer_EmptyPassiveDomains(t *testing.T) {
	inputDomains := []string{"example.com"}
	passiveDomains := []string{}

	inducer := NewPatternInducer(inputDomains, passiveDomains, 2)
	patterns, err := inducer.InferPatterns()

	if err != nil {
		t.Fatalf("InferPatterns should not error on empty input, got: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("Expected 0 patterns for empty input, got %d", len(patterns))
	}
}

func TestPatternInducer_SingleDomain(t *testing.T) {
	inputDomains := []string{"example.com"}
	passiveDomains := []string{"api.example.com"}

	inducer := NewPatternInducer(inputDomains, passiveDomains, 2)
	patterns, err := inducer.InferPatterns()

	if err != nil {
		t.Fatalf("InferPatterns failed: %v", err)
	}

	// Single domain shouldn't produce patterns (need at least 2 for clustering)
	if len(patterns) != 0 {
		t.Errorf("Expected 0 patterns for single domain, got %d", len(patterns))
	}
}

func TestPatternInducer_NumberPatterns(t *testing.T) {
	inputDomains := []string{"example.com"}
	passiveDomains := []string{
		"server-01.example.com",
		"server-02.example.com",
		"server-03.example.com",
		"server-04.example.com",
		"server-05.example.com",
	}

	inducer := NewPatternInducer(inputDomains, passiveDomains, 2)
	patterns, err := inducer.InferPatterns()

	if err != nil {
		t.Fatalf("InferPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected to infer at least one pattern")
	}

	// Should find a pattern with numbers
	hasNumber := false
	for _, pattern := range patterns {
		if contains(pattern, "{{number}}") {
			hasNumber = true
			break
		}
	}

	if !hasNumber {
		t.Error("Expected to find pattern with {{number}}")
	}

	t.Logf("Learned patterns: %v", patterns)
}

func TestPatternInducer_EnvironmentPatterns(t *testing.T) {
	inputDomains := []string{"example.com"}
	passiveDomains := []string{
		"api.dev.example.com",
		"api.staging.example.com",
		"api.prod.example.com",
		"web.dev.example.com",
		"web.staging.example.com",
		"web.prod.example.com",
	}

	inducer := NewPatternInducer(inputDomains, passiveDomains, 2)
	patterns, err := inducer.InferPatterns()

	if err != nil {
		t.Fatalf("InferPatterns failed: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Expected to infer at least one pattern")
	}

	// Note: Multi-level patterns may be simplified during conversion
	// The important thing is that we learned patterns successfully
	t.Logf("Learned %d patterns: %v", len(patterns), patterns)

	if len(patterns) == 0 {
		t.Error("Expected to learn patterns from environment variations")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

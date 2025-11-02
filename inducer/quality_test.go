package inducer

import (
	"testing"
)

func TestQualityFilter_IsGoodPattern(t *testing.T) {
	qf := NewQualityFilter(nil)

	tests := []struct {
		name     string
		pattern  *Pattern
		expected bool
	}{
		{
			name: "Small pattern - auto-accept",
			pattern: &Pattern{
				Regex:    "api-(dev|prod).example.com",
				Coverage: 2,
			},
			expected: true, // Only 2 alternatives, < threshold
		},
		{
			name: "Good ratio",
			pattern: &Pattern{
				Regex:    "api-[0-9].example.com",
				Coverage: 8,
			},
			expected: true, // 10 generations / 8 coverage = 1.25 ratio
		},
		{
			name: "Bad ratio - too broad",
			pattern: &Pattern{
				Regex:    "api-[a-z][0-9]{2}.example.com",
				Coverage: 3,
			},
			expected: false, // 26*100 = 2600 generations / 3 coverage = 866 ratio
		},
		{
			name: "Below minimum coverage",
			pattern: &Pattern{
				Regex:    "api.example.com",
				Coverage: 1,
			},
			expected: false, // Coverage < 2
		},
		{
			name: "Multiple alternations - acceptable",
			pattern: &Pattern{
				Regex:    "(api|web)-(dev|prod)-[1-3].example.com",
				Coverage: 10,
			},
			expected: true, // 2*2*3 = 12 generations / 10 coverage = 1.2 ratio
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qf.IsGoodPattern(tt.pattern)
			if result != tt.expected {
				t.Errorf("IsGoodPattern() = %v; want %v (ratio: %.2f)",
					result, tt.expected, tt.pattern.Ratio)
			}
		})
	}
}

func TestQualityFilter_EstimateGenerations(t *testing.T) {
	qf := NewQualityFilter(nil)

	tests := []struct {
		regex    string
		expected int
	}{
		{"api.example.com", 1},                      // No alternations
		{"(api|web).example.com", 2},                // Simple alternation
		{"(api|web|cdn).example.com", 3},            // Three-way alternation
		{"api-[0-9].example.com", 10},               // Single digit range
		{"api-[0-9][0-9].example.com", 100},         // Two digit positions
		{"(api|web)-[0-9].example.com", 20},         // Alternation + range (2*10)
		{"(api|web|cdn)-(dev|prod).example.com", 6}, // Multiple alternations (3*2)
		{"api(.staging)?.example.com", 2},           // Optional group (2x)
	}

	for _, tt := range tests {
		result := qf.estimateGenerations(tt.regex)
		if result != tt.expected {
			t.Errorf("estimateGenerations(%q) = %d; want %d", tt.regex, result, tt.expected)
		}
	}
}

func TestQualityFilter_EstimateCharClassSize(t *testing.T) {
	qf := NewQualityFilter(nil)

	tests := []struct {
		class    string
		expected int
	}{
		{"0-9", 10},
		{"a-z", 26},
		{"A-Z", 26},
		{"0-5", 6},
		{"1-9", 9},
		{"a-f", 6},
		{"abc", 3},
		{"0123", 4},
	}

	for _, tt := range tests {
		result := qf.estimateCharClassSize(tt.class)
		if result != tt.expected {
			t.Errorf("estimateCharClassSize(%q) = %d; want %d", tt.class, result, tt.expected)
		}
	}
}

func TestQualityFilter_FilterPatterns(t *testing.T) {
	qf := NewQualityFilter(nil)

	patterns := []*Pattern{
		{
			Regex:    "api-(dev|prod).example.com",
			Coverage: 2,
		},
		{
			Regex:    "api-[0-9].example.com",
			Coverage: 8,
		},
		{
			Regex:    "api-[a-z][a-z][a-z].example.com", // 26^3 = 17576 generations
			Coverage: 5,                                  // Ratio = 3515
		},
		{
			Regex:    "single.example.com",
			Coverage: 1, // Below min coverage
		},
	}

	filtered := qf.FilterPatterns(patterns)

	// Should filter out the overly broad pattern and single-coverage pattern
	if len(filtered) != 2 {
		t.Errorf("Expected 2 patterns after filtering, got %d", len(filtered))
	}

	// Check that good patterns were kept
	found := 0
	for _, pattern := range filtered {
		if pattern.Regex == "api-(dev|prod).example.com" || pattern.Regex == "api-[0-9].example.com" {
			found++
		}
	}

	if found != 2 {
		t.Errorf("Expected both good patterns to be kept, found %d", found)
	}
}

func TestQualityFilter_CustomConfig(t *testing.T) {
	config := &QualityConfig{
		MaxRatio:          10.0, // Stricter ratio
		AbsoluteThreshold: 100,
		MinCoverage:       3, // Require at least 3 domains
	}

	qf := NewQualityFilter(config)

	pattern := &Pattern{
		Regex:    "api-[0-9].example.com", // 10 generations
		Coverage: 2,                       // Below min coverage of 3
	}

	if qf.IsGoodPattern(pattern) {
		t.Error("Pattern should be rejected due to low coverage")
	}
}

func TestQualityFilter_ComplexPattern(t *testing.T) {
	qf := NewQualityFilter(nil)

	// Pattern with multiple features
	pattern := &Pattern{
		Regex:    "(api|web)-[0-9](.staging|.prod)?.example.com",
		Coverage: 10,
	}

	// Estimate: 2 (api|web) * 10 [0-9] * 2 (optional) = 40 generations
	// Ratio: 40/10 = 4.0 (good, < 25.0)

	if !qf.IsGoodPattern(pattern) {
		t.Errorf("Pattern should be accepted (ratio: %.2f)", pattern.Ratio)
	}
}

func TestPattern_SetRatio(t *testing.T) {
	pattern := &Pattern{
		Regex:    "test.example.com",
		Coverage: 1,
	}

	pattern.SetRatio(5.5)

	if pattern.Ratio != 5.5 {
		t.Errorf("SetRatio failed: got %.2f, want 5.5", pattern.Ratio)
	}
}

func TestPattern_SetConfidence(t *testing.T) {
	pattern := &Pattern{
		Regex:    "test.example.com",
		Coverage: 1,
	}

	pattern.SetConfidence(0.85)

	if pattern.Confidence != 0.85 {
		t.Errorf("SetConfidence failed: got %.2f, want 0.85", pattern.Confidence)
	}
}

func BenchmarkQualityFilter_IsGoodPattern(b *testing.B) {
	qf := NewQualityFilter(nil)

	pattern := &Pattern{
		Regex:    "(api|web|cdn)-(dev|prod|staging)-[0-9]{2}.example.com",
		Coverage: 50,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qf.IsGoodPattern(pattern)
	}
}

func BenchmarkQualityFilter_EstimateGenerations(b *testing.B) {
	qf := NewQualityFilter(nil)

	regex := "(api|web|cdn)-(dev|prod|staging)-[0-9]{2}(.internal)?.example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qf.estimateGenerations(regex)
	}
}

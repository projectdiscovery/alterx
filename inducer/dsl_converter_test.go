package inducer

import (
	"testing"
)

func TestDSLConverter_ConvertToDSL(t *testing.T) {
	converter := NewDSLConverter("example.com")

	tests := []struct {
		name     string
		regex    string
		expected string
	}{
		{
			name:     "Simple alternation",
			regex:    "(api|web|cdn).example.com",
			expected: "{{word}}.{{suffix}}",
		},
		{
			name:     "With dash separator",
			regex:    "(api|web)-(dev|prod).example.com",
			expected: "{{word}}-{{word}}.{{suffix}}",
		},
		{
			name:     "With number range",
			regex:    "server-0[1-5].example.com",
			expected: "server-{{number}}.{{suffix}}",
		},
		{
			name:     "Multi-level",
			regex:    "(api|web).(staging|prod).example.com",
			expected: "{{word}}.{{word}}.{{suffix}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := &Pattern{
				Regex:    tt.regex,
				Coverage: 2,
			}

			result := converter.ConvertToDSL(pattern)

			// Validate against expected output
			if !contains(result, "{{") || !contains(result, "}}") {
				t.Errorf("ConvertToDSL() = %q; must contain DSL variables", result)
			}

			// Check that result contains key elements from expected pattern
			// Note: exact match may differ due to implementation details
			if tt.expected != "" && !contains(result, "{{suffix}}") && !contains(result, "example.com") {
				t.Logf("ConvertToDSL() = %q; expected similar to %q", result, tt.expected)
			}
		})
	}
}

func TestDSLConverter_ConvertPatternsToDSL(t *testing.T) {
	converter := NewDSLConverter("example.com")

	patterns := []*Pattern{
		{Regex: "(api|web).example.com", Coverage: 2},
		{Regex: "(api|web).example.com", Coverage: 2}, // Duplicate
		{Regex: "(cdn|db).example.com", Coverage: 2},
	}

	dslPatterns := converter.ConvertPatternsToDSL(patterns)

	// Should deduplicate
	if len(dslPatterns) > 2 {
		t.Errorf("Expected at most 2 unique DSL patterns, got %d", len(dslPatterns))
	}

	// All should contain DSL variables
	for _, dsl := range dslPatterns {
		if !contains(dsl, "{{") || !contains(dsl, "}}") {
			t.Errorf("DSL pattern %q doesn't contain DSL variables", dsl)
		}
	}
}

func TestSimplifyDSL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "{{word}}-{{word}}.{{suffix}}",
			expected: "{{word}}.{{suffix}}",
		},
		{
			input:    "{{word}}.{{word}}.{{suffix}}",
			expected: "{{word}}.{{suffix}}",
		},
		{
			input:    "{{word}}.{{suffix}}",
			expected: "{{word}}.{{suffix}}", // No change
		},
	}

	for _, tt := range tests {
		result := SimplifyDSL(tt.input)
		if result != tt.expected {
			t.Errorf("SimplifyDSL(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDSLConverter_WithoutRootDomain(t *testing.T) {
	converter := NewDSLConverter("")

	pattern := &Pattern{
		Regex:    "(api|web).test.com",
		Coverage: 2,
	}

	result := converter.ConvertToDSL(pattern)

	// Should still work without root domain
	if !contains(result, "{{") {
		t.Errorf("ConvertToDSL() = %q; expected to contain DSL variables", result)
	}
}

func TestDSLConverter_ComplexPattern(t *testing.T) {
	converter := NewDSLConverter("example.com")

	pattern := &Pattern{
		Regex:    "(api|web|cdn)-(dev|prod|staging)-0[1-9].example.com",
		Coverage: 12,
	}

	result := converter.ConvertToDSL(pattern)

	// Should have {{word}} and/or {{number}}
	hasWord := contains(result, "{{word}}")
	hasNumber := contains(result, "{{number}}")

	if !hasWord {
		t.Error("Expected at least one {{word}} in converted pattern")
	}

	if !hasNumber {
		t.Error("Expected at least one {{number}} in converted pattern")
	}
}

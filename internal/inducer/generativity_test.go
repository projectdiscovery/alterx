package inducer

import "testing"

func TestEstimateRegexGenerativity(t *testing.T) {
	tests := []struct {
		name     string
		regex    string
		expected int
	}{
		// Simple cases
		{
			name:     "Empty pattern",
			regex:    "",
			expected: 1,
		},
		{
			name:     "Single literal",
			regex:    "api",
			expected: 1,
		},
		{
			name:     "Escaped dot",
			regex:    "api\\.staging",
			expected: 1,
		},

		// Simple alternations
		{
			name:     "Simple alternation - 3 options",
			regex:    "(api|web|cdn)",
			expected: 3,
		},
		{
			name:     "Simple alternation - 2 options",
			regex:    "(dev|prod)",
			expected: 2,
		},
		{
			name:     "Alternation with escaped characters",
			regex:    "(api\\-dev|web\\-prod)",
			expected: 2,
		},

		// Multiple alternations
		{
			name:     "Two alternation groups",
			regex:    "(api|web)-(dev|prod)",
			expected: 4, // 2 × 2
		},
		{
			name:     "Three alternation groups",
			regex:    "(api|web|cdn)-(dev|prod|staging)",
			expected: 9, // 3 × 3
		},
		{
			name:     "Multiple with dots",
			regex:    "(api|web)\\.(dev|prod)",
			expected: 4, // 2 × 2
		},

		// Character classes
		{
			name:     "Single digit range",
			regex:    "[0-9]",
			expected: 10,
		},
		{
			name:     "Small digit range",
			regex:    "[0-3]",
			expected: 4,
		},
		{
			name:     "Lowercase letters",
			regex:    "[a-z]",
			expected: 26,
		},
		{
			name:     "Multiple character ranges",
			regex:    "[0-9][0-9]",
			expected: 100, // 10 × 10
		},
		{
			name:     "Mixed range",
			regex:    "[0-1][0-3]",
			expected: 8, // 2 × 4
		},

		// Combined alternations and character classes
		{
			name:     "Alternation with number",
			regex:    "(api|web)[0-9]",
			expected: 20, // 2 × 10
		},
		{
			name:     "Complex pattern",
			regex:    "(db|cache)[0-1][0-9]",
			expected: 40, // 2 × 2 × 10
		},

		// Optional groups
		{
			name:     "Simple optional group",
			regex:    "(api|web)?",
			expected: 3, // 2 + 1 (absent)
		},
		{
			name:     "Optional group with prefix",
			regex:    "api(\\.internal)?",
			expected: 2, // 1 + 1 (present or absent)
		},
		{
			name:     "Optional group with alternation",
			regex:    "(api|web)(\\.staging)?",
			expected: 4, // 2 × 2
		},
		{
			name:     "Multiple optional groups",
			regex:    "(api|web)?(\\.staging)?",
			expected: 6, // 3 × 2 (first group: api, web, or absent = 3; second: present or absent = 2)
		},

		// Real-world patterns from pattern generator
		{
			name:     "Environment variation",
			regex:    "api-(dev|prod|staging)",
			expected: 3,
		},
		{
			name:     "Number variation",
			regex:    "api-dev-(01|02|03)",
			expected: 3,
		},
		{
			name:     "Multi-level with alternation",
			regex:    "(api|web)\\.(staging|prod)",
			expected: 4,
		},
		{
			name:     "Compressed number range",
			regex:    "server[0-1][0-9]",
			expected: 20,
		},
		{
			name:     "Complex multi-level",
			regex:    "(api|web|cdn)-(dev|prod)\\.(internal|external)",
			expected: 12, // 3 × 2 × 2
		},

		// Edge cases
		{
			name:     "Empty group",
			regex:    "()",
			expected: 1,
		},
		{
			name:     "Nested groups",
			regex:    "((api|web)-(dev|prod))",
			expected: 4,
		},
		{
			name:     "Multiple nested groups",
			regex:    "((api|web)|cdn)",
			expected: 3, // sum: 2 + 1
		},
		{
			name:     "Character class with escaped chars",
			regex:    "[\\-\\.]",
			expected: 2,
		},

		// Complex real-world examples
		{
			name:     "Full subdomain pattern 1",
			regex:    "(api|web)-(dev|prod|staging)-[0-9][0-9]",
			expected: 600, // 2 × 3 × 10 × 10
		},
		{
			name:     "Full subdomain pattern 2 (with optional level)",
			regex:    "(db|cache|queue)[0-1][0-9](\\.(internal|external))?",
			expected: 180, // 3 × 2 × 10 × 3 (last 3 = absent, .internal, .external)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateRegexGenerativity(tt.regex)
			if result != tt.expected {
				t.Errorf("estimateRegexGenerativity(%q) = %d; want %d", tt.regex, result, tt.expected)
			}
		})
	}
}

func TestParseCharacterClass(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "Empty",
			content:  "",
			expected: 1,
		},
		{
			name:     "Single digit range",
			content:  "0-9",
			expected: 10,
		},
		{
			name:     "Small range",
			content:  "0-3",
			expected: 4,
		},
		{
			name:     "Lowercase range",
			content:  "a-z",
			expected: 26,
		},
		{
			name:     "Uppercase range",
			content:  "A-Z",
			expected: 26,
		},
		{
			name:     "Individual characters",
			content:  "abc",
			expected: 3,
		},
		{
			name:     "Mixed range and individual",
			content:  "0-9a",
			expected: 11,
		},
		{
			name:     "Multiple ranges",
			content:  "0-9a-z",
			expected: 36,
		},
		{
			name:     "Escaped characters",
			content:  "\\-\\.",
			expected: 2,
		},
		{
			name:     "Range with escaped",
			content:  "0-9\\-",
			expected: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCharacterClass(tt.content)
			if result != tt.expected {
				t.Errorf("parseCharacterClass(%q) = %d; want %d", tt.content, result, tt.expected)
			}
		})
	}
}

func TestSplitAlternatives(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Empty",
			content:  "",
			expected: []string{},
		},
		{
			name:     "No alternations",
			content:  "api",
			expected: []string{"api"},
		},
		{
			name:     "Simple alternations",
			content:  "api|web|cdn",
			expected: []string{"api", "web", "cdn"},
		},
		{
			name:     "Two alternations",
			content:  "dev|prod",
			expected: []string{"dev", "prod"},
		},
		{
			name:     "With escaped characters",
			content:  "api\\-dev|web\\-prod",
			expected: []string{"api\\-dev", "web\\-prod"},
		},
		{
			name:     "Nested groups",
			content:  "(api|web)-(dev|prod)|cdn",
			expected: []string{"(api|web)-(dev|prod)", "cdn"},
		},
		{
			name:     "With character classes",
			content:  "[0-9]|[a-z]",
			expected: []string{"[0-9]", "[a-z]"},
		},
		{
			name:     "Character class with pipe",
			content:  "[a|b]",
			expected: []string{"[a|b]"}, // Pipe inside char class is literal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAlternatives(tt.content)
			if len(result) != len(tt.expected) {
				t.Fatalf("splitAlternatives(%q) returned %d alternatives; want %d\nGot: %v\nWant: %v",
					tt.content, len(result), len(tt.expected), result, tt.expected)
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitAlternatives(%q)[%d] = %q; want %q",
						tt.content, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFindMatchingParen(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		start    int
		expected int
	}{
		{
			name:     "Simple group",
			s:        "(api)",
			start:    0,
			expected: 4,
		},
		{
			name:     "Nested groups",
			s:        "((api))",
			start:    0,
			expected: 6,
		},
		{
			name:     "Inner group",
			s:        "((api))",
			start:    1,
			expected: 5,
		},
		{
			name:     "Multiple groups",
			s:        "(api)(web)",
			start:    0,
			expected: 4,
		},
		{
			name:     "Second group",
			s:        "(api)(web)",
			start:    5,
			expected: 9,
		},
		{
			name:     "With character class",
			s:        "(api[0-9])",
			start:    0,
			expected: 9,
		},
		{
			name:     "Character class with parens",
			s:        "([()])",
			start:    0,
			expected: 5,
		},
		{
			name:     "Escaped paren",
			s:        "(api\\)test)",
			start:    0,
			expected: 10,
		},
		{
			name:     "Not starting at paren",
			s:        "api(test)",
			start:    2,
			expected: -1,
		},
		{
			name:     "Unmatched",
			s:        "(api",
			start:    0,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMatchingParen(tt.s, tt.start)
			if result != tt.expected {
				t.Errorf("findMatchingParen(%q, %d) = %d; want %d", tt.s, tt.start, result, tt.expected)
			}
		})
	}
}

func TestFindClosingBracket(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		start    int
		expected int
	}{
		{
			name:     "Simple class",
			s:        "[0-9]",
			start:    0,
			expected: 4,
		},
		{
			name:     "Multiple chars",
			s:        "[abc]",
			start:    0,
			expected: 4,
		},
		{
			name:     "With escape",
			s:        "[\\]]",
			start:    0,
			expected: 3,
		},
		{
			name:     "Multiple classes",
			s:        "[0-9][a-z]",
			start:    0,
			expected: 4,
		},
		{
			name:     "Second class",
			s:        "[0-9][a-z]",
			start:    5,
			expected: 9,
		},
		{
			name:     "Not starting at bracket",
			s:        "api[test]",
			start:    2,
			expected: -1,
		},
		{
			name:     "Unmatched",
			s:        "[abc",
			start:    0,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findClosingBracket(tt.s, tt.start)
			if result != tt.expected {
				t.Errorf("findClosingBracket(%q, %d) = %d; want %d", tt.s, tt.start, result, tt.expected)
			}
		})
	}
}

func TestEstimateGenerativityWithPatternStruct(t *testing.T) {
	// Note: estimateGenerativity method removed - now part of DSL generation
	tests := []struct {
		name     string
		pattern  *Pattern
		expected int
	}{
		{
			name: "Empty pattern",
			pattern: &Pattern{
				Regex:    "",
				Coverage: 5,
			},
			expected: 1,
		},
		{
			name: "Simple alternation pattern",
			pattern: &Pattern{
				Regex:    "(api|web|cdn)",
				Coverage: 3,
			},
			expected: 3,
		},
		{
			name: "Complex pattern",
			pattern: &Pattern{
				Regex:    "(api|web)-(dev|prod|staging)",
				Coverage: 6,
			},
			expected: 6, // 2 × 3
		},
		{
			name: "Pattern with numbers",
			pattern: &Pattern{
				Regex:    "(db|cache)[0-9]",
				Coverage: 20,
			},
			expected: 20, // 2 × 10
		},
		{
			name: "Pattern with optional level",
			pattern: &Pattern{
				Regex:    "api(\\.staging)?",
				Coverage: 2,
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("estimateGenerativity removed - now using DSL-based generativity calculation")
			// Old test was for regex-based estimation
			// New approach: generativity calculated during DSL pattern generation
		})
	}
}

func TestQualityFiltering(t *testing.T) {
	// Note: isGoodPattern method removed - now using DSLPattern.PassesQualityFilter()
	tests := []struct {
		name       string
		pattern    *Pattern
		shouldPass bool
	}{
		{
			name: "Small pattern - auto-accept",
			pattern: &Pattern{
				Regex:    "(api|web|cdn)",
				Coverage: 3,
			},
			shouldPass: true, // generativity=3 < 500
		},
		{
			name: "Good ratio",
			pattern: &Pattern{
				Regex:    "(api|web)-(dev|prod)",
				Coverage: 4,
			},
			shouldPass: true, // generativity=4, ratio=1.0 < 25
		},
		{
			name: "Bad ratio - but under absolute limit",
			pattern: &Pattern{
				Regex:    "(a|b|c|d|e|f|g|h|i|j)-(1|2|3|4|5|6|7|8|9|0)",
				Coverage: 2,
			},
			shouldPass: true, // generativity=100 < 500 (auto-accept despite ratio=50 > 25)
		},
		{
			name: "Bad ratio - above absolute limit",
			pattern: &Pattern{
				// This will generate 10 × 10 × 10 = 1000 possibilities
				Regex:    "(a|b|c|d|e|f|g|h|i|j)-(0|1|2|3|4|5|6|7|8|9)-(x|y|z|w|v|u|t|s|r|p)",
				Coverage: 3,
			},
			shouldPass: false, // generativity=1000 > 500, ratio=333 > 25
		},
		{
			name: "Edge case - exactly at limit",
			pattern: &Pattern{
				Regex:    "api",
				Coverage: 1,
			},
			shouldPass: true, // generativity=1, ratio=1.0 < 25
		},
		{
			name: "Large but acceptable",
			pattern: &Pattern{
				Regex:    "(api|web)[0-9]",
				Coverage: 20,
			},
			shouldPass: true, // generativity=20, ratio=1.0 < 25
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("isGoodPattern removed - now using DSLPattern.PassesQualityFilter()")
			// Old test was for regex-based quality filtering
			// New approach: quality filtering done during DSL pattern generation
		})
	}
}

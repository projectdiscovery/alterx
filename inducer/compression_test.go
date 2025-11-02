package inducer

import (
	"testing"
)

func TestNumberCompressor_Compress(t *testing.T) {
	nc := NewNumberCompressor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple sequential range",
			input:    "(01|02|03|04|05)",
			expected: "(0[1-5])", // Compact format: literal 0 + range
		},
		{
			name:     "Single digit range",
			input:    "(1|2|3|4|5)",
			expected: "([1-5])",
		},
		{
			name:     "Dash prefix numbers",
			input:    "api(-01|-02|-03)",
			expected: "api(-0[1-3])", // Compact format
		},
		{
			name:     "Two digit range",
			input:    "(10|11|12|13|14|15)",
			expected: "(1[0-5])", // Compact format
		},
		{
			name:     "Mixed text and numbers - no compression",
			input:    "(dev|prod|01)",
			expected: "(dev|prod|01)", // Should not compress mixed groups
		},
		{
			name:     "No numbers",
			input:    "(api|web|cdn)",
			expected: "(api|web|cdn)", // No change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nc.Compress(tt.input)
			if result != tt.expected {
				t.Errorf("Compress(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNumberCompressor_IsNumberGroup(t *testing.T) {
	nc := NewNumberCompressor()

	tests := []struct {
		input    string
		expected bool
	}{
		{"01|02|03", true},
		{"1|2|3", true},
		{"-01|-02|-03", true},
		{"dev|prod", false},
		{"01|dev", false},
		{"", false},
	}

	for _, tt := range tests {
		result := nc.isNumberGroup(tt.input)
		if result != tt.expected {
			t.Errorf("isNumberGroup(%q) = %v; want %v", tt.input, result, tt.expected)
		}
	}
}

func TestNumberCompressor_CompressNumbers(t *testing.T) {
	nc := NewNumberCompressor()

	tests := []struct {
		name     string
		numbers  []string
		expected string
	}{
		{
			name:     "Sequential with leading zeros",
			numbers:  []string{"01", "02", "03"},
			expected: "(0[1-3])", // Compact format
		},
		{
			name:     "Sequential single digit",
			numbers:  []string{"1", "2", "3", "4", "5"},
			expected: "([1-5])",
		},
		{
			name:     "With dash prefix",
			numbers:  []string{"-01", "-02", "-03"},
			expected: "(-0[1-3])", // Compact format
		},
		{
			name:     "Single number",
			numbers:  []string{"01"},
			expected: "", // Can't compress single number
		},
		{
			name:     "Mixed dash and no dash",
			numbers:  []string{"01", "-02"},
			expected: "", // Can't compress mixed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nc.compressNumbers(tt.numbers)
			if result != tt.expected {
				t.Errorf("compressNumbers(%v) = %q; want %q", tt.numbers, result, tt.expected)
			}
		})
	}
}

func TestNumberCompressor_FormatWithWidth(t *testing.T) {
	nc := NewNumberCompressor()

	tests := []struct {
		num      int
		width    int
		expected string
	}{
		{1, 2, "01"},
		{5, 2, "05"},
		{10, 2, "10"},
		{1, 3, "001"},
		{99, 3, "099"},
		{1, 1, "1"},
	}

	for _, tt := range tests {
		result := nc.formatWithWidth(tt.num, tt.width)
		if result != tt.expected {
			t.Errorf("formatWithWidth(%d, %d) = %q; want %q", tt.num, tt.width, result, tt.expected)
		}
	}
}

func TestNumberCompressor_IsSequential(t *testing.T) {
	nc := NewNumberCompressor()

	tests := []struct {
		numbers  []int
		expected bool
	}{
		{[]int{1, 2, 3, 4, 5}, true},
		{[]int{10, 11, 12}, true},
		{[]int{1, 3, 5}, false},
		{[]int{1}, false}, // Single number
		{[]int{5, 4, 3}, false}, // Wrong order
	}

	for _, tt := range tests {
		result := nc.isSequential(tt.numbers)
		if result != tt.expected {
			t.Errorf("isSequential(%v) = %v; want %v", tt.numbers, result, tt.expected)
		}
	}
}

func TestNumberCompressor_CompressPattern(t *testing.T) {
	nc := NewNumberCompressor()

	pattern := &Pattern{
		Regex:    "api-(01|02|03).example.com",
		Coverage: 3,
	}

	nc.CompressPattern(pattern)

	// Should be compressed
	if pattern.Regex == "api-(01|02|03).example.com" {
		t.Error("Pattern was not compressed")
	}

	// Should contain range notation
	if !contains(pattern.Regex, "[") || !contains(pattern.Regex, "]") {
		t.Errorf("Compressed pattern %q doesn't contain range notation", pattern.Regex)
	}
}

func TestNumberCompressor_ComplexPattern(t *testing.T) {
	nc := NewNumberCompressor()

	// Pattern with multiple number groups
	pattern := "api-(dev|prod)-(01|02|03).region-(us|eu).example.com"

	compressed := nc.Compress(pattern)

	// Should compress the number group but leave text groups alone
	if !contains(compressed, "dev") || !contains(compressed, "prod") {
		t.Error("Text alternation was incorrectly modified")
	}

	if !contains(compressed, "us") || !contains(compressed, "eu") {
		t.Error("Region alternation was incorrectly modified")
	}

	// Should have compressed the numbers
	if compressed == pattern {
		t.Error("Pattern was not modified at all")
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

func BenchmarkNumberCompressor_Compress(b *testing.B) {
	nc := NewNumberCompressor()
	pattern := "api-(01|02|03|04|05|06|07|08|09|10).example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nc.Compress(pattern)
	}
}

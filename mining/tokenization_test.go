package mining

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []TokenizedSubdomain
	}{
		{
			name:  "simple subdomain with hyphen and number",
			input: []string{"api-prod-12"},
			expected: []TokenizedSubdomain{
				{
					Original: "api-prod-12",
					Levels: []Level{
						{Label: "api-prod-12", Tokens: []string{"api", "-prod", "-12"}},
					},
				},
			},
		},
		{
			name:  "single word subdomain",
			input: []string{"web"},
			expected: []TokenizedSubdomain{
				{
					Original: "web",
					Levels: []Level{
						{Label: "web", Tokens: []string{"web"}},
					},
				},
			},
		},
		{
			name:  "multi-level subdomain",
			input: []string{"api.dev"},
			expected: []TokenizedSubdomain{
				{
					Original: "api.dev",
					Levels: []Level{
						{Label: "api", Tokens: []string{"api"}},
						{Label: "dev", Tokens: []string{"dev"}},
					},
				},
			},
		},
		{
			name:  "hyphenated number",
			input: []string{"foo-12"},
			expected: []TokenizedSubdomain{
				{
					Original: "foo-12",
					Levels: []Level{
						{Label: "foo-12", Tokens: []string{"foo", "-12"}},
					},
				},
			},
		},
		{
			name:  "alphanumeric without hyphen",
			input: []string{"web01"},
			expected: []TokenizedSubdomain{
				{
					Original: "web01",
					Levels: []Level{
						{Label: "web01", Tokens: []string{"web", "01"}},
					},
				},
			},
		},
		{
			name:  "complex subdomain with numbers",
			input: []string{"api5-dev-staging2"},
			expected: []TokenizedSubdomain{
				{
					Original: "api5-dev-staging2",
					Levels: []Level{
						{Label: "api5-dev-staging2", Tokens: []string{"api", "5", "-dev", "-staging", "2"}},
					},
				},
			},
		},
		{
			name:  "multiple subdomains",
			input: []string{"api", "web"},
			expected: []TokenizedSubdomain{
				{
					Original: "api",
					Levels: []Level{
						{Label: "api", Tokens: []string{"api"}},
					},
				},
				{
					Original: "web",
					Levels: []Level{
						{Label: "web", Tokens: []string{"web"}},
					},
				},
			},
		},
		{
			name:  "empty subdomain",
			input: []string{""},
			expected: []TokenizedSubdomain{
				{
					Original: "",
					Levels:   []Level{},
				},
			},
		},
		{
			name:  "multiple hyphens",
			input: []string{"api-v1-prod-us-west"},
			expected: []TokenizedSubdomain{
				{
					Original: "api-v1-prod-us-west",
					Levels: []Level{
						{Label: "api-v1-prod-us-west", Tokens: []string{"api", "-v", "1", "-prod", "-us", "-west"}},
					},
				},
			},
		},
		{
			name:  "numbers at start",
			input: []string{"123api"},
			expected: []TokenizedSubdomain{
				{
					Original: "123api",
					Levels: []Level{
						{Label: "123api", Tokens: []string{"123", "api"}},
					},
				},
			},
		},
		{
			name:  "multi-level with complex tokens",
			input: []string{"api5.dev-staging2"},
			expected: []TokenizedSubdomain{
				{
					Original: "api5.dev-staging2",
					Levels: []Level{
						{Label: "api5", Tokens: []string{"api", "5"}},
						{Label: "dev-staging2", Tokens: []string{"dev", "-staging", "2"}},
					},
				},
			},
		},
		{
			name:  "consecutive numbers",
			input: []string{"api123456"},
			expected: []TokenizedSubdomain{
				{
					Original: "api123456",
					Levels: []Level{
						{Label: "api123456", Tokens: []string{"api", "123456"}},
					},
				},
			},
		},
		{
			name:  "hyphen at multiple positions",
			input: []string{"prod-web-api-v2"},
			expected: []TokenizedSubdomain{
				{
					Original: "prod-web-api-v2",
					Levels: []Level{
						{Label: "prod-web-api-v2", Tokens: []string{"prod", "-web", "-api", "-v", "2"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tokenize(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Tokenize() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestTokenizeLabel(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected []string
	}{
		{
			name:     "simple word",
			label:    "api",
			expected: []string{"api"},
		},
		{
			name:     "word with number",
			label:    "api5",
			expected: []string{"api", "5"},
		},
		{
			name:     "hyphenated words",
			label:    "api-prod",
			expected: []string{"api", "-prod"},
		},
		{
			name:     "hyphenated with number",
			label:    "api-prod-12",
			expected: []string{"api", "-prod", "-12"},
		},
		{
			name:     "word hyphen number",
			label:    "foo-12",
			expected: []string{"foo", "-12"},
		},
		{
			name:     "number only",
			label:    "123",
			expected: []string{"123"},
		},
		{
			name:     "mixed alphanumeric",
			label:    "web01test99",
			expected: []string{"web", "01", "test", "99"},
		},
		{
			name:     "multiple hyphens",
			label:    "api-v1-prod",
			expected: []string{"api", "-v", "1", "-prod"},
		},
		{
			name:     "hyphen number hyphen word",
			label:    "test-123-prod",
			expected: []string{"test", "-123", "-prod"},
		},
		{
			name:     "consecutive numbers",
			label:    "api123456test",
			expected: []string{"api", "123456", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizeLabel(tt.label)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("tokenizeLabel(%q) = %v, want %v", tt.label, result, tt.expected)
			}
		})
	}
}

func TestSplitByNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "word with number",
			input:    "api12",
			expected: []string{"api", "12"},
		},
		{
			name:     "word only",
			input:    "prod",
			expected: []string{"prod"},
		},
		{
			name:     "number only",
			input:    "123",
			expected: []string{"123"},
		},
		{
			name:     "multiple numbers",
			input:    "api12web34",
			expected: []string{"api", "12", "web", "34"},
		},
		{
			name:     "number at start",
			input:    "123api",
			expected: []string{"123", "api"},
		},
		{
			name:     "hyphen prefix with number",
			input:    "-prod12",
			expected: []string{"-prod", "12"},
		},
		{
			name:     "consecutive numbers",
			input:    "test123456",
			expected: []string{"test", "123456"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitByNumbers(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitByNumbers(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkTokenize(b *testing.B) {
	input := []string{
		"api-prod-12",
		"web01.staging",
		"api5-dev-us-west-1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Tokenize(input)
	}
}

func BenchmarkTokenizeLabel(b *testing.B) {
	label := "api-prod-12-staging"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenizeLabel(label)
	}
}

func BenchmarkSplitByNumbers(b *testing.B) {
	input := "api12web34test56"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		splitByNumbers(input)
	}
}

// Table-driven test for edge cases
func TestTokenizeEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []TokenizedSubdomain
	}{
		{
			name:     "empty input",
			input:    []string{},
			expected: []TokenizedSubdomain{},
		},
		{
			name:  "empty string in array",
			input: []string{""},
			expected: []TokenizedSubdomain{
				{
					Original: "",
					Levels:   []Level{},
				},
			},
		},
		{
			name:  "multiple dots in subdomain",
			input: []string{"a.b.c.d"},
			expected: []TokenizedSubdomain{
				{
					Original: "a.b.c.d",
					Levels: []Level{
						{Label: "a", Tokens: []string{"a"}},
						{Label: "b", Tokens: []string{"b"}},
						{Label: "c", Tokens: []string{"c"}},
						{Label: "d", Tokens: []string{"d"}},
					},
				},
			},
		},
		{
			name:  "special characters with hyphens",
			input: []string{"api-v2-beta-3"},
			expected: []TokenizedSubdomain{
				{
					Original: "api-v2-beta-3",
					Levels: []Level{
						{Label: "api-v2-beta-3", Tokens: []string{"api", "-v", "2", "-beta", "-3"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tokenize(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Tokenize() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestExtractFirstToken(t *testing.T) {
	// Create a simple PatternMiner instance for testing
	domains := []string{"api.example.com", "web.example.com"}
	opts := &Options{
		MinLDist: 2,
		MaxLDist: 10,
	}
	pm, err := NewPatternMiner(domains, opts)
	if err != nil {
		t.Fatalf("Failed to create PatternMiner: %v", err)
	}

	tests := []struct {
		name      string
		subdomain string
		expected  string
	}{
		{
			name:      "simple word",
			subdomain: "api",
			expected:  "api",
		},
		{
			name:      "hyphenated words",
			subdomain: "api-prod",
			expected:  "api",
		},
		{
			name:      "with number",
			subdomain: "api5",
			expected:  "api",
		},
		{
			name:      "multiple levels",
			subdomain: "api.dev",
			expected:  "api",
		},
		{
			name:      "complex with hyphens and numbers",
			subdomain: "api-prod-12",
			expected:  "api",
		},
		{
			name:      "starts with number",
			subdomain: "123api",
			expected:  "123",
		},
		{
			name:      "empty string",
			subdomain: "",
			expected:  "",
		},
		{
			name:      "multi-level complex",
			subdomain: "api5-dev.staging",
			expected:  "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.extractFirstToken(tt.subdomain)
			if result != tt.expected {
				t.Errorf("extractFirstToken(%q) = %q, want %q", tt.subdomain, result, tt.expected)
			}
		})
	}
}

package inducer

import (
	"strings"
	"testing"
)

func TestPatternGenerator_GeneratePattern(t *testing.T) {
	pg := NewPatternGenerator()

	tests := []struct {
		name     string
		domains  []string
		wantErr  bool
		contains []string // Substrings that should appear in the regex
	}{
		{
			name: "Simple number variation",
			domains: []string{
				"api-dev-01.example.com",
				"api-dev-02.example.com",
				"api-dev-03.example.com",
			},
			wantErr:  false,
			contains: []string{"api", "-dev", "01", "02", "03"},
		},
		{
			name: "Environment variation",
			domains: []string{
				"api-dev.example.com",
				"api-prod.example.com",
				"api-staging.example.com",
			},
			wantErr:  false,
			contains: []string{"api", "dev", "prod", "staging"},
		},
		{
			name: "Single domain",
			domains: []string{
				"api.example.com",
			},
			wantErr: true, // Should fail for single domain
		},
		{
			name:    "Empty closure",
			domains: []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closure := &Closure{
				Domains: tt.domains,
				Delta:   3,
				Size:    len(tt.domains),
			}

			pattern, err := pg.GeneratePattern(closure)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if pattern == nil {
				t.Fatal("Pattern is nil")
			}

			// Check that regex contains expected substrings
			for _, substr := range tt.contains {
				if !strings.Contains(pattern.Regex, substr) && !strings.Contains(pattern.Regex, escapeRegex(substr)) {
					t.Errorf("Pattern %q does not contain %q (or escaped version)", pattern.Regex, substr)
				}
			}

			// Check coverage
			if pattern.Coverage != len(tt.domains) {
				t.Errorf("Coverage = %d; want %d", pattern.Coverage, len(tt.domains))
			}
		})
	}
}

func TestPatternGenerator_MultiLevel(t *testing.T) {
	pg := NewPatternGenerator()

	domains := []string{
		"api.staging.example.com",
		"api.prod.example.com",
		"web.staging.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   5,
		Size:    len(domains),
	}

	pattern, err := pg.GeneratePattern(closure)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain alternations for both levels
	if !strings.Contains(pattern.Regex, "api") || !strings.Contains(pattern.Regex, "web") {
		t.Errorf("Pattern missing first level alternation: %s", pattern.Regex)
	}

	if !strings.Contains(pattern.Regex, "staging") || !strings.Contains(pattern.Regex, "prod") {
		t.Errorf("Pattern missing second level alternation: %s", pattern.Regex)
	}
}

func TestPatternGenerator_OptionalLevel(t *testing.T) {
	pg := NewPatternGenerator()

	domains := []string{
		"api.staging.example.com",
		"api.example.com", // Missing second level
	}

	closure := &Closure{
		Domains: domains,
		Delta:   10,
		Size:    len(domains),
	}

	pattern, err := pg.GeneratePattern(closure)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should mark staging level as optional with ?
	if !strings.Contains(pattern.Regex, "?") {
		t.Errorf("Pattern should contain optional marker (?): %s", pattern.Regex)
	}
}

func TestEscapeRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"api", "api"},
		{"api.staging", "api\\.staging"},
		{"api-dev", "api-dev"}, // Dash doesn't need escaping in our context
		{"api*", "api\\*"},
		{"api+test", "api\\+test"},
		{"[abc]", "\\[abc\\]"},
		{"api(test)", "api\\(test\\)"},
	}

	for _, tt := range tests {
		result := escapeRegex(tt.input)
		if result != tt.expected {
			t.Errorf("escapeRegex(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestPatternGenerator_GeneratePatternsFromClosures(t *testing.T) {
	pg := NewPatternGenerator()

	closures := []*Closure{
		{
			Domains: []string{
				"api-dev-01.example.com",
				"api-dev-02.example.com",
			},
			Delta: 2,
			Size:  2,
		},
		{
			Domains: []string{
				"web-prod.example.com",
				"web-staging.example.com",
			},
			Delta: 5,
			Size:  2,
		},
		{
			Domains: []string{
				"single.example.com", // Should be skipped
			},
			Delta: 1,
			Size:  1,
		},
	}

	patterns := pg.GeneratePatternsFromClosures(closures)

	// Should generate 2 patterns (third closure has single domain)
	if len(patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(patterns))
	}

	// Each pattern should have coverage >= 2
	for _, pattern := range patterns {
		if pattern.Coverage < 2 {
			t.Errorf("Pattern has coverage %d; want >= 2", pattern.Coverage)
		}
	}
}

func TestPatternGenerator_BuildLevelPositionMap(t *testing.T) {
	pg := NewPatternGenerator()

	tokenized := []*TokenizedDomain{
		{
			Levels: []Level{
				{
					Index: 0,
					Tokens: []Token{
						{Value: "api", Type: TokenTypeWord, Position: 0},
						{Value: "-dev", Type: TokenTypeDash, Position: 1},
					},
				},
			},
		},
		{
			Levels: []Level{
				{
					Index: 0,
					Tokens: []Token{
						{Value: "api", Type: TokenTypeWord, Position: 0},
						{Value: "-prod", Type: TokenTypeDash, Position: 1},
					},
				},
			},
		},
	}

	levelMap := pg.buildLevelPositionMap(tokenized)

	// Should have level 0
	if levelMap[0] == nil {
		t.Fatal("Level 0 not found in map")
	}

	// Position 0 should have "api"
	if !levelMap[0][0]["api"] {
		t.Error("Position 0 should contain 'api'")
	}

	// Position 1 should have both "-dev" and "-prod"
	if !levelMap[0][1]["-dev"] || !levelMap[0][1]["-prod"] {
		t.Error("Position 1 should contain both '-dev' and '-prod'")
	}
}

func TestPatternGenerator_IsLevelOptional(t *testing.T) {
	pg := NewPatternGenerator()

	tokenized := []*TokenizedDomain{
		{
			Levels: []Level{
				{Index: 0, Tokens: []Token{{Value: "api", Position: 0}}},
				{Index: 1, Tokens: []Token{{Value: "staging", Position: 0}}},
			},
		},
		{
			Levels: []Level{
				{Index: 0, Tokens: []Token{{Value: "api", Position: 0}}},
				// Missing level 1
			},
		},
	}

	// Level 0 should NOT be optional (all domains have it)
	if pg.isLevelOptional(0, tokenized) {
		t.Error("Level 0 should not be optional")
	}

	// Level 1 should be optional (second domain missing it)
	if !pg.isLevelOptional(1, tokenized) {
		t.Error("Level 1 should be optional")
	}
}

func BenchmarkPatternGenerator_GeneratePattern(b *testing.B) {
	pg := NewPatternGenerator()

	domains := make([]string, 10)
	for i := 0; i < 10; i++ {
		domains[i] = "api-dev-" + string(rune('0'+i)) + ".example.com"
	}

	closure := &Closure{
		Domains: domains,
		Delta:   3,
		Size:    len(domains),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pg.GeneratePattern(closure)
	}
}

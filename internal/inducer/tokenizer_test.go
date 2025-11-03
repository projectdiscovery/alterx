package inducer

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSubdomain string
		wantRoot    string
		wantLevels  int
		wantLevel0  []string // Expected token values at level 0
		wantLevel1  []string // Expected token values at level 1 (if exists)
	}{
		{
			name:        "simple single-level subdomain",
			input:       "api.example.com",
			wantSubdomain: "api",
			wantRoot:    "example.com",
			wantLevels:  1,
			wantLevel0:  []string{"api"},
		},
		{
			name:        "dash-separated subdomain",
			input:       "api-dev.example.com",
			wantSubdomain: "api-dev",
			wantRoot:    "example.com",
			wantLevels:  1,
			wantLevel0:  []string{"api", "-dev"},
		},
		{
			name:        "three-part dash-separated",
			input:       "api-dev-01.example.com",
			wantSubdomain: "api-dev-01",
			wantRoot:    "example.com",
			wantLevels:  1,
			wantLevel0:  []string{"api", "-dev", "-01"},
		},
		{
			name:        "multi-level subdomain",
			input:       "api.staging.example.com",
			wantSubdomain: "api.staging",
			wantRoot:    "example.com",
			wantLevels:  2,
			wantLevel0:  []string{"api"},
			wantLevel1:  []string{"staging"},
		},
		{
			name:        "three-level subdomain",
			input:       "api.v1.staging.example.com",
			wantSubdomain: "api.v1.staging",
			wantRoot:    "example.com",
			wantLevels:  3,
			wantLevel0:  []string{"api"},
			wantLevel1:  []string{"v", "1"}, // v1 is split into "v" and "1" per regulator algorithm
		},
		{
			name:        "number in token (not separated by dash)",
			input:       "api01.example.com",
			wantSubdomain: "api01",
			wantRoot:    "example.com",
			wantLevels:  1,
			wantLevel0:  []string{"api", "01"},
		},
		{
			name:        "complex multi-level with dashes and numbers",
			input:       "web-us-east-1.prod.internal.example.com",
			wantSubdomain: "web-us-east-1.prod.internal",
			wantRoot:    "example.com",
			wantLevels:  3,
			wantLevel0:  []string{"web", "-us", "-east", "-1"},
			wantLevel1:  []string{"prod"},
		},
		{
			name:        "db with number (no dash)",
			input:       "db01.prod.example.com",
			wantSubdomain: "db01.prod",
			wantRoot:    "example.com",
			wantLevels:  2,
			wantLevel0:  []string{"db", "01"},
			wantLevel1:  []string{"prod"},
		},
		{
			name:        "single level subdomain",
			input:       "cdn.example.com",
			wantSubdomain: "cdn",
			wantRoot:    "example.com",
			wantLevels:  1,
			wantLevel0:  []string{"cdn"},
		},
		{
			name:        "mixed numbers and text",
			input:       "api123test.example.com",
			wantSubdomain: "api123test",
			wantRoot:    "example.com",
			wantLevels:  1,
			wantLevel0:  []string{"api", "123", "test"},
		},
		{
			name:        "public suffix with multiple parts (co.uk)",
			input:       "api.example.co.uk",
			wantSubdomain: "api",
			wantRoot:    "example.co.uk",
			wantLevels:  1,
			wantLevel0:  []string{"api"},
		},
		{
			name:        "wildcard subdomain",
			input:       "*.example.com",
			wantSubdomain: "",
			wantRoot:    "example.com",
			wantLevels:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize() error = %v", err)
			}

			// Check subdomain extraction
			if result.Subdomain != tt.wantSubdomain {
				t.Errorf("Subdomain = %v, want %v", result.Subdomain, tt.wantSubdomain)
			}

			// Check root domain
			if result.Root != tt.wantRoot {
				t.Errorf("Root = %v, want %v", result.Root, tt.wantRoot)
			}

			// Check number of levels
			if len(result.Levels) != tt.wantLevels {
				t.Errorf("Level count = %v, want %v", len(result.Levels), tt.wantLevels)
			}

			// Check level 0 tokens
			if tt.wantLevels > 0 && len(tt.wantLevel0) > 0 {
				level0 := &result.Levels[0]
				if level0 == nil {
					t.Fatal("Level 0 is nil")
				}

				if len(level0.Tokens) != len(tt.wantLevel0) {
					t.Errorf("Level 0 token count = %v, want %v", len(level0.Tokens), len(tt.wantLevel0))
				}

				for i, wantToken := range tt.wantLevel0 {
					if i >= len(level0.Tokens) {
						break
					}
					if level0.Tokens[i].Value != wantToken {
						t.Errorf("Level 0 token[%d] = %v, want %v", i, level0.Tokens[i].Value, wantToken)
					}
				}
			}

			// Check level 1 tokens (if applicable)
			if tt.wantLevels > 1 && len(tt.wantLevel1) > 0 {
				level1 := &result.Levels[1]
				if level1 == nil {
					t.Fatal("Level 1 is nil")
				}

				if len(level1.Tokens) != len(tt.wantLevel1) {
					t.Errorf("Level 1 token count = %v, want %v", len(level1.Tokens), len(tt.wantLevel1))
				}

				for i, wantToken := range tt.wantLevel1 {
					if i >= len(level1.Tokens) {
						break
					}
					if level1.Tokens[i].Value != wantToken {
						t.Errorf("Level 1 token[%d] = %v, want %v", i, level1.Tokens[i].Value, wantToken)
					}
				}
			}
		})
	}
}

func TestTokenizeLevel(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantTokens []string
		wantTypes  []TokenType
	}{
		{
			name:       "simple word",
			input:      "api",
			wantTokens: []string{"api"},
			wantTypes:  []TokenType{TokenTypeWord},
		},
		{
			name:       "dash-separated",
			input:      "api-dev",
			wantTokens: []string{"api", "-dev"},
			wantTypes:  []TokenType{TokenTypeWord, TokenTypeDash},
		},
		{
			name:       "three-part dash",
			input:      "api-dev-prod",
			wantTokens: []string{"api", "-dev", "-prod"},
			wantTypes:  []TokenType{TokenTypeWord, TokenTypeDash, TokenTypeDash},
		},
		{
			name:       "hyphenated number",
			input:      "api-01",
			wantTokens: []string{"api", "-01"},
			wantTypes:  []TokenType{TokenTypeWord, TokenTypeDash},
		},
		{
			name:       "number without dash",
			input:      "api01",
			wantTokens: []string{"api", "01"},
			wantTypes:  []TokenType{TokenTypeWord, TokenTypeNumber},
		},
		{
			name:       "mixed numbers and text",
			input:      "server123test",
			wantTokens: []string{"server", "123", "test"},
			wantTypes:  []TokenType{TokenTypeWord, TokenTypeNumber, TokenTypeWord},
		},
		{
			name:       "complex with dashes and numbers",
			input:      "web-us-east-1",
			wantTokens: []string{"web", "-us", "-east", "-1"},
			wantTypes:  []TokenType{TokenTypeWord, TokenTypeDash, TokenTypeDash, TokenTypeDash},
		},
		{
			name:       "just a number",
			input:      "01",
			wantTokens: []string{"01"},
			wantTypes:  []TokenType{TokenTypeNumber},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizeLevel(tt.input)

			if len(tokens) != len(tt.wantTokens) {
				t.Fatalf("Token count = %v, want %v", len(tokens), len(tt.wantTokens))
			}

			for i, wantToken := range tt.wantTokens {
				if tokens[i].Value != wantToken {
					t.Errorf("Token[%d].Value = %v, want %v", i, tokens[i].Value, wantToken)
				}

				if tokens[i].Type != tt.wantTypes[i] {
					t.Errorf("Token[%d].Type = %v, want %v", i, tokens[i].Type, tt.wantTypes[i])
				}

				if tokens[i].Position != i {
					t.Errorf("Token[%d].Position = %v, want %v", i, tokens[i].Position, i)
				}
			}
		})
	}
}

func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		name           string
		hostname       string
		wantSubdomain  string
		wantRoot       string
		wantErr        bool
	}{
		{
			name:          "simple subdomain",
			hostname:      "api.example.com",
			wantSubdomain: "api",
			wantRoot:      "example.com",
			wantErr:       false,
		},
		{
			name:          "multi-level subdomain",
			hostname:      "api.staging.example.com",
			wantSubdomain: "api.staging",
			wantRoot:      "example.com",
			wantErr:       false,
		},
		{
			name:          "no subdomain (just root)",
			hostname:      "example.com",
			wantSubdomain: "",
			wantRoot:      "example.com",
			wantErr:       false,
		},
		{
			name:          "public suffix co.uk",
			hostname:      "api.example.co.uk",
			wantSubdomain: "api",
			wantRoot:      "example.co.uk",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subdomain, root, err := extractSubdomain(tt.hostname)

			if (err != nil) != tt.wantErr {
				t.Errorf("extractSubdomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if subdomain != tt.wantSubdomain {
				t.Errorf("subdomain = %v, want %v", subdomain, tt.wantSubdomain)
			}

			if root != tt.wantRoot {
				t.Errorf("root = %v, want %v", root, tt.wantRoot)
			}
		})
	}
}

func TestSplitByNumbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "no numbers",
			input: "api",
			want:  []string{"api"},
		},
		{
			name:  "number at end",
			input: "api01",
			want:  []string{"api", "01"},
		},
		{
			name:  "number in middle",
			input: "api123test",
			want:  []string{"api", "123", "test"},
		},
		{
			name:  "just number",
			input: "123",
			want:  []string{"123"},
		},
		{
			name:  "hyphenated number (should not split)",
			input: "-01",
			want:  []string{"-01"},
		},
		{
			name:  "multiple numbers",
			input: "server01rack02",
			want:  []string{"server", "01", "rack", "02"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitByNumbers(tt.input)

			if len(result) != len(tt.want) {
				t.Errorf("splitByNumbers() count = %v, want %v", len(result), len(tt.want))
			}

			for i, want := range tt.want {
				if i >= len(result) {
					break
				}
				if result[i] != want {
					t.Errorf("splitByNumbers()[%d] = %v, want %v", i, result[i], want)
				}
			}
		})
	}
}

func TestClassifyToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  TokenType
	}{
		{
			name:  "word token",
			token: "api",
			want:  TokenTypeWord,
		},
		{
			name:  "dash token",
			token: "-dev",
			want:  TokenTypeDash,
		},
		{
			name:  "number token",
			token: "01",
			want:  TokenTypeNumber,
		},
		{
			name:  "hyphenated number",
			token: "-01",
			want:  TokenTypeDash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyToken(tt.token)
			if result != tt.want {
				t.Errorf("classifyToken(%v) = %v, want %v", tt.token, result, tt.want)
			}
		})
	}
}

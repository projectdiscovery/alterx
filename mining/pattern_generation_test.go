package mining

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePattern(t *testing.T) {
	tests := []struct {
		name               string
		subdomains         []string
		expectedPattern    string
		expectedPayloadLen int
		checkPayloads      map[string][]string
	}{
		{
			name:               "simple static pattern",
			subdomains:         []string{"api", "api", "api"},
			expectedPattern:    "api",
			expectedPayloadLen: 0,
		},
		{
			name:               "single level with variable",
			subdomains:         []string{"api-prod", "api-staging"},
			expectedPattern:    "api{{p0}}",
			expectedPayloadLen: 1,
			checkPayloads: map[string][]string{
				"p0": {"-prod", "-staging"},
			},
		},
		{
			name:               "single level with number variation",
			subdomains:         []string{"api-1", "api-2", "api-3"},
			expectedPattern:    "api{{p0}}",
			expectedPayloadLen: 1,
			checkPayloads: map[string][]string{
				"p0": {"-1", "-2", "-3"},
			},
		},
		{
			name:               "complex single level",
			subdomains:         []string{"api-prod-1", "api-prod-2", "api-staging-1"},
			expectedPattern:    "api{{p0}}{{p1}}",
			expectedPayloadLen: 2,
			checkPayloads: map[string][]string{
				"p0": {"-prod", "-staging"},
				"p1": {"-1", "-2"},
			},
		},
		{
			name:               "multi-level simple",
			subdomains:         []string{"api.dev", "api.prod"},
			expectedPattern:    "api.{{p0}}",
			expectedPayloadLen: 1,
			checkPayloads: map[string][]string{
				"p0": {"dev", "prod"},
			},
		},
		{
			name:               "multi-level complex",
			subdomains:         []string{"api-1.dev", "api-2.dev", "api-1.prod"},
			expectedPattern:    "api{{p0}}.{{p1}}",
			expectedPayloadLen: 2,
			checkPayloads: map[string][]string{
				"p0": {"-1", "-2"},
				"p1": {"dev", "prod"},
			},
		},
		{
			name:               "with numbers",
			subdomains:         []string{"web01", "web02", "web03"},
			expectedPattern:    "web{{p0}}",
			expectedPayloadLen: 1,
			checkPayloads: map[string][]string{
				"p0": {"01", "02", "03"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pattern miner instance
			domains := make([]string, len(tt.subdomains))
			for i, sub := range tt.subdomains {
				domains[i] = sub + ".example.com"
			}

			pm, err := NewPatternMiner(domains, &Options{
				MinLDist: 2,
				MaxLDist: 10,
			})
			require.NoError(t, err, "Failed to create PatternMiner")

			// Generate pattern
			pattern, err := pm.generatePattern(tt.subdomains)
			require.NoError(t, err, "generatePattern() should not return error")
			require.NotNil(t, pattern, "generatePattern() should not return nil pattern")

			// Check pattern string
			assert.Equal(t, tt.expectedPattern, pattern.Pattern, "Pattern mismatch")

			// Check payload count
			assert.Len(t, pattern.Payloads, tt.expectedPayloadLen, "Payload count mismatch")

			// Check specific payloads if provided
			if tt.checkPayloads != nil {
				for varName, expectedValues := range tt.checkPayloads {
					actualValues, ok := pattern.Payloads[varName]
					require.True(t, ok, "Payload %q not found in result", varName)

					// Check if all expected values are present
					for _, expectedVal := range expectedValues {
						assert.Contains(t, actualValues, expectedVal, "Payload %q missing expected value %q", varName, expectedVal)
					}
				}
			}
		})
	}
}

func TestAnalyzeTokenAlignment(t *testing.T) {
	tests := []struct {
		name           string
		subdomains     []string
		expectedLevels int
		checkLevel0    func(t *testing.T, lp LevelPosition)
	}{
		{
			name:           "static single level",
			subdomains:     []string{"api", "api", "api"},
			expectedLevels: 1,
			checkLevel0: func(t *testing.T, lp LevelPosition) {
				assert.Len(t, lp.Positions, 1, "Expected 1 position")
				assert.Equal(t, TokenPositionStatic, lp.Positions[0].Type, "Expected static token")
			},
		},
		{
			name:           "variable single level",
			subdomains:     []string{"api-prod", "api-staging"},
			expectedLevels: 1,
			checkLevel0: func(t *testing.T, lp LevelPosition) {
				assert.Len(t, lp.Positions, 2, "Expected 2 positions")
				// First token "api" should be static
				assert.Equal(t, TokenPositionStatic, lp.Positions[0].Type, "Expected first token to be static")
				// Second token should be variable
				assert.Equal(t, TokenPositionVariable, lp.Positions[1].Type, "Expected second token to be variable")
			},
		},
		{
			name:           "multi-level",
			subdomains:     []string{"api.dev", "api.prod"},
			expectedLevels: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create pattern miner
			domains := make([]string, len(tt.subdomains))
			for i, sub := range tt.subdomains {
				domains[i] = sub + ".example.com"
			}

			pm, err := NewPatternMiner(domains, &Options{
				MinLDist: 2,
				MaxLDist: 10,
			})
			require.NoError(t, err, "Failed to create PatternMiner")

			// Tokenize
			tokenized := Tokenize(tt.subdomains)

			// Analyze alignment
			positions := pm.analyzeTokenAlignment(tokenized)

			// Check level count
			assert.Len(t, positions, tt.expectedLevels, "Level count mismatch")

			// Run custom checks if provided
			if tt.checkLevel0 != nil && len(positions) > 0 {
				tt.checkLevel0(t, positions[0])
			}
		})
	}
}

func TestBuildDSLPattern(t *testing.T) {
	tests := []struct {
		name            string
		levelPositions  []LevelPosition
		expectedPattern string
	}{
		{
			name: "single static level",
			levelPositions: []LevelPosition{
				{
					LevelIndex: 0,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionStatic, Values: []string{"api"}},
					},
				},
			},
			expectedPattern: "api",
		},
		{
			name: "single level with variable",
			levelPositions: []LevelPosition{
				{
					LevelIndex: 0,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionStatic, Values: []string{"api"}},
						{Index: 1, Type: TokenPositionVariable, VarName: "p0", Values: []string{"-prod", "-staging"}},
					},
				},
			},
			expectedPattern: "api{{p0}}",
		},
		{
			name: "multi-level pattern",
			levelPositions: []LevelPosition{
				{
					LevelIndex: 0,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionStatic, Values: []string{"api"}},
					},
				},
				{
					LevelIndex: 1,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionVariable, VarName: "p0", Values: []string{"dev", "prod"}},
					},
				},
			},
			expectedPattern: "api.{{p0}}",
		},
		{
			name: "complex pattern",
			levelPositions: []LevelPosition{
				{
					LevelIndex: 0,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionStatic, Values: []string{"api"}},
						{Index: 1, Type: TokenPositionVariable, VarName: "p0", Values: []string{"-1", "-2"}},
					},
				},
				{
					LevelIndex: 1,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionVariable, VarName: "p1", Values: []string{"dev", "prod"}},
					},
				},
			},
			expectedPattern: "api{{p0}}.{{p1}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a dummy pattern miner
			pm := &PatternMiner{}

			pattern := pm.buildDSLPattern(tt.levelPositions)

			assert.Equal(t, tt.expectedPattern, pattern, "Pattern mismatch")
		})
	}
}

func TestExtractPayloads(t *testing.T) {
	tests := []struct {
		name             string
		levelPositions   []LevelPosition
		subdomains       []string
		expectedPayloads map[string]int      // varName -> count of unique values
		checkContains    map[string][]string // varName -> values that must be present
	}{
		{
			name: "single variable",
			levelPositions: []LevelPosition{
				{
					LevelIndex: 0,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionStatic, Values: []string{"api"}},
						{Index: 1, Type: TokenPositionVariable, VarName: "p0", Values: []string{"-prod", "-staging"}},
					},
				},
			},
			subdomains: []string{"api-prod", "api-staging"},
			expectedPayloads: map[string]int{
				"p0": 2,
			},
			checkContains: map[string][]string{
				"p0": {"-prod", "-staging"},
			},
		},
		{
			name: "multiple variables",
			levelPositions: []LevelPosition{
				{
					LevelIndex: 0,
					Positions: []TokenPosition{
						{Index: 0, Type: TokenPositionStatic, Values: []string{"api"}},
						{Index: 1, Type: TokenPositionVariable, VarName: "p0", Values: []string{"-prod", "-staging"}},
						{Index: 2, Type: TokenPositionVariable, VarName: "p1", Values: []string{"-1", "-2"}},
					},
				},
			},
			subdomains: []string{"api-prod-1", "api-staging-2"},
			expectedPayloads: map[string]int{
				"p0": 2,
				"p1": 2,
			},
			checkContains: map[string][]string{
				"p0": {"-prod", "-staging"},
				"p1": {"-1", "-2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PatternMiner{}
			tokenized := Tokenize(tt.subdomains)

			payloads := pm.extractPayloads(tt.levelPositions, tokenized)

			// Check payload count
			assert.Len(t, payloads, len(tt.expectedPayloads), "Payload count mismatch")

			// Check each payload
			for varName, expectedCount := range tt.expectedPayloads {
				values, ok := payloads[varName]
				require.True(t, ok, "Payload %q not found", varName)
				assert.Len(t, values, expectedCount, "Payload %q value count mismatch", varName)
			}

			// Check specific values
			if tt.checkContains != nil {
				for varName, expectedValues := range tt.checkContains {
					actualValues, ok := payloads[varName]
					require.True(t, ok, "Payload %q not found", varName)

					for _, expectedVal := range expectedValues {
						assert.Contains(t, actualValues, expectedVal, "Payload %q missing value %q", varName, expectedVal)
					}
				}
			}
		})
	}
}

func TestCalculateCombinations(t *testing.T) {
	tests := []struct {
		name     string
		pattern  *DSLPattern
		expected int
	}{
		{
			name: "static pattern",
			pattern: &DSLPattern{
				Pattern:  "api",
				Payloads: map[string][]string{},
			},
			expected: 1,
		},
		{
			name: "single variable",
			pattern: &DSLPattern{
				Pattern: "api{{p0}}",
				Payloads: map[string][]string{
					"p0": {"-prod", "-staging"},
				},
			},
			expected: 2,
		},
		{
			name: "two variables",
			pattern: &DSLPattern{
				Pattern: "api{{p0}}.{{p1}}",
				Payloads: map[string][]string{
					"p0": {"-prod", "-staging", "-dev"},
					"p1": {"us", "eu"},
				},
			},
			expected: 6, // 3 × 2
		},
		{
			name: "three variables",
			pattern: &DSLPattern{
				Pattern: "{{p0}}{{p1}}.{{p2}}",
				Payloads: map[string][]string{
					"p0": {"api", "web"},
					"p1": {"-1", "-2"},
					"p2": {"dev", "prod", "staging"},
				},
			},
			expected: 12, // 2 × 2 × 3
		},
		{
			name: "optional position with empty string",
			pattern: &DSLPattern{
				Pattern: "api{{p0}}",
				Payloads: map[string][]string{
					"p0": {"-prod", ""},
				},
			},
			expected: 2, // Generates: api-prod, api
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PatternMiner{}
			result := pm.calculateCombinations(tt.pattern)
			assert.Equal(t, tt.expected, result, "Combination count mismatch")
		})
	}
}

func TestIsGoodPattern(t *testing.T) {
	tests := []struct {
		name      string
		pattern   *DSLPattern
		nkeys     int
		threshold float64
		maxRatio  float64
		expected  bool
		reason    string
	}{
		{
			name: "passes absolute threshold",
			pattern: &DSLPattern{
				Pattern: "api{{p0}}",
				Payloads: map[string][]string{
					"p0": {"-prod", "-staging"},
				},
			},
			nkeys:     2,
			threshold: 100,
			maxRatio:  10,
			expected:  true,
			reason:    "2 combinations < 100 threshold",
		},
		{
			name: "passes ratio check",
			pattern: &DSLPattern{
				Pattern: "api{{p0}}.{{p1}}",
				Payloads: map[string][]string{
					"p0": {"-prod", "-staging"},
					"p1": {"dev", "staging"},
				},
			},
			nkeys:     2,
			threshold: 2, // 4 combinations > 2 threshold
			maxRatio:  5, // but ratio 4/2 = 2.0 < 5
			expected:  true,
			reason:    "ratio 2.0 < 5.0 max_ratio",
		},
		{
			name: "fails both checks - too generic",
			pattern: &DSLPattern{
				Pattern: "{{p0}}{{p1}}.{{p2}}",
				Payloads: map[string][]string{
					"p0": {"api", "web", "app"},
					"p1": {"-1", "-2", "-3", "-4"},
					"p2": {"dev", "prod", "staging"},
				},
			},
			nkeys:     3,
			threshold: 10, // 36 combinations > 10 threshold
			maxRatio:  5,  // ratio 36/3 = 12.0 > 5 max_ratio
			expected:  false,
			reason:    "36 combinations exceeds threshold and ratio 12.0 exceeds max_ratio",
		},
		{
			name: "static pattern passes",
			pattern: &DSLPattern{
				Pattern:  "api",
				Payloads: map[string][]string{},
			},
			nkeys:     1,
			threshold: 2, // 1 < 2, passes
			maxRatio:  1,
			expected:  true,
			reason:    "1 combination < 2 threshold",
		},
		{
			name: "no thresholds configured - accepts all",
			pattern: &DSLPattern{
				Pattern: "{{p0}}{{p1}}{{p2}}",
				Payloads: map[string][]string{
					"p0": {"1", "2", "3", "4", "5"},
					"p1": {"a", "b", "c"},
					"p2": {"x", "y"},
				},
			},
			nkeys:     2,
			threshold: 0, // disabled
			maxRatio:  0, // disabled
			expected:  true,
			reason:    "no thresholds configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PatternMiner{
				options: &Options{
					PatternThreshold:    tt.threshold,
					PatternQualityRatio: tt.maxRatio,
				},
			}

			result := pm.isGoodPattern(tt.pattern, tt.nkeys)
			assert.Equal(t, tt.expected, result, tt.reason)
		})
	}
}

func TestGeneratePatternWithQualityCheck(t *testing.T) {
	tests := []struct {
		name       string
		subdomains []string
		threshold  float64
		maxRatio   float64
		expectNil  bool
		reason     string
	}{
		{
			name:       "good pattern - accepted",
			subdomains: []string{"api-prod", "api-staging"},
			threshold:  100,
			maxRatio:   10,
			expectNil:  false,
			reason:     "pattern generates 2 combinations, well within limits",
		},
		{
			name:       "too generic - rejected",
			subdomains: []string{"a", "b", "c"},
			threshold:  1,   // very strict
			maxRatio:   0.5, // very strict ratio
			expectNil:  true,
			reason:     "pattern would be too generic and gets rejected",
		},
		{
			name:       "no thresholds - always accepts",
			subdomains: []string{"api-1", "api-2", "api-3"},
			threshold:  0, // disabled
			maxRatio:   0, // disabled
			expectNil:  false,
			reason:     "no quality checks when thresholds disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domains := make([]string, len(tt.subdomains))
			for i, sub := range tt.subdomains {
				domains[i] = sub + ".example.com"
			}

			pm, err := NewPatternMiner(domains, &Options{
				MinLDist:            2,
				MaxLDist:            10,
				PatternThreshold:    tt.threshold,
				PatternQualityRatio: tt.maxRatio,
			})
			require.NoError(t, err)

			pattern, err := pm.generatePattern(tt.subdomains)
			require.NoError(t, err)

			if tt.expectNil {
				assert.Nil(t, pattern, tt.reason)
			} else {
				assert.NotNil(t, pattern, tt.reason)
			}
		})
	}
}

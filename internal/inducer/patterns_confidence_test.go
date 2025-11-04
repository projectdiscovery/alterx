package inducer

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateConfidence(t *testing.T) {
	tests := []struct {
		name           string
		coverage       int
		ratio          float64
		wantConfidence float64
		tolerance      float64 // Floating point comparison tolerance
	}{
		{
			name:           "example 1 from config_format.md - excellent quality",
			coverage:       450,
			ratio:          1.2,
			wantConfidence: 0.84,
			tolerance:      0.01,
		},
		{
			name:           "example 2 from config_format.md - moderate quality",
			coverage:       230,
			ratio:          2.1,
			wantConfidence: 0.53,
			tolerance:      0.01,
		},
		{
			name:           "example 3 from config_format.md - low quality",
			coverage:       85,
			ratio:          3.2,
			wantConfidence: 0.36,
			tolerance:      0.01,
		},
		{
			name:           "example 4 from config_format.md - low coverage penalty",
			coverage:       10,
			ratio:          1.5,
			wantConfidence: 0.62,
			tolerance:      0.01,
		},
		{
			name:           "example 5 from config_format.md - high coverage boost",
			coverage:       1000,
			ratio:          1.5,
			wantConfidence: 0.72,
			tolerance:      0.01,
		},
		{
			name:           "perfect pattern - ratio 1.0",
			coverage:       100,
			ratio:          1.0,
			wantConfidence: 0.85 + 0.15*math.Log10(100)/3.0, // ≈ 0.95
			tolerance:      0.01,
		},
		{
			name:           "zero coverage",
			coverage:       0,
			ratio:          1.0,
			wantConfidence: 0.0,
			tolerance:      0.01,
		},
		{
			name:           "very high ratio - poor quality",
			coverage:       100,
			ratio:          10.0,
			wantConfidence: 0.185, // 0.85 * 0.1 + 0.15 * 0.67
			tolerance:      0.01,
		},
		{
			name:           "ratio zero - use default",
			coverage:       100,
			ratio:          0.0,
			wantConfidence: 0.85 + 0.15*math.Log10(100)/3.0, // Uses ratio=1.0 default
			tolerance:      0.01,
		},
		{
			name:           "very large coverage",
			coverage:       10000,
			ratio:          1.5,
			wantConfidence: (0.85 * 1.0 / 1.5) + (0.15 * 1.0), // Coverage maxes at 1.0
			tolerance:      0.01,
		},
		{
			name:           "small coverage",
			coverage:       5,
			ratio:          2.0,
			wantConfidence: (0.85 * 0.5) + (0.15 * math.Log10(5) / 3.0),
			tolerance:      0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateConfidence(tt.coverage, tt.ratio)

			// Check that confidence is in valid range
			assert.GreaterOrEqual(t, confidence, 0.0, "confidence should be >= 0")
			assert.LessOrEqual(t, confidence, 1.0, "confidence should be <= 1")

			// Check that confidence matches expected value within tolerance
			assert.InDelta(t, tt.wantConfidence, confidence, tt.tolerance,
				"confidence mismatch for coverage=%d, ratio=%.2f", tt.coverage, tt.ratio)
		})
	}
}

func TestCalculateConfidence_Formula(t *testing.T) {
	// Test the formula components directly

	t.Run("coverage score calculation", func(t *testing.T) {
		tests := []struct {
			coverage      int
			expectedScore float64
		}{
			{coverage: 10, expectedScore: 0.33},   // log10(10)/3 = 1/3
			{coverage: 100, expectedScore: 0.67},  // log10(100)/3 = 2/3
			{coverage: 1000, expectedScore: 1.0},  // log10(1000)/3 = 1, capped at 1.0
			{coverage: 10000, expectedScore: 1.0}, // log10(10000)/3 > 1, capped at 1.0
		}

		for _, tt := range tests {
			score := math.Min(1.0, math.Log10(float64(tt.coverage))/3.0)
			assert.InDelta(t, tt.expectedScore, score, 0.01,
				"coverage score for coverage=%d", tt.coverage)
		}
	})

	t.Run("ratio score calculation", func(t *testing.T) {
		tests := []struct {
			ratio         float64
			expectedScore float64
		}{
			{ratio: 1.0, expectedScore: 1.0},  // 1/1 = 1.0
			{ratio: 1.2, expectedScore: 0.83}, // 1/1.2 ≈ 0.83
			{ratio: 2.0, expectedScore: 0.5},  // 1/2 = 0.5
			{ratio: 3.2, expectedScore: 0.31}, // 1/3.2 ≈ 0.31
			{ratio: 10.0, expectedScore: 0.1}, // 1/10 = 0.1
		}

		for _, tt := range tests {
			score := 1.0 / tt.ratio
			assert.InDelta(t, tt.expectedScore, score, 0.01,
				"ratio score for ratio=%.2f", tt.ratio)
		}
	})

	t.Run("weighted combination", func(t *testing.T) {
		// Test that weights sum to 1.0
		coverage := 100
		ratio := 2.0

		coverageScore := math.Min(1.0, math.Log10(float64(coverage))/3.0)
		ratioScore := 1.0 / ratio

		// Manual calculation
		expectedConfidence := (0.85 * ratioScore) + (0.15 * coverageScore)

		actualConfidence := calculateConfidence(coverage, ratio)

		assert.InDelta(t, expectedConfidence, actualConfidence, 0.0001)
	})
}

func TestPattern_UpdateConfidence(t *testing.T) {
	tests := []struct {
		name           string
		pattern        *Pattern
		wantConfidence float64
		tolerance      float64
	}{
		{
			name: "update after ratio is set",
			pattern: &Pattern{
				Coverage:   450,
				Ratio:      1.2,
				Confidence: 0.0, // Will be updated
			},
			wantConfidence: 0.84,
			tolerance:      0.01,
		},
		{
			name: "high coverage low ratio",
			pattern: &Pattern{
				Coverage:   1000,
				Ratio:      1.5,
				Confidence: 0.0,
			},
			wantConfidence: 0.72,
			tolerance:      0.01,
		},
		{
			name: "low coverage high ratio",
			pattern: &Pattern{
				Coverage:   50,
				Ratio:      5.0,
				Confidence: 0.0,
			},
			wantConfidence: (0.85 * 0.2) + (0.15 * math.Log10(50) / 3.0),
			tolerance:      0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pattern.UpdateConfidence()
			assert.InDelta(t, tt.wantConfidence, tt.pattern.Confidence, tt.tolerance)
		})
	}
}

func TestPattern_UpdateConfidence_NilPattern(t *testing.T) {
	var pattern *Pattern
	// Should not panic
	assert.NotPanics(t, func() {
		pattern.UpdateConfidence()
	})
}

func TestConfidenceInterpretation(t *testing.T) {
	// Test interpretation guidelines from config_format.md

	t.Run("excellent quality - 80%+ valid", func(t *testing.T) {
		// Ratio 1.2, Coverage 450 → 0.84 confidence
		confidence := calculateConfidence(450, 1.2)
		assert.Greater(t, confidence, 0.80, "should be excellent quality")
		assert.InDelta(t, 0.84, confidence, 0.01)
	})

	t.Run("moderate quality - 50-80% valid", func(t *testing.T) {
		// Ratio 2.1, Coverage 230 → 0.53 confidence
		confidence := calculateConfidence(230, 2.1)
		assert.GreaterOrEqual(t, confidence, 0.50)
		assert.Less(t, confidence, 0.80)
		assert.InDelta(t, 0.53, confidence, 0.01)
	})

	t.Run("low quality - <50% valid", func(t *testing.T) {
		// Ratio 3.2, Coverage 85 → 0.36 confidence
		confidence := calculateConfidence(85, 3.2)
		assert.Less(t, confidence, 0.50, "should be low quality")
		assert.InDelta(t, 0.36, confidence, 0.01)
	})
}

func TestConfidenceRobustness(t *testing.T) {
	// Test edge cases and robustness

	t.Run("negative coverage", func(t *testing.T) {
		confidence := calculateConfidence(-10, 1.5)
		assert.Equal(t, 0.0, confidence, "negative coverage should return 0")
	})

	t.Run("negative ratio", func(t *testing.T) {
		confidence := calculateConfidence(100, -1.0)
		// Should handle gracefully (clamped to 0)
		assert.GreaterOrEqual(t, confidence, 0.0)
		assert.LessOrEqual(t, confidence, 1.0)
	})

	t.Run("very small ratio", func(t *testing.T) {
		// Ratio < 1 means pattern is very precise
		confidence := calculateConfidence(100, 0.5)
		// ratio_score = 1/0.5 = 2.0, clamped to 1.0
		// Should give high confidence
		assert.Greater(t, confidence, 0.80)
	})

	t.Run("extremely large ratio", func(t *testing.T) {
		// Pattern generates way too many subdomains
		confidence := calculateConfidence(100, 100.0)
		// ratio_score = 1/100 = 0.01, very low
		assert.Less(t, confidence, 0.20)
	})

	t.Run("coverage of 1", func(t *testing.T) {
		// Single domain - edge case
		confidence := calculateConfidence(1, 1.0)
		// log10(1) = 0, so coverage_score = 0
		// confidence = 0.85 * 1.0 + 0.15 * 0 = 0.85
		assert.InDelta(t, 0.85, confidence, 0.01)
	})
}

func TestConfidenceMonotonicity(t *testing.T) {
	// Test that confidence behaves monotonically as expected

	t.Run("increasing coverage increases confidence", func(t *testing.T) {
		ratio := 1.5
		coverages := []int{10, 50, 100, 500, 1000}

		prevConfidence := 0.0
		for _, coverage := range coverages {
			confidence := calculateConfidence(coverage, ratio)
			assert.Greater(t, confidence, prevConfidence,
				"confidence should increase with coverage")
			prevConfidence = confidence
		}
	})

	t.Run("increasing ratio decreases confidence", func(t *testing.T) {
		coverage := 100
		ratios := []float64{1.0, 1.5, 2.0, 3.0, 5.0, 10.0}

		prevConfidence := 1.0
		for _, ratio := range ratios {
			confidence := calculateConfidence(coverage, ratio)
			assert.Less(t, confidence, prevConfidence,
				"confidence should decrease with ratio")
			prevConfidence = confidence
		}
	})
}

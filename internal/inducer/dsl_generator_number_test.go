package inducer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSLGenerator_NumberRangeCompression(t *testing.T) {
	gen := NewDSLGenerator(nil)

	tests := []struct {
		name               string
		domains            []string
		expectedRange      string // Expected range in format "XX-YY"
		expectedInTemplate bool   // Should template contain {{number}}
	}{
		{
			name: "sequential numbers with leading zeros",
			domains: []string{
				"api-dev-01.example.com",
				"api-dev-02.example.com",
				"api-dev-03.example.com",
			},
			expectedRange:      "00-04", // min(01)-1=0, max(03)+1=04 (with leading zeros)
			expectedInTemplate: true,
		},
		{
			name: "numbers without leading zeros",
			domains: []string{
				"web5.example.com",
				"web6.example.com",
				"web7.example.com",
			},
			expectedRange:      "4-8", // min(5)-1=4, max(7)+1=8
			expectedInTemplate: true,
		},
		{
			name: "large number range",
			domains: []string{
				"db-100.example.com",
				"db-101.example.com",
				"db-102.example.com",
			},
			expectedRange:      "99-103", // min(100)-1=99, max(102)+1=103
			expectedInTemplate: true,
		},
		{
			name: "single digit to double digit transition",
			domains: []string{
				"app08.example.com",
				"app09.example.com",
				"app10.example.com",
			},
			expectedRange:      "07-11", // min(08)-1=07, max(10)+1=11 (keeps leading zero format)
			expectedInTemplate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closure := &Closure{
				Domains: tt.domains,
				Delta:   3,
				Size:    len(tt.domains),
			}

			pattern, err := gen.GeneratePattern(closure)
			require.NoError(t, err)
			require.NotNil(t, pattern)

			t.Logf("Template: %s", pattern.Template)
			t.Logf("Variables: %+v", pattern.Variables)

			// Check template contains {{number}}
			if tt.expectedInTemplate {
				assert.Contains(t, pattern.Template, "{{number}}", "Template should contain {{number}} variable")
			}

			// Find the number variable
			var numberVar *DSLVariable
			for i := range pattern.Variables {
				if pattern.Variables[i].Name == "number" {
					numberVar = &pattern.Variables[i]
					break
				}
			}

			if tt.expectedInTemplate {
				require.NotNil(t, numberVar, "Should have number variable")
				require.NotNil(t, numberVar.NumberRange, "Number variable should have structured range")

				nr := numberVar.NumberRange
				// Expected range is in format "XX-YY", parse it
				parts := strings.Split(tt.expectedRange, "-")
				require.Len(t, parts, 2, "Expected range should have two parts")

				// Parse expected start and end
				expectedStart := 0
				expectedEnd := 0
				for _, ch := range parts[0] {
					if ch >= '0' && ch <= '9' {
						expectedStart = expectedStart*10 + int(ch-'0')
					}
				}
				for _, ch := range parts[1] {
					if ch >= '0' && ch <= '9' {
						expectedEnd = expectedEnd*10 + int(ch-'0')
					}
				}

				assert.Equal(t, expectedStart, nr.Start, "Start should match")
				assert.Equal(t, expectedEnd, nr.End, "End should match")
				assert.Equal(t, 1, nr.Step, "Step should be 1")
				assert.Equal(t, "iterator", nr.Type, "Type should be iterator")
				assert.NotEmpty(t, nr.Format, "Format should be set")

				t.Logf("NumberRange: Start=%d, End=%d, Format=%s, Step=%d, Type=%s",
					nr.Start, nr.End, nr.Format, nr.Step, nr.Type)
			}
		})
	}
}

func TestDSLGenerator_NumberRangeEstimation(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api-dev-01.example.com",
		"api-dev-02.example.com",
		"api-dev-03.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   3,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	// Pattern should be: api-dev-{{number}}.{{root}}
	// Range: 00-04 (5 numbers total)
	// Expected generations: 5
	// Observed: 3
	// Ratio: 5/3 = 1.67

	t.Logf("Pattern: %s", pattern.Template)
	t.Logf("Coverage: %d", pattern.Coverage)
	t.Logf("Ratio: %.2f", pattern.Ratio)
	t.Logf("Confidence: %.2f", pattern.Confidence)

	assert.Equal(t, 3, pattern.Coverage, "Coverage should be 3")
	assert.InDelta(t, 1.67, pattern.Ratio, 0.5, "Ratio should be ~1.67 (5 generations / 3 observed)")
}

func TestDSLGenerator_CompressNumberRange(t *testing.T) {
	gen := NewDSLGenerator(nil)

	tests := []struct {
		name           string
		input          []string
		expectedStart  int
		expectedEnd    int
		expectedFormat string
	}{
		{
			name:           "leading zeros preserved",
			input:          []string{"01", "02", "03"},
			expectedStart:  0,
			expectedEnd:    4,
			expectedFormat: "%02d",
		},
		{
			name:           "no leading zeros",
			input:          []string{"5", "6", "7"},
			expectedStart:  4,
			expectedEnd:    8,
			expectedFormat: "%d",
		},
		{
			name:           "large numbers",
			input:          []string{"100", "101", "102"},
			expectedStart:  99,
			expectedEnd:    103,
			expectedFormat: "%d",
		},
		{
			name:           "single number",
			input:          []string{"42"},
			expectedStart:  41,
			expectedEnd:    43,
			expectedFormat: "%d",
		},
		{
			name:           "mixed digits (takes max digit count)",
			input:          []string{"08", "09", "10"},
			expectedStart:  7,
			expectedEnd:    11,
			expectedFormat: "%02d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.compressNumberRange(tt.input)
			require.NotNil(t, result, "Should return NumberRange")
			assert.Equal(t, tt.expectedStart, result.Start)
			assert.Equal(t, tt.expectedEnd, result.End)
			assert.Equal(t, tt.expectedFormat, result.Format)
			assert.Equal(t, 1, result.Step)
			assert.Equal(t, "iterator", result.Type)
		})
	}
}

func TestDSLGenerator_NumberRangeCount(t *testing.T) {
	gen := NewDSLGenerator(nil)

	tests := []struct {
		name          string
		numberRange   *NumberRange
		expectedCount int
	}{
		{
			name: "range 0-8 with step 1",
			numberRange: &NumberRange{
				Start: 0,
				End:   8,
				Step:  1,
			},
			expectedCount: 9, // 0,1,2,3,4,5,6,7,8
		},
		{
			name: "range 5-12 with step 1",
			numberRange: &NumberRange{
				Start: 5,
				End:   12,
				Step:  1,
			},
			expectedCount: 8, // 5,6,7,8,9,10,11,12
		},
		{
			name: "range 0-10 with step 2",
			numberRange: &NumberRange{
				Start: 0,
				End:   10,
				Step:  2,
			},
			expectedCount: 6, // 0,2,4,6,8,10
		},
		{
			name: "range 100-105 with step 1",
			numberRange: &NumberRange{
				Start: 100,
				End:   105,
				Step:  1,
			},
			expectedCount: 6, // 100,101,102,103,104,105
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := gen.expandNumberRangeCount(tt.numberRange)
			assert.Equal(t, tt.expectedCount, count, "Range count should match")
		})
	}
}

func TestDSLGenerator_MixedVariablesWithNumberRange(t *testing.T) {
	gen := NewDSLGenerator(nil)

	domains := []string{
		"api-dev-01.example.com",
		"api-prod-02.example.com",
		"web-dev-03.example.com",
		"web-prod-01.example.com",
	}

	closure := &Closure{
		Domains: domains,
		Delta:   10,
		Size:    len(domains),
	}

	pattern, err := gen.GeneratePattern(closure)
	require.NoError(t, err)
	require.NotNil(t, pattern)

	t.Logf("Template: %s", pattern.Template)
	t.Logf("Variables: %+v", pattern.Variables)

	// Should have: {{p0}}-{{p0}}-{{number}}.{{root}}
	// p0 (first instance): [api, web] = 2 items
	// p0 (second instance): [dev, prod] = 2 items
	// number: [00-04] = 5 items (range)
	// Total: 2 * 2 * 5 = 20 generations
	// Observed: 4
	// Ratio: 20 / 4 = 5.0

	assert.Contains(t, pattern.Template, "{{number}}")
	assert.Contains(t, pattern.Template, "{{root}}")
	assert.NotContains(t, pattern.Template, "{{suffix}}")
	assert.InDelta(t, 5.0, pattern.Ratio, 1.0, "Ratio should account for range expansion")
}

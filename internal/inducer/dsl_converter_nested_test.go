package inducer

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConvertNestedOptionalGroups tests DSL conversion with nested optional groups
// This tests the fix for the bug where patterns like ((group1)(group2))? would
// produce incorrect templates with stray parentheses
func TestConvertNestedOptionalGroups(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name             string
		regex            string
		expectedTemplate string
		expectedPayloads map[string][]string
	}{
		{
			name:             "nested optional with two alternations",
			regex:            "((api|web)(-dev|-prod))?",
			expectedTemplate: "{{p0}}{{p1}}.{{suffix}}",
			expectedPayloads: map[string][]string{
				"p0": {"api", "web"},
				"p1": {"-dev", "-prod"},
			},
		},
		{
			name:             "nested optional with literal between",
			regex:            "((neo|scheduler|webhook)-dev)?",
			expectedTemplate: "{{p0}}-dev.{{suffix}}",
			expectedPayloads: map[string][]string{
				"p0": {"neo", "scheduler", "webhook"},
			},
		},
		{
			name:             "complex nested optional - projectdiscovery case",
			regex:            "((api|apollo|asn|auth)(-api|-data|-dev|-prod|1))?",
			expectedTemplate: "{{p0}}{{p1}}.{{suffix}}",
			expectedPayloads: map[string][]string{
				"p0": {"api", "apollo", "asn", "auth"},
				"p1": {"-api", "-data", "-dev", "-prod", "1"},
			},
		},
		{
			name:             "simple alternation without nesting",
			regex:            "(api|web|cdn)",
			expectedTemplate: "{{p0}}.{{suffix}}",
			expectedPayloads: map[string][]string{
				"p0": {"api", "cdn", "web"},
			},
		},
		{
			name:             "two alternations without optional",
			regex:            "(api|web)(-dev|-prod)",
			expectedTemplate: "{{p0}}{{p1}}.{{suffix}}",
			expectedPayloads: map[string][]string{
				"p0": {"api", "web"},
				"p1": {"-dev", "-prod"},
			},
		},
		{
			name:             "single optional group",
			regex:            "(api|web)?",
			expectedTemplate: "{{p0}}.{{suffix}}",
			expectedPayloads: map[string][]string{
				"p0": {"api", "web"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Convert(tt.regex)

			// Check for conversion errors
			if result.Error != nil {
				t.Fatalf("unexpected conversion error: %v", result.Error)
			}

			// Verify template matches expected
			assert.Equal(t, tt.expectedTemplate, result.Template,
				"Template mismatch for regex: %s", tt.regex)

			// Verify payloads match expected
			assert.Equal(t, len(tt.expectedPayloads), len(result.Payloads),
				"Payload count mismatch for regex: %s", tt.regex)

			for varName, expectedValues := range tt.expectedPayloads {
				actualValues, exists := result.Payloads[varName]
				assert.True(t, exists,
					"Variable %s not found in payloads for regex: %s", varName, tt.regex)
				assert.Equal(t, expectedValues, actualValues,
					"Payload values mismatch for variable %s in regex: %s", varName, tt.regex)
			}

			// Verify no stray parentheses or question marks
			assert.NotContains(t, result.Template, ")?",
				"Template contains stray )? for regex: %s", tt.regex)
			assert.NotContains(t, result.Template, "(?",
				"Template contains stray (? for regex: %s", tt.regex)
		})
	}
}

// TestCleanupNestedOptionalGroups tests the cleanup function directly
func TestCleanupNestedOptionalGroups(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full optional group",
			input:    "({{p0}}{{p1}})?",
			expected: "{{p0}}{{p1}}",
		},
		{
			name:     "trailing optional marker only",
			input:    "{{p0}}{{p1}})?",
			expected: "{{p0}}{{p1}}",
		},
		{
			name:     "trailing optional with literal",
			input:    "{{p0}}-dev)?",
			expected: "{{p0}}-dev",
		},
		{
			name:     "no optional markers",
			input:    "{{p0}}{{p1}}",
			expected: "{{p0}}{{p1}}",
		},
		{
			name:     "multiple optional groups",
			input:    "({{p0}})?.({{p1}})?",
			expected: "{{p0}}.{{p1}}",
		},
		{
			name:     "mixed - one full, one trailing",
			input:    "({{p0}})?{{p1}})?",
			expected: "{{p0}}{{p1}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.cleanupNestedOptionalGroups(tt.input)
			assert.Equal(t, tt.expected, result,
				"Cleanup mismatch for input: %s", tt.input)
		})
	}
}

// TestPayloadCleaning tests that payloads are cleaned of stray parentheses and duplicates
func TestPayloadCleaning(t *testing.T) {
	converter := NewDSLConverter()

	tests := []struct {
		name                string
		regex               string
		expectCleanPayloads bool
	}{
		{
			name:                "nested groups with leading paren",
			regex:               "((api|apollo|asn)(-dev|-prod))?",
			expectCleanPayloads: true,
		},
		{
			name:                "nested groups with trailing paren",
			regex:               "((neo|scheduler|webhook)-dev)?",
			expectCleanPayloads: true,
		},
		{
			name:                "deeply nested groups",
			regex:               "(((api|web|cdn)))",
			expectCleanPayloads: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Convert(tt.regex)

			if result.Error != nil {
				t.Fatalf("conversion failed: %v", result.Error)
			}

			// Check all payload values are clean
			for varName, values := range result.Payloads {
				for _, val := range values {
					// No stray parentheses
					assert.NotContains(t, val, "(",
						"Payload %s has stray opening paren: %s", varName, val)
					assert.NotContains(t, val, ")",
						"Payload %s has stray closing paren: %s", varName, val)

					// No leading/trailing whitespace
					assert.Equal(t, val, strings.TrimSpace(val),
						"Payload %s has whitespace: '%s'", varName, val)
				}

				// No duplicates within a payload variable
				unique := make(map[string]bool)
				for _, val := range values {
					assert.False(t, unique[val],
						"Duplicate value '%s' in payload %s", val, varName)
					unique[val] = true
				}
			}
		})
	}
}

// TestRemoveDuplicates tests the duplicate removal helper
func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"api", "web"},
			expected: []string{"api", "web"},
		},
		{
			name:     "with duplicates",
			input:    []string{"api", "api", "web"},
			expected: []string{"api", "web"},
		},
		{
			name:     "all duplicates",
			input:    []string{"api", "api", "api"},
			expected: []string{"api"},
		},
		{
			name:     "empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single item",
			input:    []string{"api"},
			expected: []string{"api"},
		},
		{
			name:     "multiple sets of duplicates",
			input:    []string{"api", "api", "web", "web", "cdn"},
			expected: []string{"api", "cdn", "web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: removeDuplicates expects sorted input
			sort.Strings(tt.input)
			result := removeDuplicates(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDSLConversionRegression tests the specific bug cases from projectdiscovery.io
func TestDSLConversionRegression(t *testing.T) {
	converter := NewDSLConverter()

	// These are the actual regex patterns from the projectdiscovery.io analysis
	// that were producing incorrect templates
	buggyPatterns := []struct {
		regex               string
		expectNoStrayParens bool
	}{
		{
			regex:               "((api|apollo|asn|auth|blog|careers|cdn|chaos|clerk|cloud|dast|defcon|dns|docs|feedback|log|login|loki|mcp|neo|nexus|nuclei|outreach|pdtm|policies|security|status|webhook|www)(-api|-data|-dev|-prod|1))?",
			expectNoStrayParens: true,
		},
		{
			regex:               "((api|apollo|asn|auth|blog|careers|cdn|chaos|clerk|cloud|dast|defcon|dns|docs|feedback|log|login|loki|mcp|neo|nexus|nuclei|outreach|pdtm|policies|security|status|www)(-api|-data|-dev|-prod|1))?",
			expectNoStrayParens: true,
		},
		{
			regex:               "((api|apollo|asn|auth|careers|cdn|chaos|cloud|dast|defcon|dns|docs|loki|mcp|neo|nuclei|pdtm|policies|status|www)(-api|-data|-dev|-prod|1))?",
			expectNoStrayParens: true,
		},
		{
			regex:               "((api|careers|chaos|clerk|feedback|mcp|nuclei|outreach|scheduler|security|webhook|www)(-api|-data|-dev))?",
			expectNoStrayParens: true,
		},
		{
			regex:               "((neo|scheduler|webhook)-dev)?",
			expectNoStrayParens: true,
		},
	}

	for i, tc := range buggyPatterns {
		t.Run("regression_"+string(rune('A'+i)), func(t *testing.T) {
			result := converter.Convert(tc.regex)

			if result.Error != nil {
				t.Fatalf("conversion failed: %v", result.Error)
			}

			// The bug was that we got templates like: {{p0}}{{p1}})?.{{suffix}}
			// After fix, we should get: {{p0}}{{p1}}.{{suffix}}
			if tc.expectNoStrayParens {
				assert.NotContains(t, result.Template, ")?",
					"Template contains stray )? for regex: %s\nGot template: %s",
					tc.regex, result.Template)
			}

			// Verify template is well-formed
			assert.Contains(t, result.Template, "{{",
				"Template should contain variable markers")
			assert.Contains(t, result.Template, "}}",
				"Template should contain variable markers")
			assert.Contains(t, result.Template, ".{{suffix}}",
				"Template should end with .{{suffix}}")
		})
	}
}

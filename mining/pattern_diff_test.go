package mining

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPatternDifferences identifies patterns unique to Python vs unique to Go
func TestPatternDifferences(t *testing.T) {
	testCases := []string{"nuclei.sh", "projectdiscovery.io", "tesla.com"}

	for _, dataFile := range testCases {
		t.Run(dataFile, func(t *testing.T) {
			// Load test data
			domains := loadTestData(t, dataFile)
			if len(domains) == 0 {
				t.Skip("No test data available")
			}

			analyzePatternDifferences(t, domains)
		})
	}
}

func analyzePatternDifferences(t *testing.T, domains []string) {

	// Run Python pattern generation
	pythonPatterns := runPythonHierarchicalPatterns(t, domains, 2, 5, 1000, 100, 100)

	// Run Go pattern generation (with same ngram limit as Python test)
	pm, err := NewPatternMiner(domains, &Options{
		MinLDist:            2,
		MaxLDist:            5,
		PatternThreshold:    1000,
		PatternQualityRatio: 100,
		NgramsLimit:         100, // Match Python's ngrams_limit parameter
	})
	require.NoError(t, err)

	err = pm.hierarchicalNgramClustering()
	require.NoError(t, err)

	goPatterns := pm.GetResults()

	// Convert to sets
	pythonSet := make(map[string]struct{})
	for _, p := range pythonPatterns.Patterns {
		pythonSet[p] = struct{}{}
	}

	goSet := make(map[string]struct{})
	for _, p := range goPatterns {
		goSet[p.Pattern] = struct{}{}
	}

	// Find patterns unique to Python
	pythonOnly := []string{}
	for pattern := range pythonSet {
		if _, exists := goSet[pattern]; !exists {
			pythonOnly = append(pythonOnly, pattern)
		}
	}

	// Find patterns unique to Go
	goOnly := []string{}
	for pattern := range goSet {
		if _, exists := pythonSet[pattern]; !exists {
			goOnly = append(goOnly, pattern)
		}
	}

	// Find common patterns
	common := []string{}
	for pattern := range pythonSet {
		if _, exists := goSet[pattern]; exists {
			common = append(common, pattern)
		}
	}

	// Report findings
	t.Logf("Python total: %d patterns", len(pythonSet))
	t.Logf("Go total: %d patterns", len(goSet))
	t.Logf("Common patterns: %d", len(common))
	t.Logf("Unique to Python: %d", len(pythonOnly))
	t.Logf("Unique to Go: %d", len(goOnly))

	if len(pythonOnly) > 0 {
		t.Logf("\nPatterns ONLY in Python (first 20):")
		for i, p := range pythonOnly {
			if i >= 20 {
				t.Logf("  ... and %d more", len(pythonOnly)-20)
				break
			}
			t.Logf("  - %s", p)
		}
	}

	if len(goOnly) > 0 {
		maxShow := 30
		if len(goOnly) < maxShow {
			maxShow = len(goOnly)
		}
		t.Logf("\nPatterns ONLY in Go (first %d with examples):", maxShow)
		count := 0
		for _, goPattern := range goPatterns {
			if _, exists := pythonSet[goPattern.Pattern]; !exists {
				// This pattern is unique to Go
				t.Logf("\n  Pattern: %s", goPattern.Pattern)

				// Show payloads
				if len(goPattern.Payloads) > 0 {
					for key, values := range goPattern.Payloads {
						if len(values) <= 5 {
							t.Logf("    %s: %v", key, values)
						} else {
							t.Logf("    %s: [%s, %s, %s, ... and %d more]",
								key, values[0], values[1], values[2], len(values)-3)
						}
					}
				}

				// Generate and show example combinations
				examples := generateExamples(goPattern, 3)
				if len(examples) > 0 {
					t.Logf("    Examples: %v", examples)
				}

				count++
				if count >= maxShow {
					if len(goOnly) > maxShow {
						t.Logf("\n  ... and %d more extra patterns", len(goOnly)-maxShow)
					}
					break
				}
			}
		}
	}
}

// generateExamples generates example strings from a pattern (up to maxExamples)
func generateExamples(pattern *DSLPattern, maxExamples int) []string {
	if pattern == nil || len(pattern.Payloads) == 0 {
		return []string{pattern.Pattern}
	}

	// Simple case: single payload
	if len(pattern.Payloads) == 1 {
		var key string
		var values []string
		for k, v := range pattern.Payloads {
			key = k
			values = v
			break
		}

		examples := []string{}
		for i := 0; i < len(values) && i < maxExamples; i++ {
			example := pattern.Pattern
			placeholder := "{{" + key + "}}"
			example = replaceFirst(example, placeholder, values[i])
			examples = append(examples, example)
		}
		return examples
	}

	// Multiple payloads - just show first combination
	example := pattern.Pattern
	for key, values := range pattern.Payloads {
		if len(values) > 0 {
			placeholder := "{{" + key + "}}"
			example = replaceFirst(example, placeholder, values[0])
		}
	}
	return []string{example}
}

func replaceFirst(s, old, new string) string {
	// Simple string replace for first occurrence
	idx := 0
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			idx = i
			break
		}
	}
	if idx >= 0 && idx+len(old) <= len(s) {
		return s[:idx] + new + s[idx+len(old):]
	}
	return s
}

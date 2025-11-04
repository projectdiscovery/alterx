package inducer

import (
	"sort"

	"github.com/projectdiscovery/gologger"
)

// FilterSubsumedPatterns removes patterns that are subsumed by broader patterns
// This reduces pattern explosion by eliminating redundant narrow patterns
//
// Algorithm:
// 1. Sort patterns by coverage (descending)
// 2. For each pattern, check if it's subsumed by any already-kept pattern
// 3. A pattern A is subsumed by pattern B if:
//   - B's domains are a superset of A's domains (all of A's domains are in B)
//   - B has higher or equal coverage than A
//
// Example:
//
//	Pattern A: (api|asn) - coverage: 2, domains: [api, asn]
//	Pattern B: (api|asn|cdn) - coverage: 3, domains: [api, asn, cdn]
//	→ Pattern A is subsumed by Pattern B (removed)
func FilterSubsumedPatterns(patterns []*Pattern) []*Pattern {
	if len(patterns) <= 1 {
		return patterns
	}

	gologger.Debug().Msgf("Filtering subsumed patterns from %d patterns", len(patterns))

	// Sort by coverage (descending) so we keep broader patterns first
	sorted := make([]*Pattern, len(patterns))
	copy(sorted, patterns)
	sort.Slice(sorted, func(i, j int) bool {
		// Primary: Coverage (descending)
		if sorted[i].Coverage != sorted[j].Coverage {
			return sorted[i].Coverage > sorted[j].Coverage
		}
		// Secondary: Confidence (descending)
		return sorted[i].Confidence > sorted[j].Confidence
	})

	filtered := []*Pattern{}
	subsumptionCount := 0

	for _, pattern := range sorted {
		isSubsumed := false

		// Check if this pattern is subsumed by any already-kept pattern
		for _, kept := range filtered {
			if patternSubsumes(kept, pattern) {
				isSubsumed = true
				subsumptionCount++
				gologger.Debug().Msgf("Pattern '%s' (coverage: %d) subsumed by '%s' (coverage: %d)",
					pattern.Regex, pattern.Coverage, kept.Regex, kept.Coverage)
				break
			}
		}

		if !isSubsumed {
			filtered = append(filtered, pattern)
		}
	}

	gologger.Verbose().Msgf("Subsumption filtering: %d patterns removed, %d kept", subsumptionCount, len(filtered))
	return filtered
}

// patternSubsumes checks if pattern A subsumes pattern B
// A subsumes B if:
// 1. A's domains are a superset of B's domains (all of B's domains are in A)
// 2. A has higher or equal coverage than B
func patternSubsumes(broader, narrower *Pattern) bool {
	// Can't subsume if broader has lower coverage
	if broader.Coverage < narrower.Coverage {
		return false
	}

	// Can't subsume if patterns are identical
	if broader.Regex == narrower.Regex {
		return false
	}

	// Check if all of narrower's domains are in broader's domains
	// Build a set of broader's domains for O(1) lookup
	broaderDomains := make(map[string]bool, len(broader.Domains))
	for _, domain := range broader.Domains {
		broaderDomains[domain] = true
	}

	// Check if all narrower domains exist in broader
	for _, domain := range narrower.Domains {
		if !broaderDomains[domain] {
			// Found a domain in narrower that's not in broader
			return false
		}
	}

	// All of narrower's domains are in broader → narrower is subsumed
	return true
}

// PatternSimilarity calculates structural similarity between two patterns
// Returns a score from 0.0 (completely different) to 1.0 (identical)
//
// This is used for pattern merging (Solution 3) where we want to combine
// patterns that differ by only a few services.
//
// Example:
//
//	Pattern A: (api|asn)
//	Pattern B: (api|cdn)
//	Similarity: 0.5 (1 common service out of 3 unique services)
func PatternSimilarity(p1, p2 *Pattern) float64 {
	// Build sets of domains
	set1 := make(map[string]bool)
	for _, d := range p1.Domains {
		set1[d] = true
	}

	set2 := make(map[string]bool)
	for _, d := range p2.Domains {
		set2[d] = true
	}

	// Count intersection and union
	intersection := 0
	for d := range set1 {
		if set2[d] {
			intersection++
		}
	}

	union := len(set1)
	for d := range set2 {
		if !set1[d] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	// Jaccard similarity: |A ∩ B| / |A ∪ B|
	return float64(intersection) / float64(union)
}

// FilterSubsumedDSLPatterns removes DSL patterns that are subsumed by broader patterns
// This is the DSL version of FilterSubsumedPatterns
func FilterSubsumedDSLPatterns(patterns []*DSLPattern) []*DSLPattern {
	if len(patterns) <= 1 {
		return patterns
	}

	gologger.Debug().Msgf("Filtering subsumed DSL patterns from %d patterns", len(patterns))

	// Sort by coverage (descending) so we keep broader patterns first
	sorted := make([]*DSLPattern, len(patterns))
	copy(sorted, patterns)
	sort.Slice(sorted, func(i, j int) bool {
		// Primary: Coverage (descending)
		if sorted[i].Coverage != sorted[j].Coverage {
			return sorted[i].Coverage > sorted[j].Coverage
		}
		// Secondary: Confidence (descending)
		return sorted[i].Confidence > sorted[j].Confidence
	})

	filtered := []*DSLPattern{}
	subsumptionCount := 0

	for _, pattern := range sorted {
		isSubsumed := false

		// Check if this pattern is subsumed by any already-kept pattern
		for _, kept := range filtered {
			if dslPatternSubsumes(kept, pattern) {
				isSubsumed = true
				subsumptionCount++
				gologger.Debug().Msgf("DSL Pattern '%s' (coverage: %d) subsumed by '%s' (coverage: %d)",
					pattern.Template, pattern.Coverage, kept.Template, kept.Coverage)
				break
			}
		}

		if !isSubsumed {
			filtered = append(filtered, pattern)
		}
	}

	gologger.Verbose().Msgf("Subsumption filtering: %d patterns removed, %d kept", subsumptionCount, len(filtered))
	return filtered
}

// dslPatternSubsumes checks if DSL pattern A subsumes DSL pattern B
func dslPatternSubsumes(broader, narrower *DSLPattern) bool {
	// Can't subsume if broader has lower coverage
	if broader.Coverage < narrower.Coverage {
		return false
	}

	// Can't subsume if patterns are identical
	if broader.Template == narrower.Template {
		return false
	}

	// Check if all of narrower's domains are in broader's domains
	broaderDomains := make(map[string]bool, len(broader.Domains))
	for _, domain := range broader.Domains {
		broaderDomains[domain] = true
	}

	// Check if all narrower domains exist in broader
	for _, domain := range narrower.Domains {
		if !broaderDomains[domain] {
			return false
		}
	}

	return true
}

// FilterLowQualityTokens removes patterns with low-quality token characteristics
// Uses GRADUATED thresholds: stricter for low-coverage, more lenient for high-coverage patterns
//
// Rejection criteria (ADAPTIVE based on dataset size):
// 1. Graduated ratio threshold based on coverage
//   - High coverage (50+ domains): Accept ratio ≤ 100
//   - Medium coverage (10-50): Accept ratio ≤ 60
//   - Low coverage (< 10): Accept ratio ≤ 40
//
// 2. Adaptive confidence threshold (dataset-size dependent)
//   - Small datasets (<50): 0.30 minimum (prioritize quality)
//   - Mid datasets (50-200): 0.15 minimum (balanced)
//   - Large datasets (200+): 0.10 minimum (prioritize discovery)
//
// 3. Single-char tokens: ALLOWED (many valid cases like "d" for d1, d2, d3)
func FilterLowQualityTokens(patterns []*DSLPattern, minConfidence float64, maxRatio float64, datasetSize int) []*DSLPattern {
	filtered := []*DSLPattern{}

	for _, pattern := range patterns {
		// GRADUATED RATIO THRESHOLD based on coverage (AGGRESSIVELY RELAXED)
		// Philosophy: Prioritize COVERAGE over PRECISION to hit 50-70% target
		// Trade-off: Accept significant noise to discover patterns
		var effectiveMaxRatio float64
		if pattern.Coverage >= 50 {
			effectiveMaxRatio = 100.0 // High coverage → extremely lenient
		} else if pattern.Coverage >= 10 {
			effectiveMaxRatio = 60.0 // Medium coverage → very lenient (was 30.0)
		} else {
			effectiveMaxRatio = 40.0 // Low coverage → lenient (was 20.0)
		}

		// Check 1: Graduated ratio threshold
		if pattern.Ratio > effectiveMaxRatio {
			gologger.Debug().Msgf("Rejected pattern (ratio %.2f > %.2f, coverage=%d): %s",
				pattern.Ratio, effectiveMaxRatio, pattern.Coverage, pattern.Template)
			continue
		}

		// Check 2: ADAPTIVE confidence threshold based on dataset size
		// Small datasets: prioritize quality (30% minimum)
		// Mid datasets: balanced approach (15% minimum)
		// Large datasets: prioritize discovery (10% minimum)
		var adaptiveMinConfidence float64
		if datasetSize < 50 {
			adaptiveMinConfidence = 0.30 // Small: high quality
		} else if datasetSize < 200 {
			adaptiveMinConfidence = 0.15 // Mid: balanced
		} else {
			adaptiveMinConfidence = 0.10 // Large: discovery-focused
		}

		if pattern.Confidence < adaptiveMinConfidence {
			gologger.Debug().Msgf("Rejected pattern (confidence %.2f < %.2f, dataset=%d): %s",
				pattern.Confidence, adaptiveMinConfidence, datasetSize, pattern.Template)
			continue
		}

		// Check 3: DISABLED - single-char token check removed
		// Rationale: Many valid single-char tokens (e.g., "d" for d1, d2, d3 environments)
		// Users can post-filter if needed - prioritize discovery over precision

		// Pattern passes all checks
		filtered = append(filtered, pattern)
	}

	gologger.Verbose().Msgf("Token quality filtering: %d → %d patterns (removed %d low-quality)",
		len(patterns), len(filtered), len(patterns)-len(filtered))
	return filtered
}

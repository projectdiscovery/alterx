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
//    - B's domains are a superset of A's domains (all of A's domains are in B)
//    - B has higher or equal coverage than A
//
// Example:
//   Pattern A: (api|asn) - coverage: 2, domains: [api, asn]
//   Pattern B: (api|asn|cdn) - coverage: 3, domains: [api, asn, cdn]
//   → Pattern A is subsumed by Pattern B (removed)
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
//   Pattern A: (api|asn)
//   Pattern B: (api|cdn)
//   Similarity: 0.5 (1 common service out of 3 unique services)
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

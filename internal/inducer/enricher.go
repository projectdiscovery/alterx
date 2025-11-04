package inducer

import (
	"math/rand"
	"strings"
)

// Enricher adds optional variable support to DSL patterns
type Enricher struct {
	EnrichmentRate float64
	SemanticVars   []string
}

// NewEnricher creates a new enricher
func NewEnricher(enrichmentRate float64) *Enricher {
	return &Enricher{
		EnrichmentRate: enrichmentRate,
		SemanticVars:   []string{"env", "region", "service", "stage", "tier"},
	}
}

// EnrichPatterns adds optional variable support
func (e *Enricher) EnrichPatterns(patterns []*DSLPattern) []*DSLPattern {
	if len(patterns) == 0 {
		return patterns
	}

	enriched := make([]*DSLPattern, len(patterns))
	for i, pattern := range patterns {
		enriched[i] = pattern.Copy()
		e.enrichPattern(enriched[i])
	}

	return enriched
}

func (e *Enricher) enrichPattern(pattern *DSLPattern) {
	if pattern == nil {
		return
	}

	for i := range pattern.Variables {
		variable := &pattern.Variables[i]

		// Heuristic 1: Numbers ALWAYS optional
		if variable.NumberRange != nil {
			variable.Payloads = []string{""}
			continue
		}

		// Heuristic 2: Semantic variables optional based on rate
		if e.isSemanticVariable(variable.Name) {
			if rand.Float64() < e.EnrichmentRate {
				variable.Payloads = append([]string{""}, variable.Payloads...)
			}
			continue
		}

		// Heuristic 3: Single-value payloads (50% of rate)
		if len(variable.Payloads) == 1 {
			if rand.Float64() < (e.EnrichmentRate * 0.5) {
				variable.Payloads = append([]string{""}, variable.Payloads...)
			}
			continue
		}

		// Heuristic 4: Common affixes
		if e.hasCommonAffixes(variable.Payloads) {
			if rand.Float64() < e.EnrichmentRate {
				variable.Payloads = append([]string{""}, variable.Payloads...)
			}
		}
	}
}

func (e *Enricher) isSemanticVariable(name string) bool {
	for _, s := range e.SemanticVars {
		if name == s {
			return true
		}
	}
	return false
}

func (e *Enricher) hasCommonAffixes(payloads []string) bool {
	commonAffixes := []string{
		"v1", "v2", "v3", "v4", "v5",
		"old", "new", "temp", "test",
		"beta", "alpha", "canary",
		"public", "private",
		"internal", "external",
	}

	matches := 0
	for _, payload := range payloads {
		for _, affix := range commonAffixes {
			if strings.EqualFold(payload, affix) {
				matches++
				break
			}
		}
	}

	return matches > len(payloads)/2
}

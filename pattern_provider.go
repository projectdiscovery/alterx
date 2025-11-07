package alterx

import (
	"fmt"

	"github.com/projectdiscovery/alterx/mining"
	"github.com/projectdiscovery/gologger"
)

// PatternProvider defines the interface for pattern generation strategies.
// Implementations provide patterns and payloads that can be used by the Mutator
// to generate domain permutations.
type PatternProvider interface {
	// GetPatterns returns the patterns and their associated payloads.
	// Returns:
	//   - patterns: slice of pattern strings in DSL format (e.g., "api-{{p0}}.{{root}}")
	//   - payloads: map of payload variables to their values (e.g., {"p0": ["prod", "dev"]})
	//   - error: any error encountered during pattern generation
	GetPatterns() (patterns []string, payloads map[string][]string, err error)
}

// ManualPatternProvider provides user-specified patterns and payloads.
// This is the default mode where users explicitly provide patterns and wordlists.
type ManualPatternProvider struct {
	patterns []string
	payloads map[string][]string
}

// NewManualPatternProvider creates a new manual pattern provider.
func NewManualPatternProvider(patterns []string, payloads map[string][]string) *ManualPatternProvider {
	return &ManualPatternProvider{
		patterns: patterns,
		payloads: payloads,
	}
}

// GetPatterns returns the manually specified patterns and payloads.
func (m *ManualPatternProvider) GetPatterns() ([]string, map[string][]string, error) {
	if len(m.patterns) == 0 {
		return nil, nil, fmt.Errorf("no patterns provided")
	}
	return m.patterns, m.payloads, nil
}

// MinedPatternProvider discovers patterns from input domains using pattern mining algorithms.
// This mode automatically generates patterns by analyzing the structure of provided domains.
type MinedPatternProvider struct {
	domains       []string
	miningOptions *mining.Options
}

// NewMinedPatternProvider creates a new mined pattern provider.
func NewMinedPatternProvider(domains []string, opts *mining.Options) *MinedPatternProvider {
	return &MinedPatternProvider{
		domains:       domains,
		miningOptions: opts,
	}
}

// GetPatterns mines patterns from the input domains and returns them in mutator format.
func (m *MinedPatternProvider) GetPatterns() ([]string, map[string][]string, error) {
	gologger.Info().Msgf("Mining patterns from %d domains...", len(m.domains))

	// Create pattern miner
	pm, err := mining.NewPatternMiner(m.domains, m.miningOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pattern miner: %w", err)
	}

	// Execute pattern mining (runs both Levenshtein and Hierarchical clustering)
	if err := pm.Execute(); err != nil {
		return nil, nil, fmt.Errorf("pattern mining failed: %w", err)
	}

	// Get mined DSL patterns
	dslPatterns := pm.GetResults()
	if len(dslPatterns) == 0 {
		return nil, nil, fmt.Errorf("no patterns discovered from input domains")
	}

	gologger.Info().Msgf("Discovered %d patterns", len(dslPatterns))

	// Convert DSLPatterns to mutator format
	patterns := make([]string, 0, len(dslPatterns))
	allPayloads := make(map[string][]string)

	for _, dsl := range dslPatterns {
		// Add pattern (need to append .{{root}} to match mutator expectations)
		patterns = append(patterns, dsl.Pattern+".{{root}}")

		// Merge payloads from this pattern
		for key, values := range dsl.Payloads {
			if existing, ok := allPayloads[key]; ok {
				// Merge and deduplicate
				valueSet := make(map[string]struct{})
				for _, v := range existing {
					valueSet[v] = struct{}{}
				}
				for _, v := range values {
					valueSet[v] = struct{}{}
				}
				// Convert back to slice
				merged := make([]string, 0, len(valueSet))
				for v := range valueSet {
					merged = append(merged, v)
				}
				allPayloads[key] = merged
			} else {
				// First time seeing this key, just copy values
				allPayloads[key] = append([]string{}, values...)
			}
		}
	}

	gologger.Info().Msgf("Generated %d unique payload keys", len(allPayloads))

	return patterns, allPayloads, nil
}

package alterx

import (
	"fmt"
	"strings"

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

	// Convert DSLPatterns to mutator format with UNIQUE payload keys per pattern
	// This prevents cross-contamination when multiple patterns use same key names (e.g., "p0")
	//
	// BEFORE (BUGGY): Pattern1="api{{p0}}", Pattern2="web{{p0}}"
	//                 Merged payloads = {"p0": ["-prod", "-staging", "-dev", "-test"]}
	//                 Result: BOTH patterns use ALL 4 values → 2x explosion
	//
	// AFTER (FIXED): Pattern1="api{{p0_0}}", Pattern2="web{{p0_1}}"
	//                Payloads = {"p0_0": ["-prod", "-staging"], "p0_1": ["-dev", "-test"]}
	//                Result: Each pattern uses only its own values → correct output
	patterns := make([]string, 0, len(dslPatterns))
	allPayloads := make(map[string][]string)

	for patternIdx, dsl := range dslPatterns {
		// Create unique payload keys by appending pattern index
		// Original: "{{p0}}{{p1}}" → Unique: "{{p0_0}}{{p1_0}}"
		uniquePattern := dsl.Pattern
		for key := range dsl.Payloads {
			uniqueKey := fmt.Sprintf("%s_%d", key, patternIdx)
			// Replace old key with unique key in pattern
			uniquePattern = strings.ReplaceAll(uniquePattern, "{{"+key+"}}", "{{"+uniqueKey+"}}")
			// Store payloads with unique key
			allPayloads[uniqueKey] = append([]string{}, dsl.Payloads[key]...)
		}

		// Add pattern with unique keys (append .{{root}} to match mutator expectations)
		patterns = append(patterns, uniquePattern+".{{root}}")
	}

	gologger.Info().Msgf("Generated %d unique payload keys", len(allPayloads))

	return patterns, allPayloads, nil
}

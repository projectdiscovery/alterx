package alterx

import (
	"fmt"
	"strings"

	"github.com/projectdiscovery/alterx/internal/inducer"
	"github.com/projectdiscovery/gologger"
	"golang.org/x/net/publicsuffix"
)

// LearnedPattern represents a single learned pattern with all metadata
// This structure matches the YAML specification in permutations.yaml
type LearnedPattern struct {
	ID         string                 `yaml:"id"`                 // Unique identifier (e.g., "pattern_001")
	Template   string                 `yaml:"template"`           // DSL template (e.g., "{{p0}}-{{p1}}.{{suffix}}")
	Regex      string                 `yaml:"regex,omitempty"`    // Original regex pattern (for analysis/debugging)
	Coverage   int                    `yaml:"coverage"`           // Number of input domains matched
	Ratio      float64                `yaml:"ratio"`              // Possible generations / observed count
	Confidence float64                `yaml:"confidence"`         // Quality score (0.0-1.0)
	Payloads   map[string]interface{} `yaml:"payloads,omitempty"` // Inline payloads: []string for literals, NumberRange for numbers
	Examples   []string               `yaml:"examples,omitempty"` // Example domains (optional)
}

// PatternInducer discovers patterns from passive subdomain enumeration results
type PatternInducer struct {
	passiveDomains []string              // Existing discovered subdomains to learn patterns from
	minFrequency   int                   // Minimum pattern frequency to be considered valid
	orchestrator   *inducer.Orchestrator // Internal orchestrator for stats access
}

// NewPatternInducer creates a new pattern inducer
// passiveDomains: the passive subdomain enumeration results to learn patterns from
// minFrequency: minimum times a pattern must occur to be considered (default: 2)
func NewPatternInducer(passiveDomains []string, minFrequency int) *PatternInducer {
	if minFrequency < 2 {
		minFrequency = 2
	}
	return &PatternInducer{
		passiveDomains: passiveDomains,
		minFrequency:   minFrequency,
	}
}

// InferPatterns analyzes passive subdomain results and infers patterns
// Returns a list of structured LearnedPattern objects with full metadata
//
// Algorithm:
// 1. Pass full domains to orchestrator (preserves root domain info)
// 2. Orchestrator groups domains by level count (structural depth)
// 3. Run pattern induction on each level group independently
// 4. DSL generator creates patterns with {{root}} placeholder
// 5. Filter by quality and confidence thresholds
// 6. Return high-confidence patterns with all metadata
func (pi *PatternInducer) InferPatterns() ([]*LearnedPattern, error) {
	if len(pi.passiveDomains) == 0 {
		gologger.Warning().Msg("No passive domains provided for pattern induction")
		return []*LearnedPattern{}, nil
	}

	// STEP 1: Filter out wildcard domains and root-only domains (BEFORE mode detection)
	validDomains := filterValidDomains(pi.passiveDomains)
	if len(validDomains) == 0 {
		gologger.Warning().Msg("No valid domains after filtering")
		return []*LearnedPattern{}, nil
	}

	// STEP 2: Create orchestrator with mode-based configuration (auto-detects THOROUGH/BALANCED/FAST)
	pi.orchestrator = inducer.NewOrchestrator(len(validDomains))

	// Load token dictionary from config for semantic classification
	tokenDict := DefaultConfig.GetTokenDictionary()
	if tokenDict != nil {
		pi.orchestrator.SetTokenDictionary(tokenDict)
	}

	// Learn patterns from FULL domains (orchestrator handles all steps with verbose logging)
	patterns, err := pi.orchestrator.LearnPatterns(validDomains)
	if err != nil {
		gologger.Error().Msgf("Pattern induction failed: %v", err)
		return []*LearnedPattern{}, err
	}

	// Convert DSLPattern objects to LearnedPattern format
	learnedPatterns := make([]*LearnedPattern, 0, len(patterns))

	for idx, pattern := range patterns {
		// Build payloads map from DSLVariable array
		payloads := make(map[string]interface{})
		for _, variable := range pattern.Variables {
			// Check if enrichment added optional marker ("") to Payloads
			hasOptionalMarker := len(variable.Payloads) > 0 && variable.Payloads[0] == ""

			if variable.NumberRange != nil && !hasOptionalMarker {
				// Number variable without enrichment - use NumberRange directly
				payloads[variable.Name] = variable.NumberRange
			} else {
				// Word/literal variables OR enriched number variables use string arrays
				payloads[variable.Name] = variable.Payloads
			}
		}

		// Create structured LearnedPattern object
		learnedPattern := &LearnedPattern{
			ID:         fmt.Sprintf("pattern_%03d", idx+1),
			Template:   pattern.Template,
			Coverage:   pattern.Coverage,
			Ratio:      pattern.Ratio,
			Confidence: pattern.Confidence,
			Payloads:   payloads,
		}

		// Add example domains (first 3 from the pattern's domain list)
		if len(pattern.Domains) > 0 {
			exampleCount := 3
			if len(pattern.Domains) < exampleCount {
				exampleCount = len(pattern.Domains)
			}
			learnedPattern.Examples = pattern.Domains[:exampleCount]
		}

		learnedPatterns = append(learnedPatterns, learnedPattern)
	}

	if len(learnedPatterns) == 0 {
		gologger.Warning().Msg("No valid DSL patterns generated")
		return []*LearnedPattern{}, nil
	}

	gologger.Verbose().Msgf("Completed: %d patterns generated", len(learnedPatterns))
	return learnedPatterns, nil
}

// filterValidDomains filters out invalid domains while preserving full domain names
// Removes wildcards, root-only domains, and domains with invalid TLDs
func filterValidDomains(domains []string) []string {
	validDomains := make([]string, 0, len(domains))

	for _, domain := range domains {
		// Skip wildcard domains
		if strings.HasPrefix(domain, "*.") {
			continue
		}

		// Extract root domain (eTLD+1) to validate structure
		etld, err := publicsuffix.EffectiveTLDPlusOne(domain)
		if err != nil {
			continue
		}

		// Skip root-only domains (no subdomain)
		if domain == etld {
			continue
		}

		// Keep FULL domain (with root) for level-based grouping
		validDomains = append(validDomains, domain)
	}

	return validDomains
}

// GetStats returns orchestrator statistics after pattern induction
// Must be called after InferPatterns() to access stats
// Returns nil if InferPatterns() hasn't been called yet
func (pi *PatternInducer) GetStats() map[string]interface{} {
	if pi.orchestrator == nil {
		return nil
	}

	stats := pi.orchestrator.GetStats()
	return map[string]interface{}{
		"InputDomains":      stats.InputDomains,
		"Mode":              stats.Mode,
		"FilteredDomains":   stats.FilteredDomains,
		"LevelGroups":       stats.LevelGroups,
		"Strategy1Patterns": stats.Strategy1Patterns,
		"Strategy2Patterns": stats.Strategy2Patterns,
		"Strategy3Patterns": stats.Strategy3Patterns,
		"RawPatterns":       stats.RawPatterns,
		"AfterDedup":        stats.AfterDedup,
		"AfterAP":           stats.AfterAP,
		"FinalPatterns":     stats.FinalPatterns,
	}
}

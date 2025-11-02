package alterx

import (
	"github.com/projectdiscovery/alterx/inducer"
	"github.com/projectdiscovery/gologger"
	"golang.org/x/net/publicsuffix"
)

// PatternInducer discovers patterns from passive subdomain enumeration results
type PatternInducer struct {
	inputDomains   []string // Target domains to generate permutations for
	passiveDomains []string // Existing discovered subdomains to learn patterns from
	minFrequency   int      // Minimum pattern frequency to be considered valid
}

// NewPatternInducer creates a new pattern inducer
// inputDomains: the target domains for which we want to generate permutations
// passiveDomains: the passive subdomain enumeration results to learn patterns from
// minFrequency: minimum times a pattern must occur to be considered (default: 2)
func NewPatternInducer(inputDomains []string, passiveDomains []string, minFrequency int) *PatternInducer {
	if minFrequency < 2 {
		minFrequency = 2
	}
	return &PatternInducer{
		inputDomains:   inputDomains,
		passiveDomains: passiveDomains,
		minFrequency:   minFrequency,
	}
}

// InferPatterns analyzes passive subdomain results and infers patterns
// Returns a list of pattern templates compatible with AlterX DSL
// Example patterns: "{{word}}-{{word}}.{{suffix}}", "{{word}}.{{word}}.{{suffix}}"
//
// Algorithm:
// 1. Extract root domain from input
// 2. Run hierarchical pattern induction on passive domains
// 3. Convert learned regex patterns to AlterX DSL format
// 4. Filter by quality and confidence thresholds
// 5. Return high-confidence patterns for the target organization
func (pi *PatternInducer) InferPatterns() ([]string, error) {
	gologger.Info().Msgf("Starting pattern induction with %d passive domains", len(pi.passiveDomains))

	if len(pi.passiveDomains) == 0 {
		gologger.Warning().Msg("No passive domains provided for pattern induction")
		return []string{}, nil
	}

	// Extract root domain from first input domain
	rootDomain := ""
	if len(pi.inputDomains) > 0 {
		etld, err := publicsuffix.EffectiveTLDPlusOne(pi.inputDomains[0])
		if err == nil {
			rootDomain = etld
		}
	}

	// If we couldn't get root from input, try from passive domains
	if rootDomain == "" && len(pi.passiveDomains) > 0 {
		etld, err := publicsuffix.EffectiveTLDPlusOne(pi.passiveDomains[0])
		if err == nil {
			rootDomain = etld
		}
	}

	gologger.Verbose().Msgf("Using root domain: %s", rootDomain)

	// Create orchestrator with default config
	config := inducer.DefaultOrchestratorConfig(rootDomain)

	// Use minFrequency as quality threshold
	config.QualityConfig.MinCoverage = pi.minFrequency

	orchestrator := inducer.NewOrchestrator(config)

	// Learn patterns from passive domains
	patterns, err := orchestrator.LearnPatterns(pi.passiveDomains)
	if err != nil {
		gologger.Error().Msgf("Pattern induction failed: %v", err)
		return []string{}, err
	}

	gologger.Info().Msgf("Learned %d high-quality patterns", len(patterns))

	// Convert learned regex patterns to AlterX DSL format
	converter := inducer.NewDSLConverter(rootDomain)
	dslPatterns := converter.ConvertPatternsToDSL(patterns)

	gologger.Info().Msgf("Converted to %d DSL patterns", len(dslPatterns))

	// Log patterns for debugging
	for i, pattern := range dslPatterns {
		gologger.Verbose().Msgf("Pattern %d: %s", i+1, pattern)
	}

	return dslPatterns, nil
}

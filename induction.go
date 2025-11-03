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
	ID         string                 `yaml:"id"`                   // Unique identifier (e.g., "pattern_001")
	Template   string                 `yaml:"template"`             // DSL template (e.g., "{{p0}}-{{p1}}.{{suffix}}")
	Regex      string                 `yaml:"regex,omitempty"`      // Original regex pattern (for analysis/debugging)
	Coverage   int                    `yaml:"coverage"`             // Number of input domains matched
	Ratio      float64                `yaml:"ratio"`                // Possible generations / observed count
	Confidence float64                `yaml:"confidence"`           // Quality score (0.0-1.0)
	Payloads   map[string]interface{} `yaml:"payloads,omitempty"`   // Inline payloads: []string for literals, NumberRange for numbers
	Examples   []string               `yaml:"examples,omitempty"`   // Example domains (optional)
}

// PatternInducer discovers patterns from passive subdomain enumeration results
type PatternInducer struct {
	passiveDomains []string // Existing discovered subdomains to learn patterns from
	minFrequency   int      // Minimum pattern frequency to be considered valid
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
	gologger.Verbose().Msgf("Starting pattern induction with %d passive domains", len(pi.passiveDomains))

	if len(pi.passiveDomains) == 0 {
		gologger.Warning().Msg("No passive domains provided for pattern induction")
		return []*LearnedPattern{}, nil
	}

	// Filter out wildcard domains and root-only domains
	validDomains := filterValidDomains(pi.passiveDomains)
	if len(validDomains) == 0 {
		gologger.Warning().Msg("No valid domains found after filtering")
		return []*LearnedPattern{}, nil
	}

	// Create orchestrator with default config
	// MinCoverage will be calculated dynamically in LearnPatterns() based on input size
	config := inducer.DefaultOrchestratorConfig()

	orchestrator := inducer.NewOrchestrator(config)

	// Load token dictionary from config for semantic classification
	tokenDict := DefaultConfig.GetTokenDictionary()
	if tokenDict != nil {
		gologger.Verbose().Msgf("Using token dictionary for semantic classification (env: %d, region: %d, service: %d tokens)",
			len(tokenDict.Env), len(tokenDict.Region), len(tokenDict.Service))
		orchestrator.SetTokenDictionary(tokenDict)
	} else {
		gologger.Verbose().Msg("No token dictionary configured, using type-based classification")
	}

	// Learn patterns from FULL domains (preserves root domain for level-based grouping)
	// The orchestrator will group domains by level count and generate patterns with {{root}}
	patterns, err := orchestrator.LearnPatterns(validDomains)
	if err != nil {
		gologger.Error().Msgf("Pattern induction failed: %v", err)
		return []*LearnedPattern{}, err
	}

	gologger.Verbose().Msgf("Learned %d high-quality DSL patterns", len(patterns))

	// Convert DSLPattern objects to LearnedPattern format
	learnedPatterns := make([]*LearnedPattern, 0, len(patterns))

	for idx, pattern := range patterns {
		// DSLPattern already has the template - just convert to LearnedPattern format
		// Build payloads map from DSLVariable array
		payloads := make(map[string]interface{})
		for _, variable := range pattern.Variables {
			// Number variables use structured NumberRange
			if variable.NumberRange != nil {
				payloads[variable.Name] = variable.NumberRange
				gologger.Verbose().Msgf("Variable %s: NumberRange{Start: %d, End: %d, Format: %s, Step: %d, Type: %s}",
					variable.Name, variable.NumberRange.Start, variable.NumberRange.End,
					variable.NumberRange.Format, variable.NumberRange.Step, variable.NumberRange.Type)
			} else {
				// Word/literal variables use string arrays
				payloads[variable.Name] = variable.Payloads
				gologger.Verbose().Msgf("Variable %s: %v (type: %s)", variable.Name, variable.Payloads, variable.Type)
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

		// Log pattern details for debugging
		gologger.Verbose().Msgf("Pattern %d: %s (coverage: %d, ratio: %.2f, confidence: %.2f)",
			idx+1, learnedPattern.Template, learnedPattern.Coverage, learnedPattern.Ratio, learnedPattern.Confidence)

		// Log payloads if present
		if len(learnedPattern.Payloads) > 0 {
			for varName, values := range learnedPattern.Payloads {
				gologger.Debug().Msgf("  {{%s}}: %v", varName, values)
			}
		}

		learnedPatterns = append(learnedPatterns, learnedPattern)
	}

	if len(learnedPatterns) == 0 {
		gologger.Warning().Msg("No valid DSL patterns generated")
		return []*LearnedPattern{}, nil
	}

	gologger.Verbose().Msgf("Successfully generated %d DSL patterns", len(learnedPatterns))
	return learnedPatterns, nil
}

// filterValidDomains filters out invalid domains while preserving full domain names
// Removes:
// - Wildcard domains (*.example.com)
// - Root-only domains (example.com with no subdomain)
// - Domains with invalid TLDs
//
// Returns FULL domains (e.g., "api-dev.example.com", "staging.prod.example.com")
// The orchestrator needs full domains to extract root and count levels
func filterValidDomains(domains []string) []string {
	validDomains := make([]string, 0, len(domains))

	for _, domain := range domains {
		// Skip wildcard domains
		if strings.HasPrefix(domain, "*.") {
			gologger.Verbose().Msgf("Skipping wildcard domain: %s", domain)
			continue
		}

		// Extract root domain (eTLD+1) to validate structure
		etld, err := publicsuffix.EffectiveTLDPlusOne(domain)
		if err != nil {
			gologger.Verbose().Msgf("Skipping domain with invalid TLD: %s", domain)
			continue
		}

		// Skip root-only domains (no subdomain)
		if domain == etld {
			gologger.Verbose().Msgf("Skipping root-only domain: %s", domain)
			continue
		}

		// Keep FULL domain (with root) for level-based grouping
		validDomains = append(validDomains, domain)
	}

	return validDomains
}

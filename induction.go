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
	ID         string              `yaml:"id"`                   // Unique identifier (e.g., "pattern_001")
	Template   string              `yaml:"template"`             // DSL template (e.g., "{{p0}}-{{p1}}.{{suffix}}")
	Regex      string              `yaml:"regex,omitempty"`      // Original regex pattern (for analysis/debugging)
	Coverage   int                 `yaml:"coverage"`             // Number of input domains matched
	Ratio      float64             `yaml:"ratio"`                // Possible generations / observed count
	Confidence float64             `yaml:"confidence"`           // Quality score (0.0-1.0)
	Payloads   map[string][]string `yaml:"payloads,omitempty"`   // Inline payloads for positional variables
	Examples   []string            `yaml:"examples,omitempty"`   // Example domains (optional)
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
// 1. Extract subdomains from full domains (filter root-only domains)
// 2. Run hierarchical pattern induction on subdomain data (root-agnostic)
// 3. Convert learned regex patterns to AlterX DSL format
// 4. Filter by quality and confidence thresholds
// 5. Return high-confidence patterns with all metadata
func (pi *PatternInducer) InferPatterns() ([]*LearnedPattern, error) {
	gologger.Info().Msgf("Starting pattern induction with %d passive domains", len(pi.passiveDomains))

	if len(pi.passiveDomains) == 0 {
		gologger.Warning().Msg("No passive domains provided for pattern induction")
		return []*LearnedPattern{}, nil
	}

	// Extract subdomains from full domains (remove root domain parts)
	subdomains := extractSubdomains(pi.passiveDomains)
	if len(subdomains) == 0 {
		gologger.Warning().Msg("No valid subdomains found after filtering")
		return []*LearnedPattern{}, nil
	}

	// Create orchestrator with default config
	// MinCoverage will be calculated dynamically in LearnPatterns() based on input size
	config := inducer.DefaultOrchestratorConfig()

	orchestrator := inducer.NewOrchestrator(config)

	// Learn patterns from subdomain data (root-agnostic)
	patterns, err := orchestrator.LearnPatterns(subdomains)
	if err != nil {
		gologger.Error().Msgf("Pattern induction failed: %v", err)
		return []*LearnedPattern{}, err
	}

	gologger.Info().Msgf("Learned %d high-quality patterns", len(patterns))

	// Convert learned regex patterns to AlterX DSL template format
	converter := inducer.NewDSLConverter()
	learnedPatterns := make([]*LearnedPattern, 0, len(patterns))

	for idx, pattern := range patterns {
		// Convert regex to DSL template
		result := converter.Convert(pattern.Regex)
		if result.Error != nil {
			gologger.Warning().Msgf("Failed to convert pattern '%s': %v", pattern.Regex, result.Error)
			continue
		}

		// Validate the conversion
		if err := converter.ValidateTemplate(result.Template, result.Payloads); err != nil {
			gologger.Warning().Msgf("Invalid template '%s': %v", result.Template, err)
			continue
		}

		// Create structured LearnedPattern object
		learnedPattern := &LearnedPattern{
			ID:         fmt.Sprintf("pattern_%03d", idx+1),
			Template:   result.Template,
			Regex:      pattern.Regex, // Include original regex for analysis
			Coverage:   pattern.Coverage,
			Ratio:      pattern.Ratio,
			Confidence: pattern.Confidence,
			Payloads:   result.Payloads,
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
		gologger.Warning().Msg("No valid DSL patterns after conversion")
		return []*LearnedPattern{}, nil
	}

	gologger.Info().Msgf("Successfully converted %d patterns to DSL format", len(learnedPatterns))
	return learnedPatterns, nil
}

// extractSubdomains extracts only the subdomain part from full domains
// Filters out domains with no subdomain (root-only domains)
// Example: "api-dev.example.com" → "api-dev"
// Example: "staging.prod.example.com" → "staging.prod"
// Example: "example.com" → filtered out (no subdomain)
func extractSubdomains(domains []string) []string {
	subdomains := make([]string, 0, len(domains))

	for _, domain := range domains {
		// Handle wildcard subdomains
		hostname := strings.TrimPrefix(domain, "*.")

		// Extract root domain (eTLD+1)
		etld, err := publicsuffix.EffectiveTLDPlusOne(hostname)
		if err != nil {
			gologger.Verbose().Msgf("Skipping domain with invalid TLD: %s", domain)
			continue
		}

		// Extract subdomain part by removing root domain
		if hostname == etld {
			// No subdomain (just root domain like "example.com")
			gologger.Verbose().Msgf("Skipping root-only domain: %s", domain)
			continue
		}

		// Remove root domain suffix to get subdomain
		subdomain := strings.TrimSuffix(hostname, "."+etld)

		if subdomain != "" {
			subdomains = append(subdomains, subdomain)
		}
	}

	return subdomains
}

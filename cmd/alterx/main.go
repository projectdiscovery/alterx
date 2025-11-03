package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/alterx/internal/runner"
	"github.com/projectdiscovery/gologger"
	"gopkg.in/yaml.v3"
)

func main() {

	cliOpts := runner.ParseFlags()

	// Handle pattern analysis mode (outputs learned patterns to config file)
	if cliOpts.Analyze {
		handleAnalyzeMode(cliOpts)
		return
	}

	alterOpts := alterx.Options{
		Domains:  cliOpts.Domains,
		Patterns: cliOpts.Patterns,
		Payloads: cliOpts.Payloads,
		Limit:    cliOpts.Limit,
		Enrich:   cliOpts.Enrich, // Original: enrich word payloads from input
		MaxSize:  cliOpts.MaxSize,
	}

	if cliOpts.PermutationConfig != "" {
		// read config
		config, err := alterx.NewConfig(cliOpts.PermutationConfig)
		if err != nil {
			gologger.Fatal().Msgf("failed to read %v file got: %v", cliOpts.PermutationConfig, err)
		}
		if len(config.Patterns) > 0 {
			alterOpts.Patterns = config.Patterns
		}
		if len(config.Payloads) > 0 {
			alterOpts.Payloads = config.Payloads
		}
	}

	// Learn patterns from input domains if mode is inferred or both
	if cliOpts.Mode == "inferred" || cliOpts.Mode == "both" {
		learnedPatterns := learnPatternsFromDomains(cliOpts.Domains)
		if len(learnedPatterns) > 0 {
			if cliOpts.Mode == "both" {
				// Merge learned patterns with manual patterns
				alterOpts.Patterns = append(alterOpts.Patterns, learnedPatterns...)
				gologger.Info().Msgf("Using %d manual + %d learned patterns", len(alterOpts.Patterns)-len(learnedPatterns), len(learnedPatterns))
			} else {
				// Use only learned patterns
				alterOpts.Patterns = learnedPatterns
				gologger.Info().Msgf("Using %d learned patterns", len(learnedPatterns))
			}
		}
	}

	// configure output writer
	var output io.Writer
	if cliOpts.Output != "" {
		fs, err := os.OpenFile(cliOpts.Output, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			gologger.Fatal().Msgf("failed to open output file %v got %v", cliOpts.Output, err)
		}
		output = fs
		defer func() {
			if err := fs.Close(); err != nil {
				gologger.Warning().Msgf("failed to close output file: %v", err)
			}
		}()
	} else {
		output = os.Stdout
	}

	// create new alterx instance with options
	m, err := alterx.New(&alterOpts)
	if err != nil {
		gologger.Fatal().Msgf("failed to parse alterx config got %v", err)
	}

	if cliOpts.Estimate {
		gologger.Info().Msgf("Estimated Payloads (including duplicates) : %v", m.EstimateCount())
		return
	}

	if err = m.ExecuteWithWriter(output); err != nil {
		gologger.Error().Msgf("failed to write output to file got %v", err)
	}

}

// handleAnalyzeMode learns patterns from input domains and outputs to config file or stdout
func handleAnalyzeMode(opts *runner.Options) {
	// Create pattern inducer
	// Pattern learning is root-agnostic (learns subdomain structures)
	// opts.Domains: full enumerated subdomains to learn patterns from (e.g., api-dev.example.com)
	inducer := alterx.NewPatternInducer(opts.Domains, 2)
	learnedPatterns, err := inducer.InferPatterns()
	if err != nil {
		gologger.Fatal().Msgf("Pattern learning failed: %v", err)
	}

	if len(learnedPatterns) == 0 {
		gologger.Warning().Msg("No patterns learned from input domains")
		return
	}

	// Calculate summary statistics
	totalCoverage := 0
	avgConfidence := 0.0
	for _, p := range learnedPatterns {
		totalCoverage += p.Coverage
		avgConfidence += p.Confidence
	}
	if len(learnedPatterns) > 0 {
		avgConfidence /= float64(len(learnedPatterns))
	}

	// Print concise summary to stderr (so it doesn't interfere with stdout YAML)
	gologger.Info().Msgf("Patterns: %d | Coverage: %d domains | Avg confidence: %.2f",
		len(learnedPatterns), totalCoverage, avgConfidence)

	// Prepare output structure matching permutations.yaml specification
	outputConfig := struct {
		LearnedPatterns []*alterx.LearnedPattern `yaml:"learned_patterns"`
		Metadata        map[string]interface{}   `yaml:"metadata,omitempty"`
	}{
		LearnedPatterns: learnedPatterns,
		Metadata: map[string]interface{}{
			"generated_by":           "alterx pattern induction",
			"total_domains_analyzed": len(opts.Domains),
			"patterns_learned":       len(learnedPatterns),
		},
	}

	// Marshal to YAML with proper formatting
	data, err := yaml.Marshal(outputConfig)
	if err != nil {
		gologger.Fatal().Msgf("Failed to marshal patterns: %v", err)
	}

	// Add header comment explaining the format
	header := []byte(`# ============================================================================
# Learned Patterns - Pattern Induction Results
# ============================================================================
# This file contains patterns learned from observed subdomains using the
# pattern induction algorithm (based on hierarchical partitioning and edit
# distance clustering with optimizations for scalability).
#
# Each pattern includes:
#   - id: Unique identifier
#   - template: DSL template with positional variables ({{p0}}, {{p1}}, etc.)
#   - coverage: Number of input domains matched by this pattern
#   - ratio: possible_generations / observed_count (lower is better)
#   - confidence: Quality score (0.0-1.0, higher is better)
#   - payloads: Inline payload definitions for positional variables
#   - examples: Sample domains that match this pattern
#
# TEMPLATE FORMAT:
#   Templates use AlterX DSL syntax with variable placeholders.
#   - Positional variables: {{p0}}, {{p1}}, {{p2}}, etc. for discovered tokens
#   - Semantic variables: {{env}}, {{service}}, {{region}}, etc. (if classified)
#   - Number variables: {{number}} for numeric sequences (with ±5 inference)
#   - Built-in variables: {{root}}, {{suffix}}, {{sub}}, {{tld}}
#
# NUMBER INFERENCE:
#   When numeric tokens are detected, the system infers a range by ±5.
#   Example: If "01", "02", "03" are found, generates range "01" to "08".
#   These are mapped to the {{number}} payload type for reuse.
#
# To use these patterns:
# 1. Review the patterns and their confidence scores
# 2. Copy desired patterns to your permutations.yaml file
# 3. Optionally classify positional payloads to semantic types in token_dictionary
# 4. Number payloads are automatically integrated with the {{number}} type
# ============================================================================

`)
	fullData := append(header, data...)

	// Output to file or stdout depending on -o flag
	if opts.Output != "" {
		// Write to file
		if err := os.WriteFile(opts.Output, fullData, 0644); err != nil {
			gologger.Fatal().Msgf("Failed to write patterns to %s: %v", opts.Output, err)
		}
		absPath, _ := filepath.Abs(opts.Output)
		gologger.Info().Msgf("Patterns written to: %s", absPath)
	} else {
		// Output to stdout (default behavior)
		os.Stdout.Write(fullData)
	}
}

// learnPatternsFromDomains learns patterns from domains and returns DSL template strings
func learnPatternsFromDomains(domains []string) []string {
	if len(domains) < 2 {
		gologger.Verbose().Msg("Not enough domains for pattern learning (minimum: 2)")
		return []string{}
	}

	// Pattern learning is root-agnostic (learns subdomain structures)
	gologger.Verbose().Msgf("Learning patterns from %d subdomains", len(domains))

	inducer := alterx.NewPatternInducer(domains, 2)
	learnedPatterns, err := inducer.InferPatterns()
	if err != nil {
		gologger.Warning().Msgf("Pattern learning failed: %v", err)
		return []string{}
	}

	// Extract just the template strings for runtime pattern generation
	templates := make([]string, 0, len(learnedPatterns))
	for _, p := range learnedPatterns {
		templates = append(templates, p.Template)
	}

	return templates
}

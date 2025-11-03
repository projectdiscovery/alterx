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

// handleAnalyzeMode learns patterns from input domains and outputs to config file
func handleAnalyzeMode(opts *runner.Options) {
	gologger.Info().Msgf("Pattern learning mode: analyzing %d domains", len(opts.Domains))

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

	// Determine output file
	outputFile := "learned_patterns.yaml"
	if opts.Output != "" {
		outputFile = opts.Output
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
# pattern induction algorithm (based on the regulator algorithm with optimizations).
#
# Each pattern includes:
#   - id: Unique identifier
#   - template: DSL template with positional variables ({{p0}}, {{p1}}, etc.)
#   - regex: Original regex pattern (for analysis and debugging)
#   - coverage: Number of input domains matched
#   - ratio: possible_generations / observed_count
#   - confidence: Quality score (0.0-1.0, higher is better)
#   - payloads: Inline payload definitions for positional variables
#   - examples: Sample domains that match this pattern
#
# REGEX vs DSL:
#   The 'regex' field shows the raw pattern learned from edit distance clustering.
#   The 'template' field shows the AlterX-compatible DSL format for generation.
#   Example: regex "(api|web)(-dev|-prod)" → template "{{p0}}{{p1}}.{{suffix}}"
#
# To use these patterns:
# 1. Review the patterns, their confidence scores, and regex forms
# 2. Copy desired patterns to your permutations.yaml file
# 3. Optionally classify payloads to semantic types in token_dictionary
# 4. Use the regex field to understand the underlying pattern structure
# ============================================================================

`)
	fullData := append(header, data...)

	// Write to file
	if err := os.WriteFile(outputFile, fullData, 0644); err != nil {
		gologger.Fatal().Msgf("Failed to write patterns to %s: %v", outputFile, err)
	}

	absPath, _ := filepath.Abs(outputFile)
	gologger.Info().Msgf("Learned patterns written to: %s", absPath)
	gologger.Info().Msgf("Patterns discovered: %d", len(learnedPatterns))

	// Log summary statistics
	totalCoverage := 0
	avgConfidence := 0.0
	for _, p := range learnedPatterns {
		totalCoverage += p.Coverage
		avgConfidence += p.Confidence
	}
	if len(learnedPatterns) > 0 {
		avgConfidence /= float64(len(learnedPatterns))
	}

	gologger.Info().Msgf("Total coverage: %d domains | Avg confidence: %.2f", totalCoverage, avgConfidence)
	gologger.Info().Msg("Review patterns and add high-confidence ones to your permutations.yaml config")
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

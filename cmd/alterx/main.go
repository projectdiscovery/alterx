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
		Mode:     cliOpts.Mode, // Pass pattern generation mode to mutator
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

	// Pattern learning is handled by the mutator based on Mode
	// (see mutator.go New() function which calls InferPatterns() based on opts.Mode)

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
#   - Number variables: {{number}} for numeric sequences (with ±1/±2 inference)
#   - Built-in variables: {{root}}, {{suffix}}, {{sub}}, {{tld}}
#
# NUMBER INFERENCE:
#   When numeric tokens are detected, the system infers a range by ±1 (or ±2 if min-1 < 0).
#   Example: If "01", "02", "03" are found, generates range "00" to "04".
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
		if absPath, err := filepath.Abs(opts.Output); err == nil {
			gologger.Info().Msgf("Patterns written to: %s", absPath)
		} else {
			gologger.Info().Msgf("Patterns written to: %s", opts.Output)
		}
	} else {
		// Output to stdout (default behavior)
		if _, err := os.Stdout.Write(fullData); err != nil {
			gologger.Error().Msgf("Failed to write patterns to stdout: %v", err)
		}
	}

	// Calculate estimated permutations for the learned patterns
	estimatedPermutations := 0
	// Create temporary mutator to estimate permutations using already-learned patterns
	// Extract template strings from learned patterns
	templates := make([]string, 0, len(learnedPatterns))
	for _, p := range learnedPatterns {
		templates = append(templates, p.Template)
	}

	alterOpts := alterx.Options{
		Domains:         opts.Domains,
		Payloads:        make(map[string][]string),
		Patterns:        templates,       // Use already-learned patterns
		LearnedPatterns: learnedPatterns, // Pass learned patterns directly to avoid re-learning
		Mode:            "default",       // Don't trigger re-learning
	}

	// Create mutator to estimate permutations
	if m, err := alterx.New(&alterOpts); err == nil {
		estimatedPermutations = m.EstimateCount()
	} else {
		gologger.Warning().Msgf("Failed to estimate permutations: %v", err)
	}

	// Print comprehensive summary to stderr (after YAML output)
	gologger.Info().Msgf("Input: %d domains | Patterns: %d | Coverage: %d domains | Avg confidence: %.2f | Estimated permutations: %d",
		len(opts.Domains), len(learnedPatterns), totalCoverage, avgConfidence, estimatedPermutations)
}


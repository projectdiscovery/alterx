package main

import (
	"io"
	"os"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/alterx/internal/runner"
	"github.com/projectdiscovery/gologger"
)

func main() {

	cliOpts := runner.ParseFlags()

	// Write output with deduplication
	output := getOutputWriter(cliOpts.Output)

	// Build alterx options with all modes supported
	alterOpts := alterx.Options{
		Domains:          cliOpts.Domains,
		Patterns:         cliOpts.Patterns,
		Payloads:         cliOpts.Payloads,
		Limit:            cliOpts.Limit,
		Enrich:           cliOpts.Enrich,
		MaxSize:          cliOpts.MaxSize,
		Mode:             cliOpts.Mode,
		MinDistance:      cliOpts.MinDistance,
		MaxDistance:      cliOpts.MaxDistance,
		PatternThreshold: cliOpts.PatternThreshold,
		QualityRatio:     float64(cliOpts.QualityRatio),
		NgramsLimit:      cliOpts.NgramsLimit,
		MaxLength:        1000,
	}

	if cliOpts.PermutationConfig != "" {
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

	m, err := alterx.New(&alterOpts)
	if err != nil {
		gologger.Fatal().Msgf("failed to initialize alterx: %v", err)
	}

	// Save rules if requested
	if cliOpts.SaveRules != "" {
		if err := m.SaveRules(cliOpts.SaveRules); err != nil {
			gologger.Error().Msgf("failed to save rules: %v", err)
		}
	}

	if cliOpts.Estimate {
		estimated := m.EstimateCount()
		gologger.Info().Msgf("Estimated Payloads (including duplicates): %v", estimated)
		return
	}

	// Execute mutator (handles all modes internally)
	if err = m.ExecuteWithWriter(output); err != nil {
		gologger.Error().Msgf("failed to execute alterx: %v", err)
	}

	gologger.Info().Msgf("Generated %d total unique subdomains", m.PayloadCount())
}

// getOutputWriter returns the appropriate output writer
func getOutputWriter(outputPath string) io.Writer {
	if outputPath != "" {
		fs, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			gologger.Fatal().Msgf("failed to open output file %v got %v", outputPath, err)
		}
		return fs
	}
	return os.Stdout
}

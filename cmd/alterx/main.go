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

	output, closer, err := newOutputWriter(cliOpts.Output, os.Stdout)
	if err != nil {
		gologger.Fatal().Msgf("failed to open output file %v got %v", cliOpts.Output, err)
	}
	defer func() {
		if cerr := closer.Close(); cerr != nil {
			gologger.Error().Msgf("failed to close output file: %v", cerr)
		}
	}()

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

	if cliOpts.Estimate {
		estimated := m.EstimateCount()
		gologger.Info().Msgf("Estimated Payloads (including duplicates): %v", estimated)
		return
	}

	// Execute mutator (handles all modes internally)
	if err = m.ExecuteWithWriter(output); err != nil {
		gologger.Error().Msgf("failed to execute alterx: %v", err)
	}

	// Save rules if requested (must be after Execute to ensure mining is complete)
	if cliOpts.SaveRules != "" {
		if err := m.SaveRules(cliOpts.SaveRules); err != nil {
			gologger.Error().Msgf("failed to save rules: %v", err)
		}
	}
}

// newOutputWriter writes results to stdout, and also to outputPath when set.
func newOutputWriter(outputPath string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if outputPath == "" {
		return stdout, io.NopCloser(nil), nil
	}
	fs, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, err
	}
	return io.MultiWriter(stdout, fs), fs, nil
}

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

	alterOpts := alterx.Options{
		Domains:  cliOpts.Domains,
		Patterns: cliOpts.Patterns,
		Payloads: cliOpts.Payloads,
		Limit:    cliOpts.Limit,
		Enrich:   cliOpts.Enrich, // enrich payloads
		Mode:     cliOpts.Mode,   // pattern generation mode
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

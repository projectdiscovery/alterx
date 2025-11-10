package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/alterx/internal/patternmining"
	"github.com/projectdiscovery/alterx/internal/runner"
	"github.com/projectdiscovery/gologger"
)

func main() {

	cliOpts := runner.ParseFlags()

	// Validate mode
	if cliOpts.Mode != "default" && cliOpts.Mode != "discover" && cliOpts.Mode != "both" {
		gologger.Fatal().Msgf("invalid mode: %s (must be 'default', 'discover', or 'both')", cliOpts.Mode)
	}

	// Handle pattern mining modes (discover or both)
	var minedPatterns []string
	if cliOpts.Mode == "discover" || cliOpts.Mode == "both" {
		target := extractTargetDomain(cliOpts.Domains)
		if target == "" {
			gologger.Fatal().Msgf("pattern mining requires domains with a common target (e.g., sub.example.com)")
		}

		gologger.Info().Msgf("Pattern mining mode enabled (Go port of Regulator by @cramppet)")
		gologger.Info().Msgf("Target domain: %s", target)

		miner := patternmining.NewMiner(&patternmining.Options{
			Domains:          cliOpts.Domains,
			Target:           target,
			MinDistance:      cliOpts.MinDistance,
			MaxDistance:      cliOpts.MaxDistance,
			PatternThreshold: cliOpts.PatternThreshold,
			QualityRatio:     float64(cliOpts.QualityRatio),
			MaxLength:        1000,
			NgramsLimit:      cliOpts.NgramsLimit,
		})

		result, err := miner.Mine()
		if err != nil {
			gologger.Fatal().Msgf("pattern mining failed: %v", err)
		}

		// Save rules if requested
		if cliOpts.SaveRules != "" {
			if err := miner.SaveRules(result, cliOpts.SaveRules); err != nil {
				gologger.Error().Msgf("failed to save rules: %v", err)
			} else {
				gologger.Info().Msgf("Saved %d patterns to %s", len(result.Patterns), cliOpts.SaveRules)
			}
		}

		// Generate subdomains from discovered patterns
		if cliOpts.Mode == "discover" {
			// In discover mode, only use mined patterns
			generated := miner.GenerateFromPatterns(result.Patterns)

			// Write output
			output := getOutputWriter(cliOpts.Output)
			defer closeOutput(output, cliOpts.Output)

			for _, subdomain := range generated {
				if cliOpts.Limit > 0 && len(generated) >= cliOpts.Limit {
					break
				}
				output.Write([]byte(subdomain + "\n"))
			}

			gologger.Info().Msgf("Generated %d subdomains from discovered patterns", len(generated))
			return
		}

		// In 'both' mode, collect mined patterns for combination
		minedPatterns = result.Patterns
		gologger.Info().Msgf("Discovered %d patterns, combining with user-defined patterns", len(minedPatterns))
	}

	// Handle default mode or 'both' mode
	alterOpts := alterx.Options{
		Domains:  cliOpts.Domains,
		Patterns: cliOpts.Patterns,
		Payloads: cliOpts.Payloads,
		Limit:    cliOpts.Limit,
		Enrich:   cliOpts.Enrich,
		MaxSize:  cliOpts.MaxSize,
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

	// In 'both' mode, add mined patterns to user patterns
	if cliOpts.Mode == "both" && len(minedPatterns) > 0 {
		// Convert mined patterns to alterx format
		// Mined patterns are already in regex format, but alterx expects template format
		// For now, we'll generate from mined patterns separately and combine results
		target := extractTargetDomain(cliOpts.Domains)
		miner := patternmining.NewMiner(&patternmining.Options{
			Domains:          cliOpts.Domains,
			Target:           target,
			MinDistance:      cliOpts.MinDistance,
			MaxDistance:      cliOpts.MaxDistance,
			PatternThreshold: cliOpts.PatternThreshold,
			QualityRatio:     float64(cliOpts.QualityRatio),
			MaxLength:        1000,
			NgramsLimit:      cliOpts.NgramsLimit,
		})

		generated := miner.GenerateFromPatterns(minedPatterns)

		// Use a dedupe set for both modes
		allResults := make(map[string]bool)
		for _, g := range generated {
			allResults[g] = true
		}

		// Now run the normal alterx generation
		output := getOutputWriter(cliOpts.Output)
		defer closeOutput(output, cliOpts.Output)

		m, err := alterx.New(&alterOpts)
		if err != nil {
			gologger.Fatal().Msgf("failed to parse alterx config got %v", err)
		}

		if cliOpts.Estimate {
			estimated := m.EstimateCount() + len(generated)
			gologger.Info().Msgf("Estimated Payloads (including duplicates): %v", estimated)
			return
		}

		// First write mined results
		count := 0
		for subdomain := range allResults {
			if cliOpts.Limit > 0 && count >= cliOpts.Limit {
				break
			}
			output.Write([]byte(subdomain + "\n"))
			count++
		}

		// Then write alterx results (with deduplication)
		if err = executeAlterxWithDedup(m, output, allResults, cliOpts.Limit-count); err != nil {
			gologger.Error().Msgf("failed to write output to file got %v", err)
		}

		gologger.Info().Msgf("Generated %d total unique subdomains (both modes)", len(allResults))
		return
	}

	// Standard default mode
	output := getOutputWriter(cliOpts.Output)
	defer closeOutput(output, cliOpts.Output)

	m, err := alterx.New(&alterOpts)
	if err != nil {
		gologger.Fatal().Msgf("failed to parse alterx config got %v", err)
	}

	if cliOpts.Estimate {
		gologger.Info().Msgf("Estimated Payloads (including duplicates): %v", m.EstimateCount())
		return
	}

	if err = m.ExecuteWithWriter(output); err != nil {
		gologger.Error().Msgf("failed to write output to file got %v", err)
	}
}

// extractTargetDomain extracts the common target domain from input domains
func extractTargetDomain(domains []string) string {
	if len(domains) == 0 {
		return ""
	}

	// Take the first domain and extract root domain
	first := domains[0]
	parts := strings.Split(first, ".")
	if len(parts) >= 2 {
		// Return last two parts as target domain (e.g., "example.com")
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return first
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

// closeOutput closes the output writer if it's a file
func closeOutput(output io.Writer, outputPath string) {
	if outputPath != "" {
		if closer, ok := output.(io.Closer); ok {
			closer.Close()
		}
	}
}

// executeAlterxWithDedup executes alterx with deduplication against existing results
func executeAlterxWithDedup(m *alterx.Mutator, output io.Writer, existing map[string]bool, remainingLimit int) error {
	// We need to capture alterx output and dedupe it
	// Create a custom writer that dedupes
	count := 0
	resChan := m.Execute(context.TODO())

	for value := range resChan {
		if remainingLimit > 0 && count >= remainingLimit {
			continue
		}
		if !existing[value] && !strings.HasPrefix(value, "-") {
			existing[value] = true
			output.Write([]byte(value + "\n"))
			count++
		}
	}
	return nil
}

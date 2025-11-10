package main

import (
	"io"
	"os"
	"strings"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/alterx/internal/patternmining"
	"github.com/projectdiscovery/alterx/internal/runner"
	"github.com/projectdiscovery/gologger"
	"golang.org/x/net/publicsuffix"
)

func main() {

	cliOpts := runner.ParseFlags()

	// Validate mode
	if cliOpts.Mode != "default" && cliOpts.Mode != "discover" && cliOpts.Mode != "both" {
		gologger.Fatal().Msgf("invalid mode: %s (must be 'default', 'discover', or 'both')", cliOpts.Mode)
	}

	// Write output with deduplication
	output := getOutputWriter(cliOpts.Output)
	defer closeOutput(output, cliOpts.Output)
	// we intentionally remove all known subdomains from the output
	// that way only the discovered subdomains are included in the output
	dedupWriter := alterx.NewDedupingWriter(output, cliOpts.Domains...)
	defer dedupWriter.Close()

	var estimatedDiscoverOutputs = 0

	// Handle pattern mining modes (discover or both)
	var minedPatterns []string
	if cliOpts.Mode == "discover" || cliOpts.Mode == "both" {
		target := getNValidateRootDomain(cliOpts.Domains)
		if target == "" {
			gologger.Fatal().Msgf("pattern mining requires domains with a common target (e.g., sub.example.com)")
		}
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

		estimatedDiscoverOutputs = int(miner.EstimateCount(result.Patterns))

		// Generate subdomains from discovered patterns
		// and exit early
		if cliOpts.Mode == "discover" {
			// In discover mode, only use mined patterns
			generated := miner.GenerateFromPatterns(result.Patterns)
			for _, subdomain := range generated {
				dedupWriter.Write([]byte(subdomain + "\n"))
			}
			gologger.Info().Msgf("Generated %d unique subdomains from discovered patterns", dedupWriter.Count())
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

	m, err := alterx.New(&alterOpts)
	if err != nil {
		gologger.Fatal().Msgf("failed to parse alterx config got %v", err)
	}

	if cliOpts.Estimate {
		estimated := m.EstimateCount() + estimatedDiscoverOutputs
		gologger.Info().Msgf("Estimated Payloads (including duplicates): %v", estimated)
		return
	}
	// Write alterx results to same dedupWriter (automatic deduplication)
	if err = m.ExecuteWithWriter(dedupWriter); err != nil {
		gologger.Error().Msgf("failed to write output to file got %v", err)
	}

	gologger.Info().Msgf("Generated %d total unique subdomains (both modes)", dedupWriter.Count())
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

func getNValidateRootDomain(domains []string) string {
	if len(domains) == 0 {
		return ""
	}

	var rootDomain string
	// parse root domain from publicsuffix for first entry
	for _, domain := range domains {
		if strings.TrimSpace(domain) == "" {
			continue
		}
		if rootDomain == "" {
			root, _ := publicsuffix.EffectiveTLDPlusOne(domain)
			rootDomain = root
		} else {
			if !strings.HasSuffix(domain, rootDomain) {
				gologger.Fatal().Msgf("domain %v does not have the same root domain as %v, only homogeneous domains are supported in discover mode", domain, rootDomain)
			}
		}
	}
	return ""
}

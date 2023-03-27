package main

import (
	"fmt"
	"io"
	"os"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	updateutils "github.com/projectdiscovery/utils/update"
)

type Options struct {
	// list of Domains to use as base
	Domains goflags.StringSlice
	// list of pattersn to use while creating permutations
	// if empty DefaultPatterns are used
	Patterns           goflags.StringSlice
	WordList           goflags.StringSlice
	Output             string
	Config             string
	PermutationConfig  string
	DryRun             bool
	DisableUpdateCheck bool
	Version            bool
	Verbose            bool
}

func main() {

	opts := &Options{}
	flagSet := goflags.NewFlagSet()
	flagSet.SetDescription(`Fast and customizable subdomain wordlist generator using DSL.`)

	flagSet.CreateGroup("input", "Input",
		flagSet.StringSliceVarP(&opts.Domains, "list", "l", nil, "file containing list of subdomains to use as base", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&opts.WordList, "word", "w", nil, "words to use with alterx permutation (optional)", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&opts.Patterns, "pattern", "p", nil, "words to use with alterx permutation (optional)", goflags.FileCommaSeparatedStringSliceOptions),
	)

	flagSet.CreateGroup("output", "Output",
		flagSet.StringVarP(&opts.Output, "output", "o", "", "output file to write altered subdomain list"),
		flagSet.BoolVarP(&opts.Verbose, "verbose", "v", false, "display verbose output"),
		flagSet.BoolVar(&opts.Version, "version", false, "display project version"),
	)

	flagSet.CreateGroup("config", "Config",
		flagSet.StringVar(&opts.Config, "config", alterx.DefaultConfigFilePath, `alterx cli config file (default '$HOME/.config/alterx/config.yaml')`),
		flagSet.StringVar(&opts.PermutationConfig, "ac", alterx.DefaultPermutationConfigFilePath, `alterx permutation config file (default '$HOME/.config/alterx/permutation.yaml')`),
		flagSet.BoolVarP(&opts.DryRun, "dry-run", "dn", false, "dry run and only return generated permutation counter"),
	)

	flagSet.CreateGroup("update", "Update",
		flagSet.CallbackVarP(GetUpdateCallback(), "update", "up", "update alterx to latest version"),
		flagSet.BoolVarP(&opts.DisableUpdateCheck, "disable-update-check", "duc", false, "disable automatic katana update check"),
	)

	if err := flagSet.Parse(); err != nil {
		gologger.Fatal().Msgf("Could not read flags: %s\n", err)
	}

	showBanner()

	if opts.Version {
		gologger.Info().Msgf("Current version: %s", version)
		os.Exit(0)
	}

	if !opts.DisableUpdateCheck {
		latestVersion, err := updateutils.GetVersionCheckCallback("alterx")()
		if err != nil {
			if opts.Verbose {
				gologger.Error().Msgf("alterx version check failed: %v", err.Error())
			}
		} else {
			gologger.Info().Msgf("Current alterx version %v %v", version, updateutils.GetVersionDescription(version, latestVersion))
		}
	}

	alterOpts := alterx.Options{}
	alterOpts.Domains = opts.Domains
	config, err := alterx.NewConfig(opts.Config)
	if err != nil {
		panic(err)
	}
	if len(config.Patterns) > 0 {
		alterOpts.Patterns = config.Patterns
	}
	if len(config.Payloads) > 0 {
		alterOpts.Payloads = config.Payloads
	}

	var output io.Writer
	if opts.Output != "" {
		fs, err := os.OpenFile(opts.Output, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		output = fs
		defer fs.Close()
	} else {
		output = os.Stdout
	}

	m, err := alterx.New(&alterOpts)
	if err != nil {
		panic(err)
	}

	if opts.DryRun {
		fmt.Println(m.EstimateCount())
		return
	}

	err = m.ExecuteWithWriter(output)
	if err != nil {
		panic(err)
	}
}

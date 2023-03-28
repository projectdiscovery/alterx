package runner

import (
	"io"
	"os"
	"strings"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	fileutil "github.com/projectdiscovery/utils/file"
	updateutils "github.com/projectdiscovery/utils/update"
)

type Options struct {
	Domains            goflags.StringSlice // Subdomains to use as base
	Patterns           goflags.StringSlice // Input Patterns
	Payloads           map[string][]string // Input Payloads/WordLists
	Output             string
	Config             string
	PermutationConfig  string
	DryRun             bool
	DisableUpdateCheck bool
	Verbose            bool
	// internal/unexported fields
	wordlists goflags.RuntimeMap
}

func ParseFlags() *Options {
	opts := &Options{}
	flagSet := goflags.NewFlagSet()
	flagSet.SetDescription(`Fast and customizable subdomain wordlist generator using DSL.`)

	flagSet.CreateGroup("input", "Input",
		flagSet.StringSliceVarP(&opts.Domains, "list", "l", nil, "subdomains to use when creating permutations (comma-separated, file)", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.RuntimeMapVarP(&opts.wordlists, "payload", "pp", nil, "payloads in pattern to replace/use in key=value format (-w 'words=words.txt')"),
		flagSet.StringSliceVarP(&opts.Patterns, "pattern", "p", nil, "input patterns for alterx (comma-seperated, file)", goflags.FileCommaSeparatedStringSliceOptions),
	)

	flagSet.CreateGroup("output", "Output",
		flagSet.StringVarP(&opts.Output, "output", "o", "", "output file to write altered subdomain list"),
		flagSet.BoolVarP(&opts.Verbose, "verbose", "v", false, "display verbose output"),
		flagSet.CallbackVar(printVersion, "version", "display alterx version"),
	)

	flagSet.CreateGroup("config", "Config",
		flagSet.StringVar(&opts.Config, "config", "", `alterx cli config file (default '$HOME/.config/alterx/config.yaml')`),
		flagSet.StringVar(&opts.PermutationConfig, "ac", "", `alterx permutation config file (default '$HOME/.config/alterx/permutation_vxxx.yaml')`),
		flagSet.BoolVarP(&opts.DryRun, "dry-run", "dr", false, "dry run and only return generated permutation counter"),
	)

	flagSet.CreateGroup("update", "Update",
		flagSet.CallbackVarP(GetUpdateCallback(), "update", "up", "update alterx to latest version"),
		flagSet.BoolVarP(&opts.DisableUpdateCheck, "disable-update-check", "duc", false, "disable automatic katana update check"),
	)

	if err := flagSet.Parse(); err != nil {
		gologger.Fatal().Msgf("Could not read flags: %s\n", err)
	}

	if opts.Config != "" {
		if err := flagSet.MergeConfigFile(opts.Config); err != nil {
			gologger.Error().Msgf("failed to read config file got %v", err)
		}
	}

	showBanner()

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

	opts.Payloads = map[string][]string{}
	for k, v := range opts.wordlists.AsMap() {
		value, ok := v.(string)
		if !ok {
			continue
		}
		if fileutil.FileExists(value) {
			bin, err := os.ReadFile(value)
			if err != nil {
				gologger.Error().Msgf("failed to read wordlist %v got %v", value, err)
				continue
			}
			wordlist := strings.Fields(string(bin))
			opts.Payloads[k] = wordlist
		} else {
			opts.Payloads[k] = []string{value}
		}
	}

	// read from stdin
	if fileutil.HasStdin() {
		bin, err := io.ReadAll(os.Stdin)
		if err != nil {
			gologger.Error().Msgf("failed to read input from stdin got %v", err)
		}
		opts.Domains = strings.Fields(string(bin))
	}

	// TODO: replace Options.Domains with Input String Channel
	if len(opts.Domains) == 0 {
		gologger.Fatal().Msgf("alterx: no input found")
	}

	return opts
}

func printVersion() {
	gologger.Info().Msgf("Current version: %s", version)
	os.Exit(0)
}

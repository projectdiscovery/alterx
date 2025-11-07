package runner

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/levels"
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
	Estimate           bool
	DisableUpdateCheck bool
	Verbose            bool
	Silent             bool
	Enrich             bool
	Limit              int
	MaxSize            int
	// Mining/Discovery options
	Discover               bool
	MinLDist               int
	MaxLDist               int
	PatternThreshold       int
	PatternQualityRatio    int
	NgramsLimit            int
	// internal/unexported fields
	wordlists goflags.RuntimeMap
}

func ParseFlags() *Options {
	var maxFileSize goflags.Size
	opts := &Options{}
	flagSet := goflags.NewFlagSet()
	flagSet.SetDescription(`Fast and customizable subdomain wordlist generator using DSL.`)

	flagSet.CreateGroup("input", "Input",
		flagSet.StringSliceVarP(&opts.Domains, "list", "l", nil, "subdomains to use when creating permutations (stdin, comma-separated, file)", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&opts.Patterns, "pattern", "p", nil, "custom permutation patterns input to generate (comma-seperated, file)", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.RuntimeMapVarP(&opts.wordlists, "payload", "pp", nil, "custom payload pattern input to replace/use in key=value format (-pp 'word=words.txt')"),
	)

	flagSet.CreateGroup("output", "Output",
		flagSet.BoolVarP(&opts.Estimate, "estimate", "es", false, "estimate permutation count without generating payloads"),
		flagSet.StringVarP(&opts.Output, "output", "o", "", "output file to write altered subdomain list"),
		flagSet.SizeVarP(&maxFileSize, "max-size", "ms", "", "Max export data size (kb, mb, gb, tb) (default mb)"),
		flagSet.BoolVarP(&opts.Verbose, "verbose", "v", false, "display verbose output"),
		flagSet.BoolVar(&opts.Silent, "silent", false, "display results only"),
		flagSet.CallbackVar(printVersion, "version", "display alterx version"),
	)

	flagSet.CreateGroup("config", "Config",
		flagSet.StringVar(&opts.Config, "config", "", `alterx cli config file (default '$HOME/.config/alterx/config.yaml')`),
		flagSet.BoolVarP(&opts.Enrich, "enrich", "en", false, "enrich wordlist by extracting words from input"),
		flagSet.StringVar(&opts.PermutationConfig, "ac", "", fmt.Sprintf(`alterx permutation config file (default '$HOME/.config/alterx/permutation_%v.yaml')`, version)),
		flagSet.IntVar(&opts.Limit, "limit", 0, "limit the number of results to return (default 0)"),
	)

	flagSet.CreateGroup("mining", "Pattern Mining",
		flagSet.BoolVarP(&opts.Discover, "discover", "d", false, "discover patterns from input domains (automatic mode)"),
		flagSet.IntVar(&opts.MinLDist, "min-distance", 2, "minimum levenshtein distance for clustering (used in discover mode)"),
		flagSet.IntVar(&opts.MaxLDist, "max-distance", 5, "maximum levenshtein distance for clustering (used in discover mode)"),
		flagSet.IntVar(&opts.PatternThreshold, "pattern-threshold", 1000, "pattern threshold for filtering low-quality patterns (used in discover mode)"),
		flagSet.IntVar(&opts.PatternQualityRatio, "quality-ratio", 100, "pattern quality ratio threshold (used in discover mode)"),
		flagSet.IntVar(&opts.NgramsLimit, "ngrams-limit", 0, "limit number of n-grams to process (0 = all, used in discover mode)"),
	)

	flagSet.CreateGroup("update", "Update",
		flagSet.CallbackVarP(GetUpdateCallback(), "update", "up", "update alterx to latest version"),
		flagSet.BoolVarP(&opts.DisableUpdateCheck, "disable-update-check", "duc", false, "disable automatic alterx update check"),
	)

	if err := flagSet.Parse(); err != nil {
		gologger.Fatal().Msgf("Could not read flags: %s\n", err)
	}

	if opts.Config != "" {
		if err := flagSet.MergeConfigFile(opts.Config); err != nil {
			gologger.Error().Msgf("failed to read config file got %v", err)
		}
	}

	if opts.Silent {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelSilent)
	} else if opts.Verbose {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelVerbose)
	}
	showBanner()

	if !opts.DisableUpdateCheck {
		latestVersion, err := updateutils.GetToolVersionCallback("alterx", version)()
		if err != nil {
			if opts.Verbose {
				gologger.Error().Msgf("alterx version check failed: %v", err.Error())
			}
		} else {
			gologger.Info().Msgf("Current alterx version %v %v", version, updateutils.GetVersionDescription(version, latestVersion))
		}
	}

	opts.MaxSize = math.MaxInt
	if maxFileSize > 0 {
		opts.MaxSize = int(maxFileSize)
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

package runner

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/levels"
	errorutil "github.com/projectdiscovery/utils/errors"
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
	// internal/unexported fields
	wordlists goflags.RuntimeMap
}

func ParseFlags() *Options {
	var maxFileSize string
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
		flagSet.StringVarP(&maxFileSize, "max-size", "ms", "", "Max export data size (kb, mb, gb, tb) (default mb)"),
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
		latestVersion, err := updateutils.GetVersionCheckCallback("alterx")()
		if err != nil {
			if opts.Verbose {
				gologger.Error().Msgf("alterx version check failed: %v", err.Error())
			}
		} else {
			gologger.Info().Msgf("Current alterx version %v %v", version, updateutils.GetVersionDescription(version, latestVersion))
		}
	}

	opts.MaxSize = math.MaxInt
	if len(maxFileSize) > 0 {
		maxSize, err := convertFileSizeToBytes(maxFileSize)
		if err != nil {
			gologger.Fatal().Msgf("Could not parse max-size: %s\n", err)
		}
		opts.MaxSize = maxSize
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

func convertFileSizeToBytes(maxFileSize string) (int, error) {
	maxFileSize = strings.ToLower(maxFileSize)
	// default to mb
	if size, err := strconv.Atoi(maxFileSize); err == nil {
		return size * 1024 * 1024, nil
	}
	if len(maxFileSize) < 3 {
		return 0, errorutil.New("invalid max-size value")
	}
	sizeUnit := maxFileSize[len(maxFileSize)-2:]
	size, err := strconv.Atoi(maxFileSize[:len(maxFileSize)-2])
	if err != nil {
		return 0, err
	}
	if size < 0 {
		return 0, errorutil.New("max-size cannot be negative")
	}
	if strings.EqualFold(sizeUnit, "kb") {
		return size * 1024, nil
	} else if strings.EqualFold(sizeUnit, "mb") {
		return size * 1024 * 1024, nil
	} else if strings.EqualFold(sizeUnit, "gb") {
		return size * 1024 * 1024 * 1024, nil
	} else if strings.EqualFold(sizeUnit, "tb") {
		return size * 1024 * 1024 * 1024 * 1024, nil
	}
	return 0, errorutil.New("Unsupported max-size unit")
}

package alterx

import (
	"os"
	"strings"

	_ "embed"

	"github.com/projectdiscovery/gologger"
	fileutil "github.com/projectdiscovery/utils/file"
	"gopkg.in/yaml.v3"
)

//go:embed permutations.yaml
var DefaultPermutationsBin []byte

// DefaultConfig contains default patterns and payloads
var DefaultConfig Config

type Config struct {
	Patterns []string            `yaml:"patterns"`
	Payloads map[string][]string `yaml:"payloads"`
}

// NewConfig reads config from file
func NewConfig(filePath string) (*Config, error) {
	bin, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err = yaml.Unmarshal(bin, &cfg); err != nil {
		return nil, err
	}

	var words []string
	for _, p := range cfg.Payloads["word"] {
		if !fileutil.FileExists(p) {
			words = append(words, p)
		} else {
			wordBytes, err := os.ReadFile(p)
			if err != nil {
				gologger.Error().Msgf("failed to read wordlist from %v got %v", p, err)
				continue
			}
			words = append(words, strings.Fields(string(wordBytes))...)
		}
	}
	cfg.Payloads["word"] = words
	return &cfg, nil
}

func init() {
	if err := yaml.Unmarshal(DefaultPermutationsBin, &DefaultConfig); err != nil {
		gologger.Error().Msgf("default wordlist not found: got %v", err)
	}
}

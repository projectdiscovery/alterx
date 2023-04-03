package alterx

import (
	"os"

	_ "embed"

	"github.com/projectdiscovery/gologger"
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
	return &cfg, nil
}

func init() {
	if err := yaml.Unmarshal(DefaultPermutationsBin, &DefaultConfig); err != nil {
		gologger.Error().Msgf("default wordlist not found: got %v", err)
	}
}

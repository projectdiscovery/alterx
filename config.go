package alterx

import (
	"os"
	"strings"

	_ "embed"

	"github.com/projectdiscovery/alterx/internal/inducer"
	"github.com/projectdiscovery/gologger"
	fileutil "github.com/projectdiscovery/utils/file"
	"gopkg.in/yaml.v3"
)

//go:embed permutations.yaml
var DefaultPermutationsBin []byte

// DefaultConfig contains default patterns and payloads
var DefaultConfig Config

// TokenDictionaryConfig represents the YAML structure for token_dictionary
type TokenDictionaryConfig struct {
	Env     map[string]interface{} `yaml:"env"`
	Region  map[string]interface{} `yaml:"region"`
	Service map[string]interface{} `yaml:"service"`
}

type Config struct {
	Patterns              []string                 `yaml:"patterns"`
	Payloads              map[string][]string      `yaml:"payloads"`
	TokenDictionaryConfig *TokenDictionaryConfig   `yaml:"token_dictionary,omitempty"`
	tokenDictionary       *inducer.TokenDictionary // Parsed dictionary for internal use
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

	// Parse token dictionary if present
	if cfg.TokenDictionaryConfig != nil {
		cfg.tokenDictionary = cfg.parseTokenDictionary()
	}

	return &cfg, nil
}

// GetTokenDictionary returns the parsed token dictionary for pattern induction
// Returns nil if no token dictionary is configured
func (c *Config) GetTokenDictionary() *inducer.TokenDictionary {
	return c.tokenDictionary
}

// parseTokenDictionary converts the YAML token_dictionary structure into inducer.TokenDictionary
// The YAML structure has nested description/values fields, we need to extract the values arrays
func (c *Config) parseTokenDictionary() *inducer.TokenDictionary {
	if c.TokenDictionaryConfig == nil {
		return nil
	}

	dict := &inducer.TokenDictionary{}

	// Parse env category
	if envData := c.TokenDictionaryConfig.Env; envData != nil {
		if values, ok := envData["values"].([]interface{}); ok {
			dict.Env = interfaceSliceToStringSlice(values)
		}
	}

	// Parse region category
	if regionData := c.TokenDictionaryConfig.Region; regionData != nil {
		if values, ok := regionData["values"].([]interface{}); ok {
			dict.Region = interfaceSliceToStringSlice(values)
		}
	}

	// Parse service category
	if serviceData := c.TokenDictionaryConfig.Service; serviceData != nil {
		if values, ok := serviceData["values"].([]interface{}); ok {
			dict.Service = interfaceSliceToStringSlice(values)
		}
	}

	return dict
}

// interfaceSliceToStringSlice converts []interface{} to []string
func interfaceSliceToStringSlice(input []interface{}) []string {
	result := make([]string, 0, len(input))
	for _, item := range input {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func init() {
	if err := yaml.Unmarshal(DefaultPermutationsBin, &DefaultConfig); err != nil {
		gologger.Error().Msgf("default wordlist not found: got %v", err)
	}

	// Parse token dictionary if present in default config
	if DefaultConfig.TokenDictionaryConfig != nil {
		DefaultConfig.tokenDictionary = DefaultConfig.parseTokenDictionary()
	}
}

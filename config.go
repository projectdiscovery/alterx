package alterx

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TODO: embed defaults to a config file instead of hardcoding
var defaultWordList = map[string][]string{
	"word": {
		"dev", "lib", "prod", "stage", "wp",
	},
}

var defaultPatterns = []string{
	"{{sub}}-{{word}}.{{suffix}}", // ex: api-prod.scanme.sh
	"{{word}}-{{sub}}.{{suffix}}", // ex: prod-api.scanme.sh
	"{{word}}.{{sub}}.{{suffix}}", // ex: prod.api.scanme.sh
	"{{sub}}.{{word}}.{{suffix}}", // ex: api.prod.scanme.sh
}

var DefaultConfig *Config = &Config{
	Patterns: defaultPatterns,
	Payloads: defaultWordList,
}

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

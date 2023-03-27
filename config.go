package alterx

import (
	"os"
	"path/filepath"
	"gopkg.in/yaml.v3"
)

var (
	DefaultConfigFilePath            = filepath.Join(getUserHomeDir(), ".config/alterx/config.yaml")
	DefaultPermutationConfigFilePath = filepath.Join(getUserHomeDir(), ".config/alterx/permutation.yaml")
)

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

// Generate Sample creates a sample yaml file with default/sample values
func GenerateSample(filePath string) error {
	cfg := Config{
		Patterns: DefaultPatterns,
		Payloads: DefaultWordList,
	}
	bin, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, bin, 0644)
}

func getUserHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return homeDir
}

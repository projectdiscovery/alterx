package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/gologger"
	fileutil "github.com/projectdiscovery/utils/file"
	"gopkg.in/yaml.v3"
)

func getUserHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return homeDir
}

func init() {
	defaultPermutationCfg := filepath.Join(getUserHomeDir(), fmt.Sprintf(".config/alterx/permutation_%v.yaml", version))
	// create default permutation.yaml config if does not exist
	if fileutil.FileExists(defaultPermutationCfg) {
		// if it exists use that data as default
		if bin, err := os.ReadFile(defaultPermutationCfg); err == nil {
			var cfg alterx.Config
			if errx := yaml.Unmarshal(bin, &cfg); errx == nil {
				alterx.DefaultConfig = cfg
				return
			}
		}
	}
	if err := os.WriteFile(defaultPermutationCfg, alterx.DefaultPermutationsBin, 0600); err != nil {
		gologger.Error().Msgf("failed to save default config to %v got: %v", defaultPermutationCfg, err)
	}
}

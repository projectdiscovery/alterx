package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/gologger"
	fileutil "github.com/projectdiscovery/utils/file"
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
			} else {
				gologger.Error().Msgf("alterx yaml configuration syntax error.\n %v\n.", yaml.FormatError(errx, true, true))
				os.Exit(1)
			}
		}
	}
	if err := validateDir(filepath.Join(getUserHomeDir(), ".config/alterx")); err != nil {
		gologger.Error().Msgf("alterx config dir not found and failed to create got: %v", err)
	}
	if err := os.WriteFile(defaultPermutationCfg, alterx.DefaultPermutationsBin, 0600); err != nil {
		gologger.Error().Msgf("failed to save default config to %v got: %v", defaultPermutationCfg, err)
	}
}

// validateDir checks if dir exists if not creates it
func validateDir(dirPath string) error {
	if fileutil.FolderExists(dirPath) {
		return nil
	}
	return fileutil.CreateFolder(dirPath)
}

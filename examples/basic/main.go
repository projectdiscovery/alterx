package main

import (
	"math"
	"os"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/levels"
)

func main() {
	gologger.DefaultLogger.SetMaxLevel(levels.LevelVerbose)
	opts := &alterx.Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
		MaxSize: math.MaxInt,
	}

	m, err := alterx.New(opts)
	if err != nil {
		gologger.Fatal().Msg(err.Error())
	}
	if err := m.ExecuteWithWriter(os.Stdout); err != nil {
		gologger.Fatal().Msgf("execution failed: %v", err)
	}
}

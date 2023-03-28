package main

import (
	"os"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/gologger"
)

func main() {
	opts := &alterx.Options{
		Domains: []string{"api.scanme.sh", "chaos.scanme.sh", "nuclei.scanme.sh", "cloud.nuclei.scanme.sh"},
	}
	
	m, err := alterx.New(opts)
	if err != nil {
		gologger.Fatal().Msg(err.Error())
	}
	m.ExecuteWithWriter(os.Stdout)
}

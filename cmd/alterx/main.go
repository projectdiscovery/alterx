package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/projectdiscovery/alterx"
)

type Option struct {
	Sample  bool
	Output  string
	Domains string
	Domain  string
	Config  string
	DryRun  bool
}

func main() {
	opts := &Option{}
	flag.BoolVar(&opts.Sample, "sample", false, "creates a sample config.yaml with default settings")
	flag.StringVar(&opts.Output, "output", "", "Output file to write domains")
	flag.StringVar(&opts.Domains, "list", "", "file containing list of domains to use as base")
	flag.StringVar(&opts.Domain, "domain", "", "domain to use as base")
	flag.StringVar(&opts.Config, "config", "", "config file containing payloads and patterns")
	flag.BoolVar(&opts.DryRun, "dn", false, "dry run and only return no of payloads that will be generated (considering all  edgecases)")
	flag.Parse()

	if opts.Sample {
		if err := alterx.GenerateSample("config.yaml"); err != nil {
			panic(err)
		}
		return
	}

	alterOpts := alterx.Options{}

	if opts.Config != "" {
		config, err := alterx.NewConfig(opts.Config)
		if err != nil {
			panic(err)
		}
		if len(config.Patterns) > 0 {
			alterOpts.Patterns = config.Patterns
		}
		if len(config.Payloads) > 0 {
			alterOpts.Payloads = config.Payloads
		}
	}

	if opts.Domain != "" {
		alterOpts.Domains = []string{opts.Domain}
	} else if opts.Domains != "" {
		bin, err := os.ReadFile(opts.Domains)
		if err != nil {
			panic(err)
		}
		list := []string{}
		for _, v := range strings.Split(string(bin), "\n") {
			v = strings.TrimSpace(v)
			if v != "" {
				list = append(list, v)
			}
		}
		alterOpts.Domains = list
	}

	var output io.Writer

	if opts.Output != "" {
		fs, err := os.OpenFile(opts.Output, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		output = fs
		defer fs.Close()
	} else {
		output = os.Stdout
	}

	m, err := alterx.New(&alterOpts)
	if err != nil {
		panic(err)
	}

	if opts.DryRun {
		fmt.Println(m.EstimateCount())
		return
	}

	err = m.ExecuteWithWriter(output)
	if err != nil {
		panic(err)
	}
}

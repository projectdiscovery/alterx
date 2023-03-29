package alterx

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/projectdiscovery/fasttemplate"
	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
)

// Mutator Options
type Options struct {
	// list of Domains to use as base
	Domains []string
	// list of words to use while creating permutations
	// if empty DefaultWordList is used
	Payloads map[string][]string
	// list of pattersn to use while creating permutations
	// if empty DefaultPatterns are used
	Patterns []string
	// Limits output results (0 = no limit)
	Limit int
}

// Mutator
type Mutator struct {
	Options      *Options
	payloadCount int
	Inputs       []*Input // all processed inputs
}

// New creates and returns new mutator instance from options
func New(opts *Options) (*Mutator, error) {
	if len(opts.Domains) == 0 {
		return nil, fmt.Errorf("no input provided to calculate permutations")
	}
	if len(opts.Payloads) == 0 {
		opts.Payloads = map[string][]string{}
		if len(DefaultConfig.Payloads) == 0 {
			return nil, fmt.Errorf("something went wrong, `DefaultWordList` and input wordlist are empty")
		}
		opts.Payloads = DefaultConfig.Payloads
	}
	if len(opts.Patterns) == 0 {
		if len(DefaultConfig.Patterns) == 0 {
			return nil, fmt.Errorf("something went wrong,`DefaultPatters` and input patterns are empty")
		}
		opts.Patterns = DefaultConfig.Patterns
	}
	m := &Mutator{
		Options: opts,
	}
	if err := m.validatePatterns(); err != nil {
		return nil, err
	}
	if err := m.prepareInputs(); err != nil {
		return nil, err
	}
	return m, nil
}

// Execute calculates all permutations using input wordlist and patterns
// and writes them to a string channel
func (m *Mutator) Execute(ctx context.Context) <-chan string {
	results := make(chan string, len(m.Options.Patterns))
	go func() {
		for _, v := range m.Inputs {
			varMap := getSampleMap(v.GetMap(), m.Options.Payloads)
			for _, pattern := range m.Options.Patterns {
				if err := checkMissing(pattern, varMap); err == nil {
					statement := Replace(pattern, v.GetMap())
					select {
					case <-ctx.Done():
						return
					default:
						m.clusterBomb(statement, results)
					}
				} else {
					gologger.Warning().Msgf("variables missing to evaluate pattern `%v` got: %v, skipping", pattern, err.Error())
				}
			}
		}
		close(results)
	}()
	return results
}

// ExecuteWithWriter executes Mutator and writes results directly to type that implements io.Writer interface
func (m *Mutator) ExecuteWithWriter(Writer io.Writer) error {
	if Writer == nil {
		return errorutil.NewWithTag("alterx", "writer destination cannot be nil")
	}
	resChan := m.Execute(context.TODO())
	counter := 0
	for {
		value, ok := <-resChan
		if !ok {
			return nil
		}
		if m.Options.Limit > 0 && counter == m.Options.Limit {
			return nil
		}
		_, err := Writer.Write([]byte(value + "\n"))
		counter++
		if err != nil {
			return err
		}
	}
}

// EstimateCount estimates number of payloads that will be created
// and saves to be used later on with `PayloadCount()` method
func (m *Mutator) EstimateCount() int {
	counter := 0
	for _, v := range m.Inputs {
		varMap := getSampleMap(v.GetMap(), m.Options.Payloads)
		for _, pattern := range m.Options.Patterns {
			if err := checkMissing(pattern, varMap); err == nil {
				// if say patterns is {{sub}}.{{sub1}}-{{word}}.{{root}}
				// and input domain is api.scanme.sh its clear that {{sub1}} here will be empty/missing
				// in such cases `alterx` silently skips that pattern for that specific input
				// this way user can have a long list of patterns but they are only used if all required data is given (much like self-contained templates)
				statement := Replace(pattern, v.GetMap())
				varsUsed := getAllVars(statement)
				if len(varsUsed) == 0 {
					counter += 1
				} else {
					tmpCounter := 1
					for _, word := range varsUsed {
						tmpCounter *= len(m.Options.Payloads[word])
					}
					counter += tmpCounter
				}
			} else {
				gologger.Warning().Msgf("skipping pattern `%v` got %v", pattern, err)
			}
		}
	}
	m.payloadCount = counter
	return counter
}

// clusterBomb calculates all payloads of clusterbomb attack and sends them to result channel
func (m *Mutator) clusterBomb(template string, results chan string) {
	// Early Exit: this is what saves clusterBomb from stackoverflows and reduces
	// n*len(n) iterations and n recursions
	varsUsed := getAllVars(template)
	if len(varsUsed) == 0 {
		// clusterBomb is not required
		// just send existing template as result and exit
		results <- template
		return
	}
	payloadSet := map[string][]string{}
	// instead of sending all payloads only send payloads that are used
	// in template/statement
	for _, v := range varsUsed {
		payloadSet[v] = []string{}
		payloadSet[v] = append(payloadSet[v], m.Options.Payloads[v]...)
	}
	payloads := NewIndexMap(payloadSet)
	// in clusterBomb attack no of payloads generated are
	// len(first_set)*len(second_set)*len(third_set)....
	callbackFunc := func(varMap map[string]interface{}) {
		results <- Replace(template, varMap)
	}
	ClusterBomb(payloads, callbackFunc, []string{})
}

// prepares input and patterns and calculates estimations
func (m *Mutator) prepareInputs() error {
	errors := []string{}
	// prepare input
	allInputs := []*Input{}
	for _, v := range m.Options.Domains {
		i, err := NewInput(v)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		allInputs = append(allInputs, i)
	}
	m.Inputs = allInputs
	if len(errors) > 0 {
		return errorutil.NewWithTag("alterx", "%v", strings.Join(errors, " : "))
	}
	return nil
}

// validates all patterns by compiling them
func (m *Mutator) validatePatterns() error {
	for _, v := range m.Options.Patterns {
		// check if all placeholders are correctly used and are valid
		if _, err := fasttemplate.NewTemplate(v, ParenthesisOpen, ParenthesisClose); err != nil {
			return err
		}
	}
	return nil
}

// PayloadCount returns total estimated payloads count
func (m *Mutator) PayloadCount() int {
	return m.payloadCount
}

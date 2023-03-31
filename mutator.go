package alterx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/projectdiscovery/fasttemplate"
	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
	sliceutil "github.com/projectdiscovery/utils/slice"
)

var (
	extractNumbers   = regexp.MustCompile(`[0-9]+`)
	extractWords     = regexp.MustCompile(`[a-zA-Z0-9]+`)
	extractWordsOnly = regexp.MustCompile(`[a-zA-Z]{3,}`)
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
	// Enrich when true alterx extra possible words from input
	// and adds them to default payloads word,number
	Enrich bool
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
	// purge duplicates if any
	for k, v := range opts.Payloads {
		dedupe := sliceutil.Dedupe(v)
		if len(v) != len(dedupe) {
			gologger.Warning().Msgf("%v duplicate payloads found in %v. purging them..", len(v)-len(dedupe), k)
			opts.Payloads[k] = dedupe
		}
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
	if opts.Enrich {
		m.enrichPayloads()
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
	ch := m.Execute(context.Background())
	for {
		_, ok := <-ch
		if !ok {
			break
		}
		counter++
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
		for _, word := range m.Options.Payloads[v] {
			if !strings.Contains(template, word) {
				// skip all words that are already present in template/sub , it is highly unlikely
				// we will ever find api-api.example.com
				payloadSet[v] = append(payloadSet[v], word)
			}
		}
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

func (m *Mutator) enrichPayloads() {
	var temp bytes.Buffer
	for _, v := range m.Inputs {
		temp.WriteString(v.Sub + " ")
		if len(v.MultiLevel) > 0 {
			temp.WriteString(strings.Join(v.MultiLevel, " "))
		}
	}
	numbers := extractNumbers.FindAllString(temp.String(), -1)
	extraWords := extractWords.FindAllString(temp.String(), -1)
	extraWordsOnly := extractWordsOnly.FindAllString(temp.String(), -1)
	if len(extraWordsOnly) > 0 {
		extraWords = append(extraWords, extraWordsOnly...)
		extraWords = sliceutil.Dedupe(extraWords)
	}

	if len(m.Options.Payloads["word"]) > 0 {
		extraWords = append(extraWords, m.Options.Payloads["word"]...)
		m.Options.Payloads["word"] = sliceutil.Dedupe(extraWords)
	}
	if len(m.Options.Payloads["number"]) > 0 {
		numbers = append(numbers, m.Options.Payloads["number"]...)
		m.Options.Payloads["number"] = sliceutil.Dedupe(numbers)
	}
}

// PayloadCount returns total estimated payloads count
func (m *Mutator) PayloadCount() int {
	if m.payloadCount == 0 {
		m.EstimateCount()
	}
	return m.payloadCount
}

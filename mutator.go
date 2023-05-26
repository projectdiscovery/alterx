package alterx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/projectdiscovery/fasttemplate"
	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
	sliceutil "github.com/projectdiscovery/utils/slice"
)

var (
	extractNumbers   = regexp.MustCompile(`[0-9]+`)
	extractWords     = regexp.MustCompile(`[a-zA-Z0-9]+`)
	extractWordsOnly = regexp.MustCompile(`[a-zA-Z]{3,}`)
	DedupeResults    = true // Dedupe all results (default: true)
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
	// MaxSize limits output data size
	MaxSize int
}

// Mutator
type Mutator struct {
	Options      *Options
	payloadCount int
	Inputs       []*Input // all processed inputs
	timeTaken    time.Duration
	// internal or unexported variables
	maxkeyLenInBytes int
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
	var maxBytes int
	if DedupeResults {
		count := m.EstimateCount()
		maxBytes = count * m.maxkeyLenInBytes
	}

	results := make(chan string, len(m.Options.Patterns))
	go func() {
		now := time.Now()
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
					gologger.Warning().Msgf("%v : failed to evaluate pattern %v. skipping", err.Error(), pattern)
				}
			}
		}
		m.timeTaken = time.Since(now)
		close(results)
	}()

	if DedupeResults {
		// drain results
		d := NewDedupe(results, maxBytes)
		d.Drain()
		return d.GetResults()
	}
	return results
}

// ExecuteWithWriter executes Mutator and writes results directly to type that implements io.Writer interface
func (m *Mutator) ExecuteWithWriter(Writer io.Writer) error {
	if Writer == nil {
		return errorutil.NewWithTag("alterx", "writer destination cannot be nil")
	}
	resChan := m.Execute(context.TODO())
	m.payloadCount = 0
	maxFileSize := m.Options.MaxSize
	for {
		value, ok := <-resChan
		if !ok {
			gologger.Info().Msgf("Generated %v permutations in %v", m.payloadCount, m.Time())
			return nil
		}
		if m.Options.Limit > 0 && m.payloadCount == m.Options.Limit {
			// we can't early exit, due to abstraction we have to conclude the elaboration to drain all dedupers
			continue
		}
		if maxFileSize <= 0 {
			// drain all dedupers when max-file size reached
			continue
		}
		outputData := []byte(value + "\n")
		if len(outputData) > maxFileSize {
			// truncate output data if it exceeds maxFileSize
			outputData = outputData[:maxFileSize]
		}

		n, err := Writer.Write(outputData)
		if err != nil {
			return err
		}
		// update maxFileSize limit after each write
		maxFileSize -= n
		m.payloadCount++
	}
}

// EstimateCount estimates number of payloads that will be created
// without actually executing/creating permutations
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
				bin := unsafeToBytes(statement)
				if m.maxkeyLenInBytes < len(bin) {
					m.maxkeyLenInBytes = len(bin)
				}
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
			}
		}
	}
	return counter
}

// DryRun executes payloads without storing and returns number of payloads created
// this value is also stored in variable and can be accessed via getter `PayloadCount`
func (m *Mutator) DryRun() int {
	m.payloadCount = 0
	err := m.ExecuteWithWriter(io.Discard)
	if err != nil {
		gologger.Error().Msgf("alterx: got %v", err)
	}
	return m.payloadCount
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
	var errors []string
	// prepare input
	var allInputs []*Input
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
		gologger.Warning().Msgf("errors found when preparing inputs got: %v : skipping errored inputs", strings.Join(errors, " : "))
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

// enrichPayloads extract possible words and adds them to default wordlist
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
		return m.EstimateCount()
	}
	return m.payloadCount
}

// Time returns time taken to create permutations in seconds
func (m *Mutator) Time() string {
	return fmt.Sprintf("%.4fs", m.timeTaken.Seconds())
}

package alterx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/projectdiscovery/alterx/internal/patternmining"
	"github.com/projectdiscovery/fasttemplate"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/utils/dedupe"
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
	// Mode specifies the operation mode: "default" (default), "discover" (pattern mining only), "both" (combined)
	// Empty string defaults to "default" mode for backwards compatibility
	Mode string
	// MinDistance is the minimum levenshtein distance for clustering
	MinDistance int
	// MaxDistance is the maximum levenshtein distance for clustering
	MaxDistance int
	// PatternThreshold is the threshold for pattern quality filtering
	PatternThreshold int
	// QualityRatio is the maximum ratio of synthetic/observed for pattern validation
	QualityRatio float64
	// NgramsLimit limits the number of n-grams to process (0 = no limit)
	NgramsLimit int
	// MaxLength is the maximum pattern length
	MaxLength int
}

func (v *Options) Validate() error {
	// Default to "default" mode if not specified (backwards compatibility)
	if v.Mode == "" {
		v.Mode = "default"
	}
	if v.Mode != "default" && v.Mode != "discover" && v.Mode != "both" {
		return fmt.Errorf("invalid mode: %s (must be 'default', 'discover', or 'both')", v.Mode)
	}
	// auto fill default values
	if v.MinDistance == 0 {
		v.MinDistance = 2
	}
	if v.MaxDistance == 0 {
		v.MaxDistance = 10
	}
	if v.QualityRatio == 0 {
		v.QualityRatio = 25
	}
	if v.PatternThreshold == 0 {
		v.PatternThreshold = 500
	}
	if v.NgramsLimit == 0 {
		v.NgramsLimit = 0
	}
	if v.MaxLength == 0 {
		v.MaxLength = 1000
	}
	return nil
}

// Mutator
type Mutator struct {
	Options      *Options
	payloadCount int
	Inputs       []*Input // all processed inputs
	timeTaken    int64    // atomic access only (stores nanoseconds as int64)
	// internal or unexported variables
	maxkeyLenInBytes int
	rootDomain       string
	miner            *patternmining.Miner
	miningResult     *patternmining.Result
}

// New creates and returns new mutator instance from options
func New(opts *Options) (*Mutator, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	m := &Mutator{
		Options: opts,
	}

	if opts.Mode == "discover" || opts.Mode == "both" {

		// run validation and save root domain in case of discover mode
		rootDomain, err := getNValidateRootDomain(m.Options.Domains)
		if err != nil {
			return nil, err
		}
		m.rootDomain = rootDomain

		miner := patternmining.NewMiner(&patternmining.Options{
			Domains:          opts.Domains,
			Target:           m.rootDomain,
			MinDistance:      m.Options.MinDistance,
			MaxDistance:      m.Options.MaxDistance,
			PatternThreshold: m.Options.PatternThreshold,
			QualityRatio:     m.Options.QualityRatio,
			MaxLength:        m.Options.MaxLength,
			NgramsLimit:      m.Options.NgramsLimit,
		})
		m.miner = miner

	}

	if opts.Mode == "default" || opts.Mode == "both" {
		// validate payloads and patterns for default and both modes
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

		if err := m.validatePatterns(); err != nil {
			return nil, err
		}
		if err := m.prepareInputs(); err != nil {
			return nil, err
		}
		if opts.Enrich {
			m.enrichPayloads()
		}
	}

	return m, nil
}

// SaveRules saves pattern mining result to a file
func (m *Mutator) SaveRules(filename string) error {
	if m.miner == nil {
		return fmt.Errorf("pattern mining is not enabled")
	}
	if m.miningResult == nil {
		return fmt.Errorf("pattern mining result is not available")
	}
	if err := m.miner.SaveRules(m.miningResult, filename); err != nil {
		return err
	}
	gologger.Info().Msgf("Saved %d patterns to %s", len(m.miningResult.Patterns), filename)
	return nil
}

// Execute calculates all permutations using input wordlist and patterns
// and writes them to a string channel
func (m *Mutator) Execute(ctx context.Context) <-chan string {
	var maxBytes int
	if DedupeResults {
		count := m.EstimateCount()
		maxBytes = count * m.maxkeyLenInBytes
	}
	results := make(chan string, 100)
	wg := &sync.WaitGroup{}

	now := time.Now()

	if m.miner != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Run pattern mining
			gologger.Info().Msgf("Running pattern mining on %d domains...", len(m.Options.Domains))
			result, err := m.miner.Mine()
			if err != nil {
				gologger.Error().Msgf("pattern mining failed: %v", err)
				return
			}
			m.miningResult = result
			gologger.Info().Msgf("Discovered %d patterns from input domains", len(result.Patterns))

			var seen = make(map[string]bool)
			for _, sub := range m.Options.Domains {
				seen[sub] = true
			}
			// In discover mode, only use mined patterns
			generated := m.miner.GenerateFromPatterns(m.miningResult.Patterns)
			for _, subdomain := range generated {
				if seen[subdomain] {
					// skip the input subdomains
					// regulator algo has tendency to generate input subdomains as patterns
					continue
				}
				seen[subdomain] = true
				results <- subdomain
			}
		}()
	}

	if len(m.Inputs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	go func() {
		wg.Wait()
		close(results)
		atomic.StoreInt64(&m.timeTaken, int64(time.Since(now)))
	}()

	if DedupeResults {
		// drain results
		d := dedupe.NewDedupe(results, maxBytes)
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
			gologger.Info().Msgf("Generated %v unique subdomains in %v", m.payloadCount, m.Time())
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

		if strings.HasPrefix(value, "-") {
			continue
		}

		outputData := []byte(value + "\n")
		if len(outputData) > maxFileSize {
			maxFileSize = 0
			continue
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
	if m.miner != nil && m.miningResult != nil {
		counter += int(m.miner.EstimateCount(m.miningResult.Patterns))
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
	leftmostSub := strings.Split(template, ".")[0]
	for _, v := range varsUsed {
		payloadSet[v] = []string{}
		for _, word := range m.Options.Payloads[v] {
			if !strings.HasPrefix(leftmostSub, word) && !strings.HasSuffix(leftmostSub, word) {
				// skip all words that are already present in leftmost sub , it is highly unlikely
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
	var maxInputsToProcess int
	var maxWordsToExtract int
	var maxNumbersToExtract int

	inputs := m.Inputs

	// NOTE(dwisiswant0): Scale the number of inputs to process based on limit
	// or max-size options to generate additional payloads.
	if m.Options.Limit > 0 {
		maxInputsToProcess = m.Options.Limit
		maxWordsToExtract = m.Options.Limit
		maxNumbersToExtract = m.Options.Limit
	}
	if m.Options.MaxSize > 0 && m.Options.MaxSize <= len(inputs) {
		maxInputsToProcess = m.Options.MaxSize
		maxWordsToExtract = m.Options.MaxSize
		maxNumbersToExtract = m.Options.MaxSize
	}

	if len(inputs) > maxInputsToProcess && maxInputsToProcess > 0 {
		inputs = inputs[:maxInputsToProcess]
	}

	for _, v := range inputs {
		temp.WriteString(v.Sub + " ")
		if len(v.MultiLevel) > 0 {
			temp.WriteString(strings.Join(v.MultiLevel, " "))
		}
	}

	numbers := extractNumbers.FindAllString(temp.String(), -1)
	extraWords := extractWords.FindAllString(temp.String(), -1)
	extraWordsOnly := extractWordsOnly.FindAllString(temp.String(), -1)

	var filteredWords []string
	minWordLength := 3
	for _, word := range extraWords {
		if len(word) >= minWordLength {
			filteredWords = append(filteredWords, word)
		}
	}
	extraWords = filteredWords

	if len(numbers) > maxNumbersToExtract && maxNumbersToExtract > 0 {
		numbers = numbers[:maxNumbersToExtract]
	}

	if len(extraWordsOnly) > 0 {
		extraWords = append(extraWords, extraWordsOnly...)
		extraWords = sliceutil.Dedupe(extraWords)
	}

	if len(extraWords) > maxWordsToExtract && maxWordsToExtract > 0 {
		extraWords = extraWords[:maxWordsToExtract]
	}

	if len(m.Options.Payloads["word"]) > 0 {
		extraWords = append(extraWords, m.Options.Payloads["word"]...)
		m.Options.Payloads["word"] = sliceutil.Dedupe(extraWords)
	} else {
		m.Options.Payloads["word"] = extraWords
	}

	if len(m.Options.Payloads["number"]) > 0 {
		numbers = append(numbers, m.Options.Payloads["number"]...)
		m.Options.Payloads["number"] = sliceutil.Dedupe(numbers)
	} else {
		m.Options.Payloads["number"] = numbers
	}

	gologger.Debug().Msgf("Enrichment added %d words and %d numbers",
		len(m.Options.Payloads["word"]), len(m.Options.Payloads["number"]))
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
	duration := time.Duration(atomic.LoadInt64(&m.timeTaken))
	return fmt.Sprintf("%.4fs", duration.Seconds())
}

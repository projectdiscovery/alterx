package alterx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/projectdiscovery/alterx/internal/inducer"
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
	// Mode specifies pattern generation mode: both, inferred, default
	Mode string
	// MaxSize limits output data size
	MaxSize int
	// LearnedPatterns stores learned patterns with their payloads (can be set externally to avoid re-learning)
	LearnedPatterns []*LearnedPattern
}

// Mutator
type Mutator struct {
	Options         *Options
	payloadCount    int
	Inputs          []*Input           // all processed inputs
	LearnedPatterns []*LearnedPattern  // learned patterns with their specific payloads
	timeTaken       time.Duration
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
		// Apply pattern selection based on mode
		switch opts.Mode {
		case "default":
			// Use only default patterns
			opts.Patterns = DefaultConfig.Patterns
		case "inferred":
			// Use only inferred patterns from input subdomain analysis
			// Pattern learning is root-agnostic (learns subdomain structures)
			inducer := NewPatternInducer(opts.Domains, 2)
			learnedPatterns, err := inducer.InferPatterns()
			if err != nil {
				gologger.Warning().Msgf("Pattern induction failed: %v. Falling back to default patterns.", err)
				opts.Patterns = DefaultConfig.Patterns
				opts.LearnedPatterns = []*LearnedPattern{}
			} else {
				// Extract template strings from learned patterns
				templates := make([]string, 0, len(learnedPatterns))
				for _, p := range learnedPatterns {
					templates = append(templates, p.Template)
				}
				if len(templates) == 0 {
					gologger.Warning().Msg("No patterns inferred from input. Falling back to default patterns.")
					opts.Patterns = DefaultConfig.Patterns
					opts.LearnedPatterns = []*LearnedPattern{}
				} else {
					opts.Patterns = templates
					// Store learned patterns with their specific payloads
					opts.LearnedPatterns = learnedPatterns
				}
			}
		case "both", "":
			// Use both default and inferred patterns
			// Pattern learning is root-agnostic (learns subdomain structures)
			inducer := NewPatternInducer(opts.Domains, 2)
			learnedPatterns, err := inducer.InferPatterns()
			if err != nil {
				gologger.Warning().Msgf("Pattern induction failed: %v. Using only default patterns.", err)
				opts.Patterns = DefaultConfig.Patterns
				opts.LearnedPatterns = []*LearnedPattern{}
			} else {
				// Extract template strings from learned patterns
				templates := make([]string, 0, len(learnedPatterns))
				for _, p := range learnedPatterns {
					templates = append(templates, p.Template)
				}
				if len(templates) == 0 {
					gologger.Verbose().Msg("No patterns inferred. Using only default patterns.")
					opts.Patterns = DefaultConfig.Patterns
					opts.LearnedPatterns = []*LearnedPattern{}
				} else {
					// Merge inferred patterns with default patterns
					opts.Patterns = append(templates, DefaultConfig.Patterns...)
					// Dedupe to remove any overlapping patterns
					opts.Patterns = sliceutil.Dedupe(opts.Patterns)
					gologger.Verbose().Msgf("Using %d patterns (%d inferred + %d default)",
						len(opts.Patterns), len(templates), len(DefaultConfig.Patterns))
					// Store learned patterns with their specific payloads
					opts.LearnedPatterns = learnedPatterns
				}
			}
		default:
			// Fallback to default patterns
			opts.Patterns = DefaultConfig.Patterns
		}
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
		Options:         opts,
		LearnedPatterns: opts.LearnedPatterns, // Copy learned patterns from options
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
			for _, pattern := range m.Options.Patterns {
				// Get pattern-specific payloads (learned patterns have their own)
				patternPayloads := m.getPatternPayloads(pattern)

				// Create sample map for validation with pattern-specific payloads
				varMap := getSampleMap(v.GetMap(), patternPayloads)

				if err := checkMissing(pattern, varMap); err == nil {
					statement := Replace(pattern, v.GetMap())
					select {
					case <-ctx.Done():
						return
					default:
						// Pass pattern-specific payloads to clusterBomb
						m.clusterBomb(statement, results, patternPayloads)
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
	// Use background context since this is a public API without context parameter
	// to maintain backward compatibility
	resChan := m.Execute(context.Background())
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

		// Validate subdomain and filter invalid combinations from omit functionality
		if !isValidSubdomain(value) {
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
		for _, pattern := range m.Options.Patterns {
			// Get pattern-specific payloads (learned patterns have their own)
			patternPayloads := m.getPatternPayloads(pattern)

			// Create sample map with pattern-specific payloads
			varMap := getSampleMap(v.GetMap(), patternPayloads)

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
						// Use pattern-specific payloads for counting
						if payloadList, exists := patternPayloads[word]; exists {
							tmpCounter *= len(payloadList)
						}
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
// payloads parameter allows using pattern-specific payloads instead of global payloads
func (m *Mutator) clusterBomb(template string, results chan string, payloads map[string][]string) {
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
		// Use the provided payloads map (could be global or pattern-specific)
		if payloadList, exists := payloads[v]; exists {
			for _, word := range payloadList {
				// Include empty string if present (for optional variables from pattern enrichment)
				// Skip non-empty words that are already present in leftmost sub to avoid api-api.example.com
				if word == "" || (!strings.HasPrefix(leftmostSub, word) && !strings.HasSuffix(leftmostSub, word)) {
					payloadSet[v] = append(payloadSet[v], word)
				}
			}
		}
	}
	payloadMap := NewIndexMap(payloadSet)
	// in clusterBomb attack no of payloads generated are
	// len(first_set)*len(second_set)*len(third_set)....
	callbackFunc := func(varMap map[string]interface{}) {
		results <- Replace(template, varMap)
	}
	ClusterBomb(payloadMap, callbackFunc, []string{})
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

// getKeys returns keys of a map as a slice
func getKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// isValidSubdomain checks if a generated subdomain is valid
// Filters out malformed combinations from omit functionality
func isValidSubdomain(value string) bool {
	// Filter subdomains starting with delimiters
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "_") || strings.HasPrefix(value, ".") {
		return false
	}

	// Filter consecutive delimiters
	if strings.Contains(value, "--") || strings.Contains(value, "__") ||
	   strings.Contains(value, "-.") || strings.Contains(value, "._") ||
	   strings.Contains(value, "-_") || strings.Contains(value, "_-") ||
	   strings.Contains(value, "..") {
		return false
	}

	// Filter duplicate consecutive words (e.g., api-api, dev-dev)
	// Split by dots to check each subdomain level
	parts := strings.Split(value, ".")
	if len(parts) > 0 {
		// Check the subdomain part (before root domain)
		subdomain := parts[0]

		// Split by common delimiters
		tokens := strings.FieldsFunc(subdomain, func(r rune) bool {
			return r == '-' || r == '_'
		})

		// Check for consecutive duplicate tokens
		for i := 0; i < len(tokens)-1; i++ {
			if tokens[i] != "" && tokens[i] == tokens[i+1] {
				return false
			}
		}
	}

	// Filter empty components (e.g., "-.example.com" has empty component before dash)
	if strings.HasPrefix(value, "-.") || strings.HasPrefix(value, "_.") {
		return false
	}

	return true
}

// payloadToMap converts a payload interface{} to map[string]interface{}
// This is needed to handle NumberRange pointers from pattern induction
func payloadToMap(payload interface{}) (map[string]interface{}, bool) {
	// Check if it's a *inducer.NumberRange
	if nr, ok := payload.(*inducer.NumberRange); ok {
		return map[string]interface{}{
			"start":  nr.Start,
			"end":    nr.End,
			"format": nr.Format,
			"step":   nr.Step,
			"type":   nr.Type,
		}, true
	}
	return nil, false
}

// expandNumberRange converts a NumberRange structure to a slice of formatted strings
// Uses the Format field with fmt.Sprintf to generate numbers from Start to End
func expandNumberRange(nr interface{}) []string {
	// Handle type assertion - nr could be map[string]interface{} from YAML
	var result []string

	// Try to access as a map (from YAML unmarshaling)
	if nrMap, ok := nr.(map[string]interface{}); ok {
		start, _ := nrMap["start"].(int)
		end, _ := nrMap["end"].(int)
		format, _ := nrMap["format"].(string)
		step, _ := nrMap["step"].(int)

		if step == 0 {
			step = 1
		}
		if format == "" {
			format = "%d"
		}

		for i := start; i <= end; i += step {
			result = append(result, fmt.Sprintf(format, i))
		}
	}

	return result
}

// getPatternPayloads returns the payloads for a specific pattern
// If the pattern is a learned pattern, returns its specific payloads
// Otherwise returns the global payloads
func (m *Mutator) getPatternPayloads(pattern string) map[string][]string {
	// Check if this pattern is a learned pattern
	for _, lp := range m.LearnedPatterns {
		if lp.Template == pattern {
			// Convert learned pattern payloads to map[string][]string format
			result := make(map[string][]string)
			for varName, payload := range lp.Payloads {
				switch v := payload.(type) {
				case []string:
					// Word payloads - already enriched by inducer if optional
					result[varName] = v
				case []interface{}:
					// Convert []interface{} to []string
					strSlice := make([]string, 0, len(v))
					for _, item := range v {
						if str, ok := item.(string); ok {
							strSlice = append(strSlice, str)
						}
					}
					result[varName] = strSlice
				case map[string]interface{}:
					// This is a NumberRange as map (from YAML unmarshaling)
					// Just expand it normally - enrichment is handled in inducer
					result[varName] = expandNumberRange(v)
				default:
					// Try to convert to map[string]interface{} for NumberRange
					// This handles the case where payload is a struct pointer from enrichment
					if mapPayload, ok := payloadToMap(payload); ok {
						result[varName] = expandNumberRange(mapPayload)
					} else {
						gologger.Warning().Msgf("Unknown payload type for variable %s in pattern %s (type: %T)", varName, pattern, v)
					}
				}
			}
			return result
		}
	}

	// Not a learned pattern - return global payloads
	return m.Options.Payloads
}

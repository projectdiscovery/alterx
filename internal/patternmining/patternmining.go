package patternmining

// Pattern Mining for Subdomain Discovery
// This is a Go port of Regulator by @cramppet (https://github.com/cramppet/regulator)
// Regulator uses edit-distance clustering and regex generalization to discover
// subdomain patterns from observed data.

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/projectdiscovery/alterx/internal/dank"
	"github.com/projectdiscovery/gologger"
)

var (
	reNum       = regexp.MustCompile(`[0-9]+`)
	reNumeric   = regexp.MustCompile(`^[0-9]+$`)
	reDoubleDot = regexp.MustCompile(`\.{2,}`)
)

// Options contains pattern mining configuration
type Options struct {
	// Input domains to analyze
	Domains []string
	// Target domain (e.g., "example.com")
	Target string
	// MinDistance is the minimum levenshtein distance for clustering
	MinDistance int
	// MaxDistance is the maximum levenshtein distance for clustering
	MaxDistance int
	// PatternThreshold is the threshold for pattern quality filtering
	PatternThreshold int
	// QualityRatio is the maximum ratio of synthetic/observed for pattern validation
	QualityRatio float64
	// MaxLength is the maximum pattern length
	MaxLength int
	// NgramsLimit limits the number of n-grams to process (0 = no limit)
	NgramsLimit int
}

// Result contains discovered patterns and metadata
type Result struct {
	Patterns []string
	Metadata map[string]map[string]interface{}
}

// PatternMetadata contains metadata about a discovered pattern
type PatternMetadata struct {
	Mode        string   `json:"mode"`
	K           *int     `json:"k,omitempty"`
	Ngram       string   `json:"ngram,omitempty"`
	Prefix      string   `json:"prefix,omitempty"`
	ClusterSize int      `json:"cluster_size"`
	Nwords      int      `json:"nwords"`
	Ratio       float64  `json:"ratio"`
	Members     []string `json:"members,omitempty"`
}

// RuleEntry represents a pattern with its metadata
type RuleEntry struct {
	Pattern string           `json:"pattern"`
	Meta    *PatternMetadata `json:"meta"`
}

// StepGroup represents a mining step with its discovered patterns
type StepGroup struct {
	Step    string       `json:"step"`
	Entries []*RuleEntry `json:"entries"`
}

// RulesOutput is the top-level structure for saved rules
type RulesOutput struct {
	Steps []*StepGroup `json:"steps"`
}

// Miner performs pattern mining on subdomain data
type Miner struct {
	opts *Options
	memo map[string]int // memoized levenshtein distances
}

// NewMiner creates a new pattern miner
func NewMiner(opts *Options) *Miner {
	return &Miner{
		opts: opts,
		memo: make(map[string]int),
	}
}

// Mine discovers patterns from input domains
func (m *Miner) Mine() (*Result, error) {
	if len(m.opts.Domains) == 0 {
		return nil, fmt.Errorf("no domains provided for pattern mining")
	}
	if m.opts.Target == "" {
		return nil, fmt.Errorf("target domain not specified")
	}

	gologger.Info().Msgf("Starting pattern mining on %d observations", len(m.opts.Domains))

	// Validate and filter domains
	knownHosts := m.validateDomains()
	if len(knownHosts) == 0 {
		return nil, fmt.Errorf("no valid domains after filtering")
	}

	gologger.Verbose().Msgf("Building pairwise distance table...")
	m.buildDistanceTable(knownHosts)

	newRules := make(map[string]map[string]interface{})

	// Phase 1: No enforced prefix - edit distance clustering
	gologger.Verbose().Msgf("Phase 1: Edit distance clustering...")
	for k := m.opts.MinDistance; k < m.opts.MaxDistance; k++ {
		closures := m.editClosures(knownHosts, k)
		for _, closure := range closures {
			if len(closure) <= 1 {
				continue
			}
			pattern, _ := m.closureToRegex(false, closure)
			if len(pattern) > m.opts.MaxLength {
				continue
			}
			subdomainPattern := strings.TrimSuffix(pattern, "."+m.opts.Target)
			if m.isGoodRule(subdomainPattern, len(closure)) {
				nwords := dank.NewDankEncoder(m.preparePattern(subdomainPattern), 256).NumWords(1, 256)
				ratio := float64(0)
				if len(closure) > 0 {
					ratio = float64(nwords) / float64(len(closure))
				}
				if _, exists := newRules[pattern]; !exists {
					newRules[pattern] = map[string]interface{}{
						"mode":         "no_prefix",
						"k":            k,
						"cluster_size": len(closure),
						"nwords":       nwords,
						"ratio":        ratio,
						"members":      closure,
					}
				}
			}
		}
	}

	// Phase 2: N-gram prefix clustering
	gologger.Verbose().Msgf("Phase 2: N-gram prefix clustering...")
	ngrams := m.generateNgrams(m.opts.NgramsLimit)

	for _, ngram := range ngrams {
		keys := m.prefixKeys(knownHosts, ngram)
		if len(keys) == 0 {
			continue
		}

		// Try ngram as simple prefix
		rUn, _ := m.closureToRegex(false, keys)
		rEsc, _ := m.closureToRegex(true, keys)
		if m.isGoodRule(rEsc, len(keys)) {
			nwords := dank.NewDankEncoder(m.preparePattern(rEsc), 256).NumWords(1, 256)
			ratio := float64(0)
			if len(keys) > 0 {
				ratio = float64(nwords) / float64(len(keys))
			}
			if _, exists := newRules[rUn]; !exists {
				newRules[rUn] = map[string]interface{}{
					"mode":         "ngram",
					"ngram":        ngram,
					"cluster_size": len(keys),
					"nwords":       nwords,
					"ratio":        ratio,
					"members":      keys,
				}
			}
		}

		// Try with first token as prefix
		prefixes := m.extractFirstTokens(keys)
		last := ""
		for _, prefix := range prefixes {
			keys2 := m.prefixKeys(knownHosts, prefix)
			rUn, _ := m.closureToRegex(false, keys2)
			rEsc, _ := m.closureToRegex(true, keys2)

			if m.isGoodRule(rEsc, len(keys2)) {
				// Avoid redundant prefixes
				if last == "" || !strings.HasPrefix(prefix, last) {
					last = prefix
				} else {
					continue
				}

				nwords := dank.NewDankEncoder(m.preparePattern(rEsc), 256).NumWords(1, 256)
				ratio := float64(0)
				if len(keys2) > 0 {
					ratio = float64(nwords) / float64(len(keys2))
				}
				if _, exists := newRules[rUn]; !exists {
					newRules[rUn] = map[string]interface{}{
						"mode":         "ngram_prefix",
						"ngram":        ngram,
						"prefix":       prefix,
						"cluster_size": len(keys2),
						"nwords":       nwords,
						"ratio":        ratio,
						"members":      keys2,
					}
				}
			}

			// Apply edit distance clustering within prefix group
			if len(prefix) > 1 {
				for kk := m.opts.MinDistance; kk < m.opts.MaxDistance; kk++ {
					closures := m.editClosures(keys2, kk)
					for _, closure := range closures {
						rUn, _ := m.closureToRegex(false, closure)
						rEsc, _ := m.closureToRegex(true, closure)

						if m.isGoodRule(rEsc, len(closure)) {
							nwords := dank.NewDankEncoder(m.preparePattern(rEsc), 256).NumWords(1, 256)
							ratio := float64(0)
							if len(closure) > 0 {
								ratio = float64(nwords) / float64(len(closure))
							}
							if _, exists := newRules[rUn]; !exists {
								newRules[rUn] = map[string]interface{}{
									"mode":         "ngram_prefix",
									"ngram":        ngram,
									"prefix":       prefix,
									"k":            kk,
									"cluster_size": len(closure),
									"nwords":       nwords,
									"ratio":        ratio,
									"members":      closure,
								}
							}
						}
					}
				}
			}
		}
	}

	patterns := make([]string, 0, len(newRules))
	for pattern := range newRules {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)

	gologger.Info().Msgf("Discovered %d unique patterns", len(patterns))

	return &Result{
		Patterns: patterns,
		Metadata: newRules,
	}, nil
}

// validateDomains filters and validates input domains
func (m *Miner) validateDomains() []string {
	var knownHosts []string
	for _, host := range m.opts.Domains {
		host = strings.TrimSpace(host)
		if host == "" || host == m.opts.Target {
			continue
		}
		if !strings.HasSuffix(host, "."+m.opts.Target) {
			gologger.Verbose().Msgf("Rejecting malformed input: %s", host)
			continue
		}
		// Validate tokenization
		tokens := m.tokenize([]string{host})
		if len(tokens) == 0 || len(tokens[0]) == 0 || len(tokens[0][0]) == 0 {
			gologger.Verbose().Msgf("Rejecting malformed input: %s", host)
			continue
		}
		knownHosts = append(knownHosts, host)
	}
	return m.removeDuplicatesAndSort(knownHosts)
}

// buildDistanceTable computes all pairwise levenshtein distances
func (m *Miner) buildDistanceTable(hosts []string) {
	for i := 0; i < len(hosts); i++ {
		for j := i; j < len(hosts); j++ {
			d := levenshtein(hosts[i], hosts[j])
			key := getKey(hosts[i], hosts[j])
			m.memo[key] = d
		}
	}
}

// generateNgrams creates unigrams and bigrams
func (m *Miner) generateNgrams(limit int) []string {
	dnsChars := "abcdefghijklmnopqrstuvwxyz0123456789._-"
	ngrams := []string{}

	// Unigrams
	for _, c := range dnsChars {
		ngrams = append(ngrams, string(c))
	}

	// Bigrams
	for _, c1 := range dnsChars {
		for _, c2 := range dnsChars {
			ngrams = append(ngrams, string(c1)+string(c2))
		}
	}

	sort.Strings(ngrams)

	if limit > 0 && len(ngrams) > limit {
		return ngrams[:limit]
	}
	return ngrams
}

// SaveRules writes discovered patterns and metadata to a single JSON file
func (m *Miner) SaveRules(result *Result, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Group patterns by step with metadata
	grouped := m.groupRulesByStep(result.Metadata)

	// Write as compact JSON
	enc := json.NewEncoder(f)
	return enc.Encode(grouped)
}

// GenerateFromPatterns generates subdomains from discovered patterns
func (m *Miner) GenerateFromPatterns(patterns []string) []string {
	var results []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		subdomainPattern := strings.TrimSuffix(pattern, "."+m.opts.Target)
		if len(subdomainPattern) == 0 {
			continue
		}

		// Calculate fixed length for generation
		tempEncoder := dank.NewDankEncoder(m.preparePattern(subdomainPattern), 1)
		fixedSlice := tempEncoder.NumStates() - 2
		if fixedSlice < 0 {
			fixedSlice = 0
		}

		encoder := dank.NewDankEncoder(m.preparePattern(subdomainPattern), fixedSlice)
		generated := encoder.GenerateAtFixedLength(fixedSlice)

		for _, item := range generated {
			fullHost := item + "." + m.opts.Target
			// Remove double dots
			fullHost = reDoubleDot.ReplaceAllString(fullHost, ".")
			if !seen[fullHost] && fullHost != "" {
				seen[fullHost] = true
				results = append(results, fullHost)
			}
		}
	}

	sort.Strings(results)
	return results
}

// Helper functions

func (m *Miner) isGoodRule(regex string, nkeys int) bool {
	encoder := dank.NewDankEncoder(m.preparePattern(regex), 256)
	nwords := encoder.NumWords(1, 256)
	if nwords < int64(m.opts.PatternThreshold) {
		return true
	}
	if nkeys == 0 {
		return false
	}
	return float64(nwords)/float64(nkeys) < m.opts.QualityRatio
}

func (m *Miner) preparePattern(p string) string {
	return escapeForDankEncoder(p)
}

func (m *Miner) removeDuplicatesAndSort(hosts []string) []string {
	seen := make(map[string]bool)
	for _, h := range hosts {
		seen[h] = true
	}
	res := make([]string, 0, len(seen))
	for k := range seen {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}

func (m *Miner) prefixKeys(hosts []string, pre string) []string {
	var res []string
	for _, h := range hosts {
		if strings.HasPrefix(h, pre) {
			res = append(res, h)
		}
	}
	return res
}

func (m *Miner) extractFirstTokens(keys []string) []string {
	firstTokens := make(map[string]bool)
	for _, k := range keys {
		ft := m.firstToken(k)
		if ft != "" {
			firstTokens[ft] = true
		}
	}
	var prefixes []string
	for ft := range firstTokens {
		prefixes = append(prefixes, ft)
	}
	sort.Strings(prefixes)
	return prefixes
}

func (m *Miner) firstToken(host string) string {
	tokens := m.tokenize([]string{host})
	if len(tokens) == 0 || len(tokens[0]) == 0 || len(tokens[0][0]) == 0 {
		return ""
	}
	return tokens[0][0][0]
}

func (m *Miner) groupRulesByStep(rules map[string]map[string]interface{}) *RulesOutput {
	groups := map[string][]*RuleEntry{
		"no_prefix":    {},
		"ngram":        {},
		"ngram_prefix": {},
	}
	stepNames := []string{"no_prefix", "ngram", "ngram_prefix"}

	for pattern, meta := range rules {
		mode, ok := meta["mode"].(string)
		if !ok {
			continue
		}

		// Build typed metadata struct
		patternMeta := &PatternMetadata{
			Mode: mode,
		}

		// Extract optional fields with type assertions
		if k, ok := meta["k"].(int); ok {
			patternMeta.K = &k
		}
		if ngram, ok := meta["ngram"].(string); ok {
			patternMeta.Ngram = ngram
		}
		if prefix, ok := meta["prefix"].(string); ok {
			patternMeta.Prefix = prefix
		}
		if clusterSize, ok := meta["cluster_size"].(int); ok {
			patternMeta.ClusterSize = clusterSize
		}
		if nwords, ok := meta["nwords"].(int); ok {
			patternMeta.Nwords = nwords
		}
		if ratio, ok := meta["ratio"].(float64); ok {
			patternMeta.Ratio = ratio
		}
		// Note: members field is intentionally excluded from output

		entry := &RuleEntry{
			Pattern: pattern,
			Meta:    patternMeta,
		}
		groups[mode] = append(groups[mode], entry)
	}

	// Build ordered steps
	steps := make([]*StepGroup, 0, len(stepNames))
	for _, step := range stepNames {
		entries := groups[step]
		sort.Slice(entries, func(a, b int) bool {
			return entries[a].Pattern < entries[b].Pattern
		})
		stepGroup := &StepGroup{
			Step:    step,
			Entries: entries,
		}
		steps = append(steps, stepGroup)
	}

	return &RulesOutput{
		Steps: steps,
	}
}

// getDist retrieves memoized distance
func (m *Miner) getDist(a, b string) int {
	key := getKey(a, b)
	if d, ok := m.memo[key]; ok {
		return d
	}
	return 999999
}

func getKey(a, b string) string {
	if strings.Compare(a, b) < 0 {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func levenshtein(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	m := make([]int, len(s2)+1)
	for i := range m {
		m[i] = i
	}
	for i := 1; i <= len(s1); i++ {
		curr := make([]int, len(s2)+1)
		curr[0] = i
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			curr[j] = min(curr[j-1]+1, min(m[j]+1, m[j-1]+cost))
		}
		m = curr
	}
	return m[len(s2)]
}

func escapeForDankEncoder(pattern string) string {
	var result strings.Builder
	prevWasOp := true

	for _, c := range pattern {
		if c == '(' || c == '|' {
			result.WriteRune(c)
			prevWasOp = true
		} else if c == ')' {
			result.WriteRune(c)
			prevWasOp = false
		} else if c == '*' && prevWasOp {
			result.WriteString("\\*")
			prevWasOp = false
		} else {
			result.WriteRune(c)
			prevWasOp = false
		}
	}
	return result.String()
}

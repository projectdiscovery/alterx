package inducer

import (
	"fmt"
	"math"
	"sort"

	"github.com/projectdiscovery/gologger"
)

// OrchestratorConfig contains configuration for the pattern induction orchestrator
type OrchestratorConfig struct {
	// Edit distance clustering parameters
	DistLow  int // Minimum edit distance delta (default: 2)
	DistHigh int // Maximum edit distance delta (default: 10)

	// Quality filtering parameters
	MinCoverage   int     // Minimum coverage (domains matched) for pattern to be kept (default: dynamic)
	MaxRatio      float64 // Maximum generation/observed ratio (default: 25.0)
	AbsoluteLimit int     // Auto-accept patterns generating < N subdomains (default: 500)

	// Processing options
	EnableCompression bool // Enable number range compression (default: true)
	EnableDedupe      bool // Enable pattern deduplication (default: true)
}

// DefaultOrchestratorConfig returns sensible defaults matching regulator
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		DistLow:           2,
		DistHigh:          10,
		MinCoverage:       2, // Will be overridden with dynamic value in LearnPatterns
		MaxRatio:          25.0,
		AbsoluteLimit:     500,
		EnableCompression: true,
		EnableDedupe:      true,
	}
}

// CalculateDynamicMinCoverage calculates minimum coverage threshold based on input size
// This scales the threshold to prevent pattern explosion on large datasets
//
// Formula: max(2, ceil(inputSize * 0.10))
// Examples:
//   - 10 domains:  max(2, ceil(1.0)) = 2
//   - 50 domains:  max(2, ceil(5.0)) = 5
//   - 100 domains: max(2, ceil(10.0)) = 10
//   - 500 domains: max(2, ceil(50.0)) = 50
//
// This ensures patterns match at least 10% of input domains, significantly reducing noise
// while still allowing discovery of common patterns across the dataset.
func CalculateDynamicMinCoverage(inputSize int) int {
	if inputSize <= 0 {
		return 2
	}

	// 10% of input size (rounded up), minimum 2
	threshold := int(math.Ceil(float64(inputSize) * 0.10))
	if threshold < 2 {
		threshold = 2
	}

	return threshold
}

// Orchestrator implements the ORIGINAL regulator algorithm faithfully
// WITHOUT premature optimizations (no partitioning, no bounded groups)
//
// This follows the algorithm from literature_survey/regulator/algorithm.md:
// 1. Build full edit distance MEMO table (O(N²) space)
// 2. Apply 3 clustering strategies:
//    - Strategy 1: Global edit distance clustering
//    - Strategy 2: N-gram prefix anchoring (TODO)
//    - Strategy 3: Token-level prefix + edit distance (TODO)
// 3. Generate patterns from closures
// 4. Compress number ranges
// 5. Quality filtering
// 6. Deduplicate
//
// IMPORTANT: This is designed to work FIRST, not to scale.
// We will profile and optimize AFTER it works correctly.
type Orchestrator struct {
	config    *OrchestratorConfig
	generator *PatternGenerator
	compressor *NumberCompressor

	// Data structures
	domains     []string            // Input domains
	edm         *EditDistanceMemo   // Edit distance memoization table (FULL O(N²))
	allPatterns []*Pattern          // Accumulated patterns from all strategies

	// Per-strategy pattern tracking
	strategy1Patterns []*Pattern // Patterns from global clustering
	strategy2Patterns []*Pattern // Patterns from n-gram prefix anchoring
	strategy3Patterns []*Pattern // Patterns from token-level clustering
}

// NewOrchestrator creates a new pattern induction orchestrator
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	return &Orchestrator{
		config:            config,
		generator:         NewPatternGenerator(),
		compressor:        NewNumberCompressor(),
		edm:               NewEditDistanceMemo(),
		allPatterns:       []*Pattern{},
		strategy1Patterns: []*Pattern{},
		strategy2Patterns: []*Pattern{},
		strategy3Patterns: []*Pattern{},
	}
}

// LearnPatterns executes the complete pattern induction pipeline
// This implements the ORIGINAL regulator algorithm
//
// Returns learned patterns or error
func (o *Orchestrator) LearnPatterns(domains []string) ([]*Pattern, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains provided")
	}

	o.domains = domains

	// Calculate dynamic minimum coverage threshold to prevent pattern explosion
	o.config.MinCoverage = CalculateDynamicMinCoverage(len(domains))
	gologger.Info().Msgf("Starting pattern induction on %d domains (min coverage: %d)", len(domains), o.config.MinCoverage)

	// STEP 1: Build full edit distance MEMO table
	// This is O(N²) space - the "bottleneck" we'll profile later
	gologger.Info().Msg("Building edit distance MEMO table...")
	if err := o.buildMemoTable(); err != nil {
		return nil, fmt.Errorf("failed to build MEMO table: %w", err)
	}
	gologger.Info().Msgf("MEMO table built: %d entries", o.edm.Size())

	// STEP 2: Build Trie index for prefix lookups (for Strategy 2 & 3)
	gologger.Info().Msg("Building Trie index...")
	trie := NewTrie(domains)
	trieStats := trie.GetStats()
	gologger.Info().Msgf("Trie built: %d nodes, max depth %d", trieStats.TotalNodes, trieStats.MaxDepth)

	// STEP 3: Strategy 1 - Global edit distance clustering
	gologger.Info().Msg("Strategy 1: Global edit distance clustering...")
	if err := o.strategyGlobalClustering(); err != nil {
		return nil, fmt.Errorf("strategy 1 failed: %w", err)
	}
	gologger.Info().Msgf("Strategy 1 complete: %d patterns found", len(o.strategy1Patterns))

	// STEP 4: Strategy 2 - N-gram prefix anchoring
	gologger.Info().Msg("Strategy 2: N-gram prefix anchoring...")
	if err := o.strategyNgramPrefix(trie); err != nil {
		return nil, fmt.Errorf("strategy 2 failed: %w", err)
	}
	gologger.Info().Msgf("Strategy 2 complete: %d patterns found", len(o.strategy2Patterns))

	// STEP 5: Strategy 3 - Token-level prefix + clustering
	gologger.Info().Msg("Strategy 3: Token-level clustering...")
	if err := o.strategyTokenLevel(trie); err != nil {
		return nil, fmt.Errorf("strategy 3 failed: %w", err)
	}
	gologger.Info().Msgf("Strategy 3 complete: %d patterns found", len(o.strategy3Patterns))

	// STEP 6: Post-processing (compression, quality, dedupe)
	gologger.Info().Msg("Post-processing patterns...")
	finalPatterns := o.postProcess()

	gologger.Info().Msgf("Pattern induction complete: %d final patterns", len(finalPatterns))
	return finalPatterns, nil
}

// buildMemoTable creates the full O(N²) edit distance table
// This is the "expensive" operation we'll profile later
func (o *Orchestrator) buildMemoTable() error {
	// Use the PrecomputeDistances method from EditDistanceMemo
	// This precomputes all pairwise distances
	o.edm.PrecomputeDistances(o.domains)

	return nil
}

// strategyGlobalClustering implements Strategy 1 from regulator
// Clusters ALL domains by edit distance, trying multiple delta values
func (o *Orchestrator) strategyGlobalClustering() error {
	// Try multiple delta values (k=2, k=3, ..., k=10)
	for k := o.config.DistLow; k <= o.config.DistHigh; k++ {
		gologger.Debug().Msgf("  Clustering with delta=%d", k)

		// Find edit closures with this delta
		closures := o.editClosures(o.domains, k)
		gologger.Debug().Msgf("  Found %d closures with delta=%d", len(closures), k)

		// Generate patterns from each closure
		for _, closure := range closures {
			// Skip patterns with insufficient coverage
			if len(closure.Domains) < o.config.MinCoverage {
				continue
			}

			// Generate pattern
			pattern, err := o.generator.GeneratePattern(closure)
			if err != nil {
				continue
			}

			// Compress numbers
			if o.config.EnableCompression {
				o.compressor.CompressPattern(pattern)
			}

			// Quality filtering (sets Ratio field)
			if o.isGoodPattern(pattern) {
				// Update confidence now that ratio is calculated
				pattern.UpdateConfidence()
				o.strategy1Patterns = append(o.strategy1Patterns, pattern)
				o.allPatterns = append(o.allPatterns, pattern)
			}
		}
	}

	return nil
}

// editClosures implements the regulator edit_closures() function
// For each domain, find all neighbors within delta edit distance
func (o *Orchestrator) editClosures(domains []string, delta int) []*Closure {
	closures := []*Closure{}
	seen := make(map[string]bool)

	for _, a := range domains {
		// Build closure around domain 'a'
		closureDomains := []string{a}

		for _, b := range domains {
			if a == b {
				continue
			}

			// Lookup distance from MEMO table
			dist := o.getDistance(a, b)
			if dist <= delta {
				closureDomains = append(closureDomains, b)
			}
		}

		// Deduplicate: skip if we've seen this exact closure before
		closureKey := o.closureKey(closureDomains)
		if seen[closureKey] {
			continue
		}
		seen[closureKey] = true

		closure := &Closure{
			Domains: closureDomains,
			Delta:   delta,
		}
		closures = append(closures, closure)
	}

	return closures
}

// getDistance retrieves edit distance from MEMO table
func (o *Orchestrator) getDistance(a, b string) int {
	return o.edm.Distance(a, b)
}

// closureKey creates a unique key for deduplication
// Sorts domains to ensure identical closures have same key
func (o *Orchestrator) closureKey(domains []string) string {
	// Sort domains for canonical key
	sorted := make([]string, len(domains))
	copy(sorted, domains)
	sort.Strings(sorted)

	// Concatenate with separator
	key := ""
	for _, d := range sorted {
		key += d + "|"
	}
	return key
}

// isGoodPattern implements regulator's quality filtering logic
// Rejects patterns that generate too many subdomains relative to observed
func (o *Orchestrator) isGoodPattern(pattern *Pattern) bool {
	// Calculate how many subdomains this pattern could generate
	generativity := o.estimateGenerativity(pattern)

	// Auto-accept small patterns
	if generativity < o.config.AbsoluteLimit {
		return true
	}

	// Ratio test for larger patterns
	ratio := float64(generativity) / float64(pattern.Coverage)
	pattern.Ratio = ratio

	return ratio < o.config.MaxRatio
}

// estimateGenerativity estimates how many domains a pattern generates
// Parses the regex pattern and calculates the total number of possible combinations
// by multiplying the counts of alternations, character classes, and optional groups
func (o *Orchestrator) estimateGenerativity(pattern *Pattern) int {
	if pattern.Regex == "" {
		return 1
	}

	return estimateRegexGenerativity(pattern.Regex)
}

// estimateRegexGenerativity calculates how many strings a regex pattern can generate
// This handles the specific regex constructs used by the pattern generator:
//   - Alternations: (a|b|c) → 3 options
//   - Character classes: [0-9] → 10 options, [a-z] → 26 options
//   - Optional groups: (...)? → 2 options (present or absent)
//   - Escaped characters: \., \-, etc. → 1 option (literal)
func estimateRegexGenerativity(regex string) int {
	count := 1
	i := 0

	for i < len(regex) {
		switch regex[i] {
		case '(':
			// Find matching closing paren
			closeIdx := findMatchingParen(regex, i)
			if closeIdx == -1 {
				i++
				continue
			}

			// Extract group content
			groupContent := regex[i+1 : closeIdx]

			// Check if it's an optional group (followed by ?)
			isOptional := closeIdx+1 < len(regex) && regex[closeIdx+1] == '?'

			// Parse the group
			groupCount := parseGroup(groupContent)

			if isOptional {
				// Optional: can be present or absent
				count *= (groupCount + 1)
				i = closeIdx + 2 // Skip past )?
			} else {
				// Required: multiply by group count
				count *= groupCount
				i = closeIdx + 1 // Skip past )
			}

		case '[':
			// Character class
			closeIdx := findClosingBracket(regex, i)
			if closeIdx == -1 {
				i++
				continue
			}

			// Parse character class
			classContent := regex[i+1 : closeIdx]
			classCount := parseCharacterClass(classContent)
			count *= classCount

			i = closeIdx + 1 // Skip past ]

		case '\\':
			// Escaped character - literal, counts as 1
			i += 2 // Skip escape and next char

		default:
			// Regular character - counts as 1
			i++
		}
	}

	return count
}

// parseGroup parses the content of a group and returns the number of alternatives
// Handles alternations like "a|b|c" → 3
func parseGroup(content string) int {
	if content == "" {
		return 1
	}

	// Check if it contains alternations (|)
	// Need to be careful about nested groups
	alternatives := splitAlternatives(content)

	if len(alternatives) <= 1 {
		// No alternations - calculate generativity of the content
		return estimateRegexGenerativity(content)
	}

	// Multiple alternations - sum the generativity of each alternative
	totalCount := 0
	for _, alt := range alternatives {
		totalCount += estimateRegexGenerativity(alt)
	}

	return totalCount
}

// splitAlternatives splits a group content by top-level | characters
// Ignores | inside nested groups or character classes
func splitAlternatives(content string) []string {
	if content == "" {
		return []string{}
	}

	alternatives := []string{}
	current := ""
	depth := 0
	inCharClass := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		switch ch {
		case '\\':
			// Escaped character - add both chars
			current += string(ch)
			if i+1 < len(content) {
				i++
				current += string(content[i])
			}

		case '[':
			if !inCharClass {
				inCharClass = true
			}
			current += string(ch)

		case ']':
			if inCharClass {
				inCharClass = false
			}
			current += string(ch)

		case '(':
			if !inCharClass {
				depth++
			}
			current += string(ch)

		case ')':
			if !inCharClass {
				depth--
			}
			current += string(ch)

		case '|':
			if depth == 0 && !inCharClass {
				// Top-level alternation separator
				alternatives = append(alternatives, current)
				current = ""
			} else {
				current += string(ch)
			}

		default:
			current += string(ch)
		}
	}

	// Add last alternative
	if current != "" {
		alternatives = append(alternatives, current)
	}

	if len(alternatives) == 0 {
		return []string{content}
	}

	return alternatives
}

// parseCharacterClass parses a character class like [0-9] or [a-zA-Z]
// Returns the number of characters matched
func parseCharacterClass(content string) int {
	if content == "" {
		return 1
	}

	count := 0
	i := 0

	for i < len(content) {
		if i+2 < len(content) && content[i+1] == '-' {
			// Range like 0-9 or a-z
			start := content[i]
			end := content[i+2]

			if end >= start {
				count += int(end-start) + 1
			} else {
				count++ // Invalid range, count as single char
			}

			i += 3

		} else if content[i] == '\\' {
			// Escaped character
			count++
			i += 2

		} else {
			// Single character
			count++
			i++
		}
	}

	return count
}

// findMatchingParen finds the matching closing parenthesis
// Returns -1 if not found
func findMatchingParen(s string, start int) int {
	if start >= len(s) || s[start] != '(' {
		return -1
	}

	depth := 1
	inCharClass := false

	for i := start + 1; i < len(s); i++ {
		ch := s[i]

		if ch == '\\' {
			// Skip escaped character
			i++
			continue
		}

		if ch == '[' && !inCharClass {
			inCharClass = true
			continue
		}

		if ch == ']' && inCharClass {
			inCharClass = false
			continue
		}

		if inCharClass {
			continue
		}

		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// findClosingBracket finds the matching closing bracket for a character class
// Returns -1 if not found
func findClosingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' {
			// Skip escaped character
			i++
			continue
		}

		if s[i] == ']' {
			return i
		}
	}

	return -1
}

// postProcess handles quality filtering and deduplication
func (o *Orchestrator) postProcess() []*Pattern {
	patterns := o.allPatterns

	// Step 1: Deduplicate by regex (remove exact duplicates)
	if o.config.EnableDedupe {
		seen := make(map[string]bool)
		unique := []*Pattern{}

		for _, pattern := range patterns {
			if !seen[pattern.Regex] {
				seen[pattern.Regex] = true
				unique = append(unique, pattern)
			}
		}

		gologger.Debug().Msgf("Deduplication: %d → %d patterns", len(patterns), len(unique))
		patterns = unique
	}

	// Step 2: Subsumption filtering (remove patterns subsumed by broader ones)
	patterns = FilterSubsumedPatterns(patterns)

	return patterns
}

// Stats returns statistics about pattern induction
type OrchestratorStats struct {
	InputDomains      int
	MemoTableSize     int
	Strategy1Patterns int
	Strategy2Patterns int
	Strategy3Patterns int
	TotalPatterns     int
	FinalPatterns     int
}

// GetStats returns current statistics
func (o *Orchestrator) GetStats() *OrchestratorStats {
	return &OrchestratorStats{
		InputDomains:      len(o.domains),
		MemoTableSize:     o.edm.Size(),
		Strategy1Patterns: len(o.strategy1Patterns),
		Strategy2Patterns: len(o.strategy2Patterns),
		Strategy3Patterns: len(o.strategy3Patterns),
		TotalPatterns:     len(o.allPatterns),
		FinalPatterns:     len(o.allPatterns),
	}
}

// strategyNgramPrefix implements Strategy 2: N-gram prefix anchoring
// Groups domains by common N-gram prefixes and applies edit distance clustering
// within each group. This reduces memory by avoiding full O(N²) MEMO table.
//
// Algorithm:
// 1. Extract 1-gram, 2-gram, 3-gram prefixes using Trie
// 2. For each prefix group (domains sharing same prefix):
//    - Build local edit distance MEMO for this group only
//    - Apply edit distance clustering within group
//    - Generate patterns from closures
// 3. This partitions the domain space by prefix similarity
func (o *Orchestrator) strategyNgramPrefix(trie *Trie) error {
	// Try different N-gram sizes (1, 2, 3)
	for n := 1; n <= 3; n++ {
		gologger.Debug().Msgf("  Strategy 2: N-gram=%d", n)

		// Get prefix groups from Trie
		prefixGroups := trie.GetNgramPrefixes(n)
		gologger.Debug().Msgf("  Found %d prefix groups for n=%d", len(prefixGroups), n)

		// Process each prefix group independently
		for prefix, domainIDs := range prefixGroups {
			if len(domainIDs) <= 1 {
				continue // Skip single-domain groups
			}

			// Extract actual domain strings
			groupDomains := make([]string, len(domainIDs))
			for i, id := range domainIDs {
				groupDomains[i] = o.domains[id]
			}

			gologger.Debug().Msgf("  Processing prefix '%s': %d domains", prefix, len(groupDomains))

			// Build local MEMO table for this group only
			localMemo := NewEditDistanceMemo()
			localMemo.PrecomputeDistances(groupDomains)

			// Apply clustering with different deltas
			for k := o.config.DistLow; k <= o.config.DistHigh; k++ {
				closures := o.editClosuresWithMemo(groupDomains, k, localMemo)

				// Generate patterns from closures
				for _, closure := range closures {
					// Skip patterns with insufficient coverage
					if len(closure.Domains) < o.config.MinCoverage {
						continue
					}

					pattern, err := o.generator.GeneratePattern(closure)
					if err != nil {
						continue
					}

					// Compress numbers
					if o.config.EnableCompression {
						o.compressor.CompressPattern(pattern)
					}

					// Quality filtering (sets Ratio field)
					if o.isGoodPattern(pattern) {
						// Update confidence now that ratio is calculated
						pattern.UpdateConfidence()
						o.strategy2Patterns = append(o.strategy2Patterns, pattern)
						o.allPatterns = append(o.allPatterns, pattern)
					}
				}
			}
		}
	}

	return nil
}

// strategyTokenLevel implements Strategy 3: Token-level clustering
// Groups domains by first token value and applies edit distance clustering
// within each token group.
//
// Algorithm:
// 1. Tokenize all domains into token arrays
// 2. Group domains by first token (token-level prefix)
// 3. For each token group:
//    - Build local edit distance MEMO
//    - Apply clustering
//    - Generate patterns
func (o *Orchestrator) strategyTokenLevel(trie *Trie) error {
	gologger.Debug().Msg("  Strategy 3: Token-level clustering")

	// Get token groups from Trie (uses internal tokenization)
	tokenGroups := trie.GetTokenGroupDomains()
	gologger.Debug().Msgf("  Found %d token groups", len(tokenGroups))

	// Process each token group
	for token, groupDomains := range tokenGroups {
		if len(groupDomains) <= 1 {
			continue
		}

		gologger.Debug().Msgf("  Token group '%s': %d domains", token, len(groupDomains))

		// Build local MEMO for this group
		localMemo := NewEditDistanceMemo()
		localMemo.PrecomputeDistances(groupDomains)

		// Apply clustering with different deltas
		for k := o.config.DistLow; k <= o.config.DistHigh; k++ {
			closures := o.editClosuresWithMemo(groupDomains, k, localMemo)

			for _, closure := range closures {
				// Skip patterns with insufficient coverage
				if len(closure.Domains) < o.config.MinCoverage {
					continue
				}

				pattern, err := o.generator.GeneratePattern(closure)
				if err != nil {
					continue
				}

				// Compress numbers
				if o.config.EnableCompression {
					o.compressor.CompressPattern(pattern)
				}

				// Quality filtering (sets Ratio field)
				if o.isGoodPattern(pattern) {
					// Update confidence now that ratio is calculated
					pattern.UpdateConfidence()
					o.strategy3Patterns = append(o.strategy3Patterns, pattern)
					o.allPatterns = append(o.allPatterns, pattern)
				}
			}
		}
	}

	return nil
}

// editClosuresWithMemo builds edit closures using a local MEMO table
// This is similar to editClosures but uses a provided memo instead of the global one
// Useful for processing bounded groups in Strategy 2 and 3
func (o *Orchestrator) editClosuresWithMemo(domains []string, delta int, memo *EditDistanceMemo) []*Closure {
	closures := []*Closure{}
	seen := make(map[string]bool)

	for _, a := range domains {
		// Build closure around domain 'a'
		closureDomains := []string{a}

		for _, b := range domains {
			if a == b {
				continue
			}

			// Lookup distance from local MEMO table
			dist := memo.Distance(a, b)
			if dist <= delta {
				closureDomains = append(closureDomains, b)
			}
		}

		// Deduplicate: skip if we've seen this exact closure before
		closureKey := o.closureKey(closureDomains)
		if seen[closureKey] {
			continue
		}
		seen[closureKey] = true

		closure := &Closure{
			Domains: closureDomains,
			Delta:   delta,
		}
		closures = append(closures, closure)
	}

	return closures
}

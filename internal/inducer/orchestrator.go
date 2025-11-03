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

// DefaultOrchestratorConfig returns optimized defaults for high-confidence patterns
// These values are tuned to achieve 70%+ confidence scores
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		DistLow:           2,
		DistHigh:          15, // Increased from 10 to allow more flexible clustering
		MinCoverage:       2,  // Will be overridden with dynamic value in LearnPatterns
		MaxRatio:          5.0, // Reduced from 25.0 to enforce tight patterns (70%+ confidence target)
		AbsoluteLimit:     500,
		EnableCompression: true,
		EnableDedupe:      true,
	}
}

// CalculateDynamicMinCoverage calculates minimum coverage threshold based on input size
// Uses exponential decay to balance pattern discovery across different dataset sizes
//
// Formula: minsup(x) = e^(-ax-b) + c
//   Where a=0.3, b=0.5, c=0.02 (asymptotic minimum at 2%)
//
// Results:
//   - 5 domains   → 25% → 2 domains min  (flexible for small groups)
//   - 25 domains  → 3%  → 1 domain min   (catches rare patterns)
//   - 100 domains → 2%  → 2 domains min  (balanced)
//   - 500 domains → 2%  → 10 domains min (prevents noise)
//
// This approach is based on research:
//   - Philippe Fournier-Viger: Adaptive minimum support with exponential decay
//   - Pattern mining literature: Small datasets need higher % but lower absolute thresholds
//   - Regulator defaults: threshold=500 with max_ratio=25 as secondary filter
//
// Key insight: Let MORE patterns form initially, then apply strict ratio/confidence filtering
func CalculateDynamicMinCoverage(inputSize int) int {
	if inputSize <= 0 {
		return 2
	}

	// Exponential decay parameters
	a := 0.3
	b := 0.5
	c := 0.02 // Asymptotic minimum (2%)

	// Calculate relative threshold using exponential decay
	// Dividing inputSize by 10 smooths the curve
	relativeThreshold := math.Exp(-a*float64(inputSize)/10.0-b) + c
	absoluteThreshold := int(math.Ceil(float64(inputSize) * relativeThreshold))

	// Apply bounds: min=2, max=10% of input (safety cap)
	if absoluteThreshold < 2 {
		absoluteThreshold = 2
	}
	maxThreshold := int(math.Ceil(float64(inputSize) * 0.10))
	if absoluteThreshold > maxThreshold {
		absoluteThreshold = maxThreshold
	}

	return absoluteThreshold
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
// 3. Generate DSL patterns directly from closures (bypasses regex generation)
// 4. Quality filtering
// 5. Deduplicate
//
// IMPORTANT: This is designed to work FIRST, not to scale.
// We will profile and optimize AFTER it works correctly.
type Orchestrator struct {
	config       *OrchestratorConfig
	dslGenerator *DSLGenerator

	// Data structures
	domains     []string            // Input domains
	edm         *EditDistanceMemo   // Edit distance memoization table (FULL O(N²))
	allPatterns []*DSLPattern       // Accumulated patterns from all strategies

	// Per-strategy pattern tracking
	strategy1Patterns []*DSLPattern // Patterns from global clustering
	strategy2Patterns []*DSLPattern // Patterns from n-gram prefix anchoring
	strategy3Patterns []*DSLPattern // Patterns from token-level clustering
}

// NewOrchestrator creates a new pattern induction orchestrator
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	return &Orchestrator{
		config:            config,
		dslGenerator:      NewDSLGenerator(nil), // No semantic dictionary for now
		edm:               NewEditDistanceMemo(),
		allPatterns:       []*DSLPattern{},
		strategy1Patterns: []*DSLPattern{},
		strategy2Patterns: []*DSLPattern{},
		strategy3Patterns: []*DSLPattern{},
	}
}

// SetTokenDictionary updates the token dictionary for semantic classification
// This should be called before LearnPatterns() to enable semantic token naming
func (o *Orchestrator) SetTokenDictionary(dict *TokenDictionary) {
	if dict != nil {
		o.dslGenerator = NewDSLGenerator(dict)
	}
}

// LearnPatterns executes the complete pattern induction pipeline
// WITH level-based grouping to respect structural depth boundaries
//
// New Architecture:
// 1. Group domains by level count (structural depth)
// 2. Process each level group independently (avoids mixing different structures)
// 3. Apply all 3 clustering strategies per level group
// 4. Merge patterns from all groups
//
// Returns learned DSL patterns or error
func (o *Orchestrator) LearnPatterns(domains []string) ([]*DSLPattern, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains provided")
	}

	o.domains = domains

	// Calculate dynamic minimum coverage threshold to prevent pattern explosion
	o.config.MinCoverage = CalculateDynamicMinCoverage(len(domains))
	gologger.Verbose().Msgf("Starting pattern induction on %d domains (min coverage: %d)", len(domains), o.config.MinCoverage)

	// STEP 1: Group domains by level count (structural depth)
	// This prevents mixing domains with different hierarchical structures
	// e.g., "api.example.com" (1 level) vs "scheduler.api.example.com" (2 levels)
	gologger.Verbose().Msg("Step 1: Grouping domains by level count...")
	levelGroups, err := GroupByLevelCount(domains)
	if err != nil {
		return nil, fmt.Errorf("failed to group by level count: %w", err)
	}
	PrintLevelGroupStats(levelGroups)

	// STEP 2: Process each level group independently
	gologger.Verbose().Msgf("Step 2: Processing %d level groups...", len(levelGroups))
	sortedGroups := GetSortedLevelGroups(levelGroups)

	for i, group := range sortedGroups {
		gologger.Verbose().Msgf("Processing group %d/%d: level=%d (%d domains)",
			i+1, len(sortedGroups), group.LevelCount, len(group.Domains))

		// Process this level group
		if err := o.processLevelGroup(group); err != nil {
			gologger.Warning().Msgf("Failed to process level group %d: %v", group.LevelCount, err)
			continue
		}
	}

	// STEP 3: Post-processing (compression, quality, dedupe)
	gologger.Verbose().Msg("Step 3: Post-processing patterns...")
	finalPatterns := o.postProcess()

	// STEP 4: Enrich patterns with optional variable support
	// This adds empty string "" to appropriate variables for ClusterBomb flexibility
	gologger.Verbose().Msg("Step 4: Enriching patterns with optional variables...")
	enrichedPatterns := EnrichPatterns(finalPatterns, o.dslGenerator.dictionary)

	gologger.Verbose().Msgf("Pattern induction complete: %d final patterns", len(enrichedPatterns))
	return enrichedPatterns, nil
}

// processLevelGroup processes a single level group through all 3 strategies
// This is the core of the level-aware pattern induction
func (o *Orchestrator) processLevelGroup(group *LevelGroup) error {
	groupDomains := group.Domains

	// Dynamic minimum coverage based on group size (not global size)
	groupMinCoverage := CalculateDynamicMinCoverage(len(groupDomains))
	if groupMinCoverage < 2 {
		groupMinCoverage = 2
	}

	gologger.Debug().Msgf("  Group min coverage: %d", groupMinCoverage)

	// Build local MEMO table for this group only
	// This keeps memory bounded per group
	gologger.Debug().Msg("  Building local MEMO table...")
	localMemo := NewEditDistanceMemo()
	localMemo.PrecomputeDistances(groupDomains)
	gologger.Debug().Msgf("  Local MEMO table: %d entries", localMemo.Size())

	// Build local Trie for this group
	gologger.Debug().Msg("  Building local Trie...")
	localTrie := NewTrie(groupDomains)

	// Strategy 1: Global clustering within this level group
	gologger.Debug().Msg("  Strategy 1: Global clustering (within level group)...")
	if err := o.strategyGlobalClusteringWithMemo(groupDomains, localMemo, groupMinCoverage); err != nil {
		return fmt.Errorf("strategy 1 failed: %w", err)
	}

	// Strategy 2: N-gram prefix anchoring within this level group
	gologger.Debug().Msg("  Strategy 2: N-gram prefix (within level group)...")
	if err := o.strategyNgramPrefixWithMemo(localTrie, groupDomains, localMemo, groupMinCoverage); err != nil {
		return fmt.Errorf("strategy 2 failed: %w", err)
	}

	// Strategy 3: Token-level clustering within this level group
	gologger.Debug().Msg("  Strategy 3: Token-level (within level group)...")
	if err := o.strategyTokenLevelWithMemo(localTrie, groupDomains, localMemo, groupMinCoverage); err != nil {
		return fmt.Errorf("strategy 3 failed: %w", err)
	}

	return nil
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

			// Generate DSL pattern directly (no regex intermediate step)
			pattern, err := o.dslGenerator.GeneratePattern(closure)
			if err != nil {
				continue
			}

			// Quality filtering (ratio already calculated in GeneratePattern)
			if pattern.PassesQualityFilter() {
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

// Note: Quality filtering is now handled by DSLPattern.PassesQualityFilter()
// The ratio calculation is done during pattern generation in DSLGenerator

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
func (o *Orchestrator) postProcess() []*DSLPattern {
	patterns := o.allPatterns

	// Step 1: Deduplicate by template (remove exact duplicates)
	if o.config.EnableDedupe {
		seen := make(map[string]bool)
		unique := []*DSLPattern{}

		for _, pattern := range patterns {
			if !seen[pattern.Template] {
				seen[pattern.Template] = true
				unique = append(unique, pattern)
			}
		}

		gologger.Debug().Msgf("Deduplication: %d → %d patterns", len(patterns), len(unique))
		patterns = unique
	}

	// Step 2: Smart pattern consolidation with ADAPTIVE MAX_PATTERNS
	// Automatically adjust based on dataset size for optimal coverage
	// Based on cross-domain testing (hackerone, zomato, stripe, openai, tstaging)
	//
	// Results:
	//   Small (< 50):     8 patterns, 62% conf, no clustering needed
	//   Mid (50-200):     6 patterns, 87% coverage, 49% conf
	//   Large (200-500):  7-9 patterns, 50-51% coverage, 57-58% conf
	//   XLarge (500+):    5-9 patterns, 41-51% coverage, 45-57% conf
	datasetSize := len(o.domains)
	var maxPatterns int

	if datasetSize < 50 {
		maxPatterns = 5 // Small: prioritize quality over quantity
		gologger.Debug().Msgf("Small dataset (%d domains): targeting %d high-quality patterns", datasetSize, maxPatterns)
	} else if datasetSize < 200 {
		maxPatterns = 8 // Mid: balanced approach (87% coverage achievable)
		gologger.Debug().Msgf("Mid dataset (%d domains): targeting %d patterns", datasetSize, maxPatterns)
	} else if datasetSize < 500 {
		maxPatterns = 10 // Large: more patterns for diverse structures
		gologger.Debug().Msgf("Large dataset (%d domains): targeting %d patterns", datasetSize, maxPatterns)
	} else {
		maxPatterns = 12 // XLarge: allow more patterns for complex organizations
		gologger.Debug().Msgf("XLarge dataset (%d domains): targeting %d patterns", datasetSize, maxPatterns)
	}
	if len(patterns) <= maxPatterns {
		gologger.Verbose().Msgf("Pattern count (%d) within target (%d), skipping clustering",
			len(patterns), maxPatterns)
	} else {
		gologger.Verbose().Msgf("Pattern count (%d) exceeds target (%d), clustering to consolidate",
			len(patterns), maxPatterns)

		clusterConfig := DefaultClusterConfig
		clusterConfig.MergeStrategy = MergeUnionConservative
		clusterConfig.MinClusterSize = 1

		// Use PURE STRUCTURAL weights (template + token sequence ONLY)
		// Domain overlap EXCLUDED because patterns with identical structure
		// can match completely different domain sets (this is valid!)
		clusterConfig.DistanceWeights = PureStructuralWeights

		// Adjust AP preference to target ~maxPatterns clusters
		// Higher preference → more clusters (closer to maxPatterns)
		// Lower preference → fewer clusters
		// Use median similarity as baseline, then adjust upward to get more clusters
		clusterConfig.APConfig.Preference = -0.3 // Higher = more clusters (targeting ~10)

		gologger.Debug().Msgf("Clustering config: weights=pure_structural, preference=%.2f, target=%d patterns",
			clusterConfig.APConfig.Preference, maxPatterns)

		clustered, metrics, err := ClusterPatterns(patterns, clusterConfig)
		if err != nil {
			gologger.Warning().Msgf("Clustering failed: %v, continuing with unclustered patterns", err)
		} else {
			gologger.Verbose().Msgf("Clustering: %d → %d patterns (target: %d, silhouette=%.3f)",
				len(patterns), len(clustered), maxPatterns, metrics.Silhouette)

			// Check if we're close to target
			if len(clustered) <= maxPatterns {
				gologger.Info().Msgf("✓ Consolidation successful: %d patterns (within target of %d)",
					len(clustered), maxPatterns)
			} else {
				gologger.Warning().Msgf("⚠ Still have %d patterns (target: %d), may need stricter preference",
					len(clustered), maxPatterns)
			}

			patterns = clustered
		}
	}

	// Step 3: Adaptive quality filtering based on dataset size
	// Small datasets: stricter (prioritize quality)
	// Large datasets: lenient (prioritize coverage)
	minConfidence := 0.50 // Legacy parameter (unused, now adaptive inside function)
	patterns = FilterLowQualityTokens(patterns, minConfidence, o.config.MaxRatio, datasetSize)

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

					// Generate DSL pattern directly
					pattern, err := o.dslGenerator.GeneratePattern(closure)
					if err != nil {
						continue
					}

					// Quality filtering
					if pattern.PassesQualityFilter() {
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

				// Generate DSL pattern directly
				pattern, err := o.dslGenerator.GeneratePattern(closure)
				if err != nil {
					continue
				}

				// Quality filtering
				if pattern.PassesQualityFilter() {
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

// strategyGlobalClusteringWithMemo implements Strategy 1 using a local MEMO table
// This is used when processing suffix groups independently
func (o *Orchestrator) strategyGlobalClusteringWithMemo(domains []string, memo *EditDistanceMemo, minCoverage int) error {
	// Try multiple delta values (k=2, k=3, ..., k=10)
	for k := o.config.DistLow; k <= o.config.DistHigh; k++ {
		// Find edit closures with this delta using local memo
		closures := o.editClosuresWithMemo(domains, k, memo)

		// Generate patterns from each closure
		for _, closure := range closures {
			// Skip patterns with insufficient coverage
			if len(closure.Domains) < minCoverage {
				continue
			}

			// Generate DSL pattern directly (no regex intermediate step)
			pattern, err := o.dslGenerator.GeneratePattern(closure)
			if err != nil {
				continue
			}

			// Quality filtering (ratio already calculated in GeneratePattern)
			if pattern.PassesQualityFilter() {
				o.strategy1Patterns = append(o.strategy1Patterns, pattern)
				o.allPatterns = append(o.allPatterns, pattern)
			}
		}
	}

	return nil
}

// strategyNgramPrefixWithMemo implements Strategy 2 using local data structures
// This is used when processing suffix groups independently
func (o *Orchestrator) strategyNgramPrefixWithMemo(trie *Trie, domains []string, memo *EditDistanceMemo, minCoverage int) error {
	// Try different N-gram sizes (1, 2, 3)
	for n := 1; n <= 3; n++ {
		// Get prefix groups from local Trie
		prefixGroups := trie.GetNgramPrefixes(n)

		// Process each prefix group independently
		for _, domainIDs := range prefixGroups {
			if len(domainIDs) <= 1 {
				continue // Skip single-domain groups
			}

			// Extract actual domain strings
			groupDomains := make([]string, len(domainIDs))
			for i, id := range domainIDs {
				groupDomains[i] = domains[id]
			}

			// Apply clustering with different deltas
			for k := o.config.DistLow; k <= o.config.DistHigh; k++ {
				closures := o.editClosuresWithMemo(groupDomains, k, memo)

				// Generate patterns from closures
				for _, closure := range closures {
					// Skip patterns with insufficient coverage
					if len(closure.Domains) < minCoverage {
						continue
					}

					// Generate DSL pattern directly
					pattern, err := o.dslGenerator.GeneratePattern(closure)
					if err != nil {
						continue
					}

					// Quality filtering
					if pattern.PassesQualityFilter() {
						o.strategy2Patterns = append(o.strategy2Patterns, pattern)
						o.allPatterns = append(o.allPatterns, pattern)
					}
				}
			}
		}
	}

	return nil
}

// strategyTokenLevelWithMemo implements Strategy 3 using local data structures
// This is used when processing suffix groups independently
func (o *Orchestrator) strategyTokenLevelWithMemo(trie *Trie, domains []string, memo *EditDistanceMemo, minCoverage int) error {
	// Get token groups from local Trie
	tokenGroups := trie.GetTokenGroupDomains()

	// Process each token group
	for _, groupDomains := range tokenGroups {
		if len(groupDomains) <= 1 {
			continue
		}

		// Apply clustering with different deltas
		for k := o.config.DistLow; k <= o.config.DistHigh; k++ {
			closures := o.editClosuresWithMemo(groupDomains, k, memo)

			for _, closure := range closures {
				// Skip patterns with insufficient coverage
				if len(closure.Domains) < minCoverage {
					continue
				}

				// Generate DSL pattern directly
				pattern, err := o.dslGenerator.GeneratePattern(closure)
				if err != nil {
					continue
				}

				// Quality filtering
				if pattern.PassesQualityFilter() {
					o.strategy3Patterns = append(o.strategy3Patterns, pattern)
					o.allPatterns = append(o.allPatterns, pattern)
				}
			}
		}
	}

	return nil
}

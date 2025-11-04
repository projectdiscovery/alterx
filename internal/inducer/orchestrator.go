package inducer

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"

	"github.com/projectdiscovery/gologger"
)

// Orchestrator coordinates the complete pattern induction pipeline
type Orchestrator struct {
	modeConfig   *ModeConfig
	dslGenerator *DSLGenerator

	// Statistics
	stats OrchestratorStats
}

// OrchestratorStats tracks pipeline metrics
type OrchestratorStats struct {
	InputDomains      int
	Mode              string
	FilteredDomains   int
	LevelGroups       int
	Strategy1Patterns int
	Strategy2Patterns int
	Strategy3Patterns int
	RawPatterns       int
	AfterDedup        int
	AfterAP           int
	FinalPatterns     int
	MemoryPeakMB      float64
}

// NewOrchestrator creates a new orchestrator with mode-based configuration
func NewOrchestrator(inputSize int) *Orchestrator {
	modeConfig := NewModeConfig(inputSize)
	return &Orchestrator{
		modeConfig:   modeConfig,
		dslGenerator: NewDSLGenerator(nil), // Dictionary set later via SetTokenDictionary
	}
}

// SetTokenDictionary configures semantic token classification
func (o *Orchestrator) SetTokenDictionary(dict *TokenDictionary) {
	o.dslGenerator = NewDSLGenerator(dict)
}

// GetStats returns orchestrator statistics
func (o *Orchestrator) GetStats() *OrchestratorStats {
	return &o.stats
}

// LearnPatterns is the main entry point for pattern induction
// Implements the complete production pipeline with parallelization
func (o *Orchestrator) LearnPatterns(domains []string) ([]*DSLPattern, error) {
	o.stats.InputDomains = len(domains)
	o.stats.Mode = o.modeConfig.Mode.String()

	if len(domains) == 0 {
		return []*DSLPattern{}, nil
	}

	gologger.Verbose().Msgf("[Mode: %s] Starting pattern induction with %d domains",
		o.modeConfig.Mode, len(domains))

	// STEP 1: Input filtering
	filtered := o.filterInvalidDomains(domains)
	o.stats.FilteredDomains = len(filtered)
	if len(filtered) == 0 {
		gologger.Warning().Msg("No valid domains after filtering")
		return []*DSLPattern{}, nil
	}
	gologger.Verbose().Msgf("[Step 1] Filtered %d → %d valid domains",
		len(domains), len(filtered))

	// STEP 2: Level-based grouping
	groupsMap, err := GroupByLevelCount(filtered)
	if err != nil {
		return []*DSLPattern{}, fmt.Errorf("level grouping failed: %v", err)
	}
	// Convert map to slice for easier parallelization
	groups := make([]*LevelGroup, 0, len(groupsMap))
	for _, group := range groupsMap {
		groups = append(groups, group)
	}
	o.stats.LevelGroups = len(groups)
	gologger.Verbose().Msgf("[Step 2] Grouped into %d level-based groups", len(groups))

	// STEP 3-6: Process level groups in parallel (L1 parallelization)
	allPatterns := o.processLevelGroupsParallel(groups)
	o.stats.RawPatterns = len(allPatterns)
	gologger.Verbose().Msgf("[Step 3-6] Generated %d raw patterns from all strategies", len(allPatterns))

	// STEP 7: Deduplication
	allPatterns = o.deduplicatePatterns(allPatterns)
	o.stats.AfterDedup = len(allPatterns)
	gologger.Verbose().Msgf("[Step 7] Deduplicated %d → %d patterns", o.stats.RawPatterns, len(allPatterns))

	// STEP 8: Affinity Propagation (conditional)
	if len(allPatterns) > o.modeConfig.MaxPatterns {
		allPatterns = o.clusterPatternsAP(allPatterns)
		o.stats.AfterAP = len(allPatterns)
		gologger.Verbose().Msgf("[Step 8] AP clustering %d → %d patterns",
			o.stats.AfterDedup, len(allPatterns))
	} else {
		o.stats.AfterAP = len(allPatterns)
		gologger.Verbose().Msgf("[Step 8] Skipped AP clustering (%d patterns within limit)", len(allPatterns))
	}

	// STEP 9: Entropy-based selection
	selected := o.selectPatternsByEntropy(allPatterns, filtered)
	o.stats.FinalPatterns = len(selected)
	gologger.Verbose().Msgf("[Step 9] Entropy selection %d → %d patterns (coverage: %.1f%%)",
		len(allPatterns), len(selected), o.calculateCoverage(selected, filtered)*100)

	// STEP 10: Enrichment
	enriched := o.enrichPatterns(selected)
	gologger.Verbose().Msgf("[Step 10] Enriched %d patterns with optional variables", len(enriched))

	return enriched, nil
}

// filterInvalidDomains removes wildcards, root-only, invalid TLDs
func (o *Orchestrator) filterInvalidDomains(domains []string) []string {
	valid := make([]string, 0, len(domains))
	for _, d := range domains {
		if strings.HasPrefix(d, "*.") {
			continue
		}
		// Additional validation happens in GroupByLevelCount
		valid = append(valid, d)
	}
	return valid
}

// processLevelGroupsParallel processes each level group in parallel
// L1 PARALLELIZATION: Group-level parallelism (2-4x speedup)
func (o *Orchestrator) processLevelGroupsParallel(groups []*LevelGroup) []*DSLPattern {
	var wg sync.WaitGroup
	resultChan := make(chan []*DSLPattern, len(groups))

	for _, group := range groups {
		wg.Add(1)
		go func(g *LevelGroup) {
			defer wg.Done()
			patterns := o.processLevelGroup(g)
			resultChan <- patterns
		}(group)
	}

	// Wait and collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	allPatterns := make([]*DSLPattern, 0)
	for patterns := range resultChan {
		allPatterns = append(allPatterns, patterns...)
	}

	return allPatterns
}

// processLevelGroup processes a single level group through all strategies
func (o *Orchestrator) processLevelGroup(group *LevelGroup) []*DSLPattern {
	groupDomains := group.Domains

	// STEP 3: Adaptive sampling (FAST mode only)
	if o.modeConfig.EnableGroupSampling && len(groupDomains) > o.modeConfig.MaxGroupSize {
		groupDomains = o.sampleGroupStratified(groupDomains)
	}

	// STEP 4: Build local indexes
	memo := NewEditDistanceMemo()
	memo.PrecomputeDistances(groupDomains)
	trie := NewTrie(groupDomains)

	// STEP 5: Three strategies in parallel (L2 PARALLELIZATION)
	patterns := o.runStrategiesParallel(groupDomains, memo, trie)

	return patterns
}

// sampleGroupStratified performs stratified sampling for FAST mode
func (o *Orchestrator) sampleGroupStratified(domains []string) []string {
	// Partition by first token
	partitions := make(map[string][]string)
	for _, d := range domains {
		parts := strings.Split(strings.Split(d, ".")[0], "-")
		if len(parts) > 0 {
			firstToken := parts[0]
			partitions[firstToken] = append(partitions[firstToken], d)
		}
	}

	sampled := make([]string, 0, o.modeConfig.MaxGroupSize)
	totalSize := len(domains)

	for _, partition := range partitions {
		freq := float64(len(partition)) / float64(totalSize)

		var toKeep []string
		if freq < 0.05 {
			// Rare tokens - keep all
			toKeep = partition
		} else if freq > 0.50 {
			// Dominant tokens - aggressive sampling
			sampleSize := minInt(200, len(partition))
			toKeep = sampleRandom(partition, sampleSize)
		} else {
			// Normal tokens - moderate sampling
			sampleSize := int(float64(len(partition)) * 0.60)
			if sampleSize < 1 {
				sampleSize = 1
			}
			toKeep = sampleRandom(partition, sampleSize)
		}

		sampled = append(sampled, toKeep...)
	}

	// Shuffle to avoid bias
	rand.Shuffle(len(sampled), func(i, j int) {
		sampled[i], sampled[j] = sampled[j], sampled[i]
	})

	return sampled
}

// runStrategiesParallel runs all three strategies in parallel
// L2 PARALLELIZATION: Strategy-level parallelism (3x speedup)
func (o *Orchestrator) runStrategiesParallel(domains []string, memo *EditDistanceMemo, trie *Trie) []*DSLPattern {
	var wg sync.WaitGroup
	var mu sync.Mutex

	allPatterns := make([]*DSLPattern, 0)

	// Strategy 1: Global clustering (ALWAYS)
	wg.Add(1)
	go func() {
		defer wg.Done()
		patterns := o.strategyGlobal(domains, memo)
		mu.Lock()
		allPatterns = append(allPatterns, patterns...)
		o.stats.Strategy1Patterns += len(patterns)
		mu.Unlock()
	}()

	// Strategy 2: N-gram anchoring (CONDITIONAL)
	if o.shouldRunStrategy2(len(domains)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			patterns := o.strategyNgram(domains, memo, trie)
			mu.Lock()
			allPatterns = append(allPatterns, patterns...)
			o.stats.Strategy2Patterns += len(patterns)
			mu.Unlock()
		}()
	}

	// Strategy 3: Token-level clustering (ALWAYS)
	wg.Add(1)
	go func() {
		defer wg.Done()
		patterns := o.strategyTokenLevel(domains, memo, trie)
		mu.Lock()
		allPatterns = append(allPatterns, patterns...)
		o.stats.Strategy3Patterns += len(patterns)
		mu.Unlock()
	}()

	wg.Wait()
	return allPatterns
}

// shouldRunStrategy2 determines if Strategy 2 should run
func (o *Orchestrator) shouldRunStrategy2(groupSize int) bool {
	if !o.modeConfig.EnableNgramStrategy {
		return false
	}
	return groupSize > o.modeConfig.NgramThreshold
}

// strategyGlobal implements Strategy 1: Global clustering
// L4 PARALLELIZATION: Distance levels in parallel (FAST mode)
func (o *Orchestrator) strategyGlobal(domains []string, memo *EditDistanceMemo) []*DSLPattern {
	patterns := make([]*DSLPattern, 0)
	minCoverage := maxInt(2, len(domains)/100) // 1% min

	distLevels := make([]int, 0)
	for k := o.modeConfig.DistLow; k <= o.modeConfig.DistHigh; k++ {
		distLevels = append(distLevels, k)
	}

	// FAST mode: parallelize distance levels
	if o.modeConfig.Mode == ModeFast {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, k := range distLevels {
			wg.Add(1)
			go func(delta int) {
				defer wg.Done()
				closures := o.editClosuresWithMemo(domains, delta, memo)
				dslPatterns := o.generateDSLFromClosures(closures, minCoverage)
				mu.Lock()
				patterns = append(patterns, dslPatterns...)
				mu.Unlock()
			}(k)
		}
		wg.Wait()
	} else {
		// THOROUGH/BALANCED: sequential
		for _, k := range distLevels {
			closures := o.editClosuresWithMemo(domains, k, memo)
			patterns = append(patterns, o.generateDSLFromClosures(closures, minCoverage)...)
		}
	}

	return patterns
}

// strategyNgram implements Strategy 2: N-gram prefix anchoring
// L4 PARALLELIZATION: Partitions in parallel (FAST mode, >10 partitions)
func (o *Orchestrator) strategyNgram(domains []string, memo *EditDistanceMemo, trie *Trie) []*DSLPattern {
	// Determine n-gram size
	ngramSize := 2
	if len(domains) > 300 {
		ngramSize = 3
	}

	// Partition by n-gram prefixes
	partitions := o.partitionByNgram(domains, ngramSize)
	minCoverage := maxInt(2, len(domains)/100)

	patterns := make([]*DSLPattern, 0)

	// FAST mode with many partitions: parallelize
	if o.modeConfig.Mode == ModeFast && len(partitions) > 10 {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, partition := range partitions {
			wg.Add(1)
			go func(p []string) {
				defer wg.Done()
				localMemo := NewEditDistanceMemo()
				localMemo.PrecomputeDistances(p)
				for k := o.modeConfig.DistLow; k <= o.modeConfig.DistHigh; k++ {
					closures := o.editClosuresWithMemo(p, k, localMemo)
					dslPatterns := o.generateDSLFromClosures(closures, minCoverage)
					mu.Lock()
					patterns = append(patterns, dslPatterns...)
					mu.Unlock()
				}
			}(partition)
		}
		wg.Wait()
	} else {
		// Sequential
		for _, partition := range partitions {
			localMemo := NewEditDistanceMemo()
			localMemo.PrecomputeDistances(partition)
			for k := o.modeConfig.DistLow; k <= o.modeConfig.DistHigh; k++ {
				closures := o.editClosuresWithMemo(partition, k, localMemo)
				patterns = append(patterns, o.generateDSLFromClosures(closures, minCoverage)...)
			}
		}
	}

	return patterns
}

// strategyTokenLevel implements Strategy 3: Token-level clustering
// L4 PARALLELIZATION: Partitions in parallel (FAST mode, >10 partitions)
func (o *Orchestrator) strategyTokenLevel(domains []string, memo *EditDistanceMemo, trie *Trie) []*DSLPattern {
	// Partition by first token
	partitions := o.partitionByFirstToken(domains)

	// FAST mode: limit to top 30 tokens + rare
	if o.modeConfig.EnableTokenLimiting && len(partitions) > 50 {
		partitions = o.limitTokenPartitions(partitions, o.modeConfig.MaxTokenGroups)
	}

	minCoverage := maxInt(2, len(domains)/100)
	patterns := make([]*DSLPattern, 0)

	// FAST mode with many partitions: parallelize
	if o.modeConfig.Mode == ModeFast && len(partitions) > 10 {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, partition := range partitions {
			wg.Add(1)
			go func(p []string) {
				defer wg.Done()
				localMemo := NewEditDistanceMemo()
				localMemo.PrecomputeDistances(p)
				for k := o.modeConfig.DistLow; k <= o.modeConfig.DistHigh; k++ {
					closures := o.editClosuresWithMemo(p, k, localMemo)
					dslPatterns := o.generateDSLFromClosures(closures, minCoverage)
					mu.Lock()
					patterns = append(patterns, dslPatterns...)
					mu.Unlock()
				}
			}(partition)
		}
		wg.Wait()
	} else {
		// Sequential
		for _, partition := range partitions {
			localMemo := NewEditDistanceMemo()
			localMemo.PrecomputeDistances(partition)
			for k := o.modeConfig.DistLow; k <= o.modeConfig.DistHigh; k++ {
				closures := o.editClosuresWithMemo(partition, k, localMemo)
				patterns = append(patterns, o.generateDSLFromClosures(closures, minCoverage)...)
			}
		}
	}

	return patterns
}

// editClosuresWithMemo finds edit closures using memoized distances
func (o *Orchestrator) editClosuresWithMemo(domains []string, delta int, memo *EditDistanceMemo) []*Closure {
	visited := make(map[string]bool)
	closures := make([]*Closure, 0)

	for _, seed := range domains {
		if visited[seed] {
			continue
		}

		closure := []string{seed}
		visited[seed] = true

		for _, candidate := range domains {
			if visited[candidate] {
				continue
			}
			if memo.Distance(seed, candidate) <= delta {
				closure = append(closure, candidate)
				visited[candidate] = true
			}
		}

		if len(closure) >= 2 {
			closures = append(closures, &Closure{
				Domains: closure,
				Delta:   delta,
				Size:    len(closure),
			})
		}
	}

	return closures
}

// generateDSLFromClosures converts closures to DSL patterns
func (o *Orchestrator) generateDSLFromClosures(closures []*Closure, minCoverage int) []*DSLPattern {
	patterns := make([]*DSLPattern, 0)

	for _, closure := range closures {
		if len(closure.Domains) < minCoverage {
			continue
		}

		// Generate DSL pattern
		pattern, err := o.dslGenerator.GeneratePattern(closure)
		if err != nil {
			continue
		}

		// Quality filter: ratio check
		if pattern.Ratio > o.modeConfig.MaxRatio {
			continue
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

// partitionByNgram partitions domains by n-gram prefixes
func (o *Orchestrator) partitionByNgram(domains []string, n int) [][]string {
	partitions := make(map[string][]string)

	for _, d := range domains {
		tokens := strings.Split(strings.Split(d, ".")[0], "-")
		ngram := ""
		for i := 0; i < n && i < len(tokens); i++ {
			if i > 0 {
				ngram += "-"
			}
			ngram += tokens[i]
		}
		partitions[ngram] = append(partitions[ngram], d)
	}

	result := make([][]string, 0, len(partitions))
	for _, p := range partitions {
		if len(p) >= 2 {
			result = append(result, p)
		}
	}
	return result
}

// partitionByFirstToken partitions domains by first token
func (o *Orchestrator) partitionByFirstToken(domains []string) [][]string {
	partitions := make(map[string][]string)

	for _, d := range domains {
		tokens := strings.Split(strings.Split(d, ".")[0], "-")
		if len(tokens) > 0 {
			partitions[tokens[0]] = append(partitions[tokens[0]], d)
		}
	}

	result := make([][]string, 0, len(partitions))
	for _, p := range partitions {
		if len(p) >= 2 {
			result = append(result, p)
		}
	}
	return result
}

// limitTokenPartitions limits to top K tokens + rare tokens + merge others
func (o *Orchestrator) limitTokenPartitions(partitions [][]string, maxGroups int) [][]string {
	if len(partitions) <= maxGroups {
		return partitions
	}

	totalSize := 0
	for _, p := range partitions {
		totalSize += len(p)
	}

	// Sort by size
	type tokenGroup struct {
		domains []string
		freq    float64
	}
	groups := make([]tokenGroup, 0, len(partitions))
	for _, p := range partitions {
		groups = append(groups, tokenGroup{
			domains: p,
			freq:    float64(len(p)) / float64(totalSize),
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].domains) > len(groups[j].domains)
	})

	// Keep top K
	result := make([][]string, 0, maxGroups)
	for i := 0; i < maxGroups && i < len(groups); i++ {
		result = append(result, groups[i].domains)
	}

	// Keep rare tokens (< 1% frequency)
	for i := maxGroups; i < len(groups); i++ {
		if groups[i].freq < 0.01 {
			result = append(result, groups[i].domains)
		}
	}

	return result
}

// deduplicatePatterns removes exact duplicate templates
func (o *Orchestrator) deduplicatePatterns(patterns []*DSLPattern) []*DSLPattern {
	seen := make(map[string]*DSLPattern)

	for _, p := range patterns {
		if existing, exists := seen[p.Template]; exists {
			// Keep pattern with higher coverage
			if p.Coverage > existing.Coverage {
				seen[p.Template] = p
			}
		} else {
			seen[p.Template] = p
		}
	}

	result := make([]*DSLPattern, 0, len(seen))
	for _, p := range seen {
		result = append(result, p)
	}
	return result
}

// clusterPatternsAP applies Affinity Propagation clustering using the existing ClusterPatterns function
func (o *Orchestrator) clusterPatternsAP(patterns []*DSLPattern) []*DSLPattern {
	if len(patterns) <= o.modeConfig.MaxPatterns {
		return patterns
	}

	// Configure clustering with mode-specific parameters
	config := ClusterConfig{
		Enabled: true,
		DistanceWeights: DistanceWeights{
			Template:  0.4,
			TokenSeq:  0.3,
			VarStruct: 0.3,
			Domain:    0.0, // Pure structural similarity (no domain overlap)
			Quality:   0.0,
		},
		APConfig: AffinityPropagationConfig{
			MaxIterations: o.modeConfig.APIterations,
			Convergence:   15,
			Damping:       0.5,
			Preference:    0.0, // Auto-determined
		},
		MergeStrategy:  MergeUnionConservative,
		MinClusterSize: 1,
	}

	// Run clustering
	clustered, _, err := ClusterPatterns(patterns, config)
	if err != nil {
		gologger.Warning().Msgf("Clustering failed: %v, using original patterns", err)
		return patterns
	}

	return clustered
}

// selectPatternsByEntropy applies entropy-based pattern selection
func (o *Orchestrator) selectPatternsByEntropy(patterns []*DSLPattern, allDomains []string) []*DSLPattern {
	if len(patterns) == 0 {
		return patterns
	}

	// Sort by coverage (descending)
	sorted := make([]*DSLPattern, len(patterns))
	copy(sorted, patterns)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Coverage > sorted[j].Coverage
	})

	// Incremental selection with marginal coverage tracking
	selected := make([]*DSLPattern, 0, o.modeConfig.MaxPatterns)
	covered := make(map[string]bool)

	totalDomains := len(allDomains)

	for _, pattern := range sorted {
		// Calculate marginal coverage
		newCoverage := 0
		for _, d := range pattern.Domains {
			if !covered[d] {
				newCoverage++
			}
		}

		marginalGain := float64(newCoverage) / float64(totalDomains)

		// Stop if diminishing returns
		if marginalGain < o.modeConfig.ElbowSensitivity {
			if len(selected) >= o.modeConfig.MinPatterns {
				break
			}
		}

		// Stop if coverage goal met
		currentCoverage := float64(len(covered)) / float64(totalDomains)
		if currentCoverage >= o.modeConfig.TargetCoverage {
			if len(selected) >= o.modeConfig.MinPatterns {
				break
			}
		}

		// Add pattern
		selected = append(selected, pattern)
		for _, d := range pattern.Domains {
			covered[d] = true
		}

		// Stop if max patterns reached
		if len(selected) >= o.modeConfig.MaxPatterns {
			break
		}
	}

	// Safety floor: ensure minimum patterns
	if len(selected) < o.modeConfig.MinPatterns && len(sorted) >= o.modeConfig.MinPatterns {
		selected = sorted[:o.modeConfig.MinPatterns]
	}

	return selected
}

// enrichPatterns adds optional variable support
func (o *Orchestrator) enrichPatterns(patterns []*DSLPattern) []*DSLPattern {
	enriched := make([]*DSLPattern, len(patterns))
	copy(enriched, patterns)

	for _, pattern := range enriched {
		for i := range pattern.Variables {
			variable := &pattern.Variables[i]

			// Numbers ALWAYS optional
			if variable.NumberRange != nil {
				variable.Payloads = []string{""}
				continue
			}

			// Semantic modifiers optional based on enrichment rate
			if o.isSemanticVariable(variable.Name) {
				if rand.Float64() < o.modeConfig.EnrichmentRate {
					// Add empty string as first payload
					variable.Payloads = append([]string{""}, variable.Payloads...)
				}
			}
		}
	}

	return enriched
}

// isSemanticVariable checks if variable is semantic
func (o *Orchestrator) isSemanticVariable(name string) bool {
	semanticNames := []string{"env", "region", "service", "stage", "tier"}
	for _, s := range semanticNames {
		if name == s {
			return true
		}
	}
	return false
}

// calculateCoverage calculates coverage ratio
func (o *Orchestrator) calculateCoverage(patterns []*DSLPattern, allDomains []string) float64 {
	covered := make(map[string]bool)
	for _, p := range patterns {
		for _, d := range p.Domains {
			covered[d] = true
		}
	}
	return float64(len(covered)) / float64(len(allDomains))
}

// Helper functions

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}


func sampleRandom(items []string, n int) []string {
	if n >= len(items) {
		return items
	}
	indices := rand.Perm(len(items))[:n]
	result := make([]string, n)
	for i, idx := range indices {
		result[i] = items[idx]
	}
	return result
}

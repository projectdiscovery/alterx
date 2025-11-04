package inducer

import (
	"math"
	"sort"
	"strings"
)

// EntropyBudgetConfig configures the entropy-based pattern budget calculator
type EntropyBudgetConfig struct {
	// MinPatterns is the safety floor (always keep at least this many)
	MinPatterns int

	// MaxPatterns is the safety ceiling (never exceed this)
	MaxPatterns int

	// TargetCoverage is the desired domain coverage ratio (0.0 to 1.0)
	// Default: 0.90 (90% of domains should be covered)
	TargetCoverage float64

	// ElbowSensitivity is the minimum marginal return to continue adding patterns
	// If marginal return drops below this, stop adding patterns
	// Default: 0.05 (5% marginal return threshold)
	ElbowSensitivity float64
}

// DefaultEntropyBudgetConfig returns sensible defaults
// Tuned for COVERAGE-FIRST approach: generate many patterns, prune conservatively
func DefaultEntropyBudgetConfig() EntropyBudgetConfig {
	return EntropyBudgetConfig{
		MinPatterns:      5,    // Increased from 3 to ensure good coverage floor
		MaxPatterns:      25,   // Increased from 20 to allow more diverse patterns
		TargetCoverage:   0.90, // 90% coverage target
		ElbowSensitivity: 0.02, // Reduced from 0.05 to 0.02 (2% = prioritize coverage over efficiency)
	}
}

// PatternBudgetCalculator calculates optimal pattern budgets using entropy analysis
type PatternBudgetCalculator struct {
	config EntropyBudgetConfig

	// Cache to avoid recomputation
	domainSetCache map[*DSLPattern]map[string]bool
}

// NewPatternBudgetCalculator creates a new calculator with the given config
func NewPatternBudgetCalculator(config EntropyBudgetConfig) *PatternBudgetCalculator {
	return &PatternBudgetCalculator{
		config:         config,
		domainSetCache: make(map[*DSLPattern]map[string]bool),
	}
}

// BudgetDecision contains detailed information about budget calculation
type BudgetDecision struct {
	OptimalCount       int
	StructuralEntropy  float64
	CoverageEfficiency float64
	ElbowPoint         int
	ActualCoverage     float64
	Reasoning          string
}

// CalculateOptimalBudget determines the optimal number of patterns to keep
// Returns the number of patterns that should be selected
func (pbc *PatternBudgetCalculator) CalculateOptimalBudget(
	patterns []*DSLPattern,
	totalDomains []string,
) int {
	decision := pbc.CalculateOptimalBudgetWithMetrics(patterns, totalDomains)
	return decision.OptimalCount
}

// CalculateOptimalBudgetWithMetrics returns detailed budget calculation info
func (pbc *PatternBudgetCalculator) CalculateOptimalBudgetWithMetrics(
	patterns []*DSLPattern,
	totalDomains []string,
) BudgetDecision {
	// Edge case: Very few patterns
	if len(patterns) <= pbc.config.MinPatterns {
		return BudgetDecision{
			OptimalCount: len(patterns),
			Reasoning:    "Pattern count below minimum threshold",
		}
	}

	// Edge case: Empty domain list (shouldn't happen, but be safe)
	if len(totalDomains) == 0 {
		return BudgetDecision{
			OptimalCount: pbc.config.MinPatterns,
			Reasoning:    "Empty domain list, using minimum",
		}
	}

	// Pre-compute domain sets for fast lookup (space optimization)
	pbc.buildDomainSetCache(patterns)

	// Calculate structural metrics
	structuralEntropy := pbc.calculateStructuralEntropy(patterns)
	coverageEfficiency := pbc.calculateCoverageEfficiency(patterns, len(totalDomains))

	// Check for uniform distribution (all patterns have similar coverage)
	if pbc.isUniformDistribution(patterns) {
		targetCount := int(math.Ceil(float64(len(patterns)) * pbc.config.TargetCoverage))
		targetCount = clamp(targetCount, pbc.config.MinPatterns, pbc.config.MaxPatterns)

		// Calculate actual coverage for the selected count
		sortedPatterns := make([]*DSLPattern, len(patterns))
		copy(sortedPatterns, patterns)
		sort.Slice(sortedPatterns, func(i, j int) bool {
			return sortedPatterns[i].Coverage > sortedPatterns[j].Coverage
		})

		actualCoverage := 0.0
		if targetCount <= len(sortedPatterns) {
			coveredCount := countCoveredDomains(sortedPatterns[:targetCount], pbc.domainSetCache)
			actualCoverage = float64(coveredCount) / float64(len(totalDomains))
		}

		return BudgetDecision{
			OptimalCount:       targetCount,
			StructuralEntropy:  structuralEntropy,
			CoverageEfficiency: coverageEfficiency,
			ActualCoverage:     actualCoverage,
			Reasoning:          "Uniform distribution detected, using target coverage ratio",
		}
	}

	// Find the elbow point using marginal returns analysis
	elbowResult := pbc.findCoverageElbow(patterns, len(totalDomains))

	// Apply constraints
	optimalCount := clamp(elbowResult.elbowPoint, pbc.config.MinPatterns, pbc.config.MaxPatterns)

	return BudgetDecision{
		OptimalCount:       optimalCount,
		StructuralEntropy:  structuralEntropy,
		CoverageEfficiency: coverageEfficiency,
		ElbowPoint:         elbowResult.elbowPoint,
		ActualCoverage:     elbowResult.coverageAtElbow,
		Reasoning:          elbowResult.reasoning,
	}
}

// buildDomainSetCache pre-computes domain sets for O(1) lookup
func (pbc *PatternBudgetCalculator) buildDomainSetCache(patterns []*DSLPattern) {
	for _, p := range patterns {
		if _, exists := pbc.domainSetCache[p]; !exists {
			domainSet := make(map[string]bool, len(p.Domains))
			for _, domain := range p.Domains {
				domainSet[domain] = true
			}
			pbc.domainSetCache[p] = domainSet
		}
	}
}

// calculateStructuralEntropy measures how diverse the pattern structures are
// Returns Shannon entropy of pattern template structures
func (pbc *PatternBudgetCalculator) calculateStructuralEntropy(patterns []*DSLPattern) float64 {
	if len(patterns) == 0 {
		return 0.0
	}

	// Group patterns by structural signature
	structureFreq := make(map[string]int)
	for _, p := range patterns {
		structure := extractStructure(p.Template)
		structureFreq[structure]++
	}

	// Calculate Shannon entropy: H = -Σ p(x) * log2(p(x))
	total := float64(len(patterns))
	entropy := 0.0
	for _, count := range structureFreq {
		p := float64(count) / total
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// extractStructure converts a DSL template to a structural signature
// Example: "{{service}}-{{env}}.{{root}}" → "V-V.V" (V = variable)
func extractStructure(template string) string {
	var structure strings.Builder
	inVariable := false

	for _, ch := range template {
		if ch == '{' {
			inVariable = true
			continue
		}
		if ch == '}' {
			inVariable = false
			structure.WriteRune('V') // V = variable placeholder
			continue
		}
		if !inVariable {
			structure.WriteRune(ch)
		}
	}

	return structure.String()
}

// calculateCoverageEfficiency measures how efficiently patterns cover domains
// Returns average domains covered per pattern normalized by total domains
func (pbc *PatternBudgetCalculator) calculateCoverageEfficiency(patterns []*DSLPattern, totalDomains int) float64 {
	if len(patterns) == 0 || totalDomains == 0 {
		return 0.0
	}

	totalCoverage := 0
	for _, p := range patterns {
		totalCoverage += p.Coverage
	}

	avgCoveragePerPattern := float64(totalCoverage) / float64(len(patterns))
	return avgCoveragePerPattern / float64(totalDomains)
}

// isUniformDistribution checks if all patterns have similar coverage
func (pbc *PatternBudgetCalculator) isUniformDistribution(patterns []*DSLPattern) bool {
	if len(patterns) < 3 {
		return false
	}

	// Calculate coefficient of variation (CV = stddev / mean)
	// Low CV (<0.3) indicates uniform distribution
	coverages := make([]float64, len(patterns))
	sum := 0.0
	for i, p := range patterns {
		coverages[i] = float64(p.Coverage)
		sum += coverages[i]
	}

	mean := sum / float64(len(patterns))
	if mean == 0 {
		return true // All zeros = uniform
	}

	variance := 0.0
	for _, cov := range coverages {
		diff := cov - mean
		variance += diff * diff
	}
	variance /= float64(len(patterns))
	stddev := math.Sqrt(variance)

	cv := stddev / mean
	return cv < 0.3 // Threshold for "uniform"
}

// elbowResult contains the result of elbow detection
type elbowResult struct {
	elbowPoint      int
	coverageAtElbow float64
	reasoning       string
}

// findCoverageElbow finds the point where adding more patterns gives diminishing returns
func (pbc *PatternBudgetCalculator) findCoverageElbow(patterns []*DSLPattern, totalDomainCount int) elbowResult {
	// Sort patterns by coverage descending (greedy selection)
	sortedPatterns := make([]*DSLPattern, len(patterns))
	copy(sortedPatterns, patterns)
	sort.Slice(sortedPatterns, func(i, j int) bool {
		// Primary: Coverage (descending)
		if sortedPatterns[i].Coverage != sortedPatterns[j].Coverage {
			return sortedPatterns[i].Coverage > sortedPatterns[j].Coverage
		}
		// Secondary: Confidence (descending)
		if sortedPatterns[i].Confidence != sortedPatterns[j].Confidence {
			return sortedPatterns[i].Confidence > sortedPatterns[j].Confidence
		}
		// Tertiary: Ratio (ascending - lower is better)
		return sortedPatterns[i].Ratio < sortedPatterns[j].Ratio
	})

	// Track unique domains covered
	coveredDomains := make(map[string]bool)
	marginalReturns := make([]float64, 0, len(sortedPatterns))

	for i, p := range sortedPatterns {
		// Measure marginal contribution of this pattern
		beforeCount := len(coveredDomains)

		// Add domains from this pattern (use cached domain set)
		domainSet := pbc.domainSetCache[p]
		for domain := range domainSet {
			coveredDomains[domain] = true
		}

		afterCount := len(coveredDomains)
		newDomainsCovered := afterCount - beforeCount

		// Marginal return: percentage of total domains covered by this pattern alone
		marginalReturn := float64(newDomainsCovered) / float64(totalDomainCount)
		marginalReturns = append(marginalReturns, marginalReturn)

		// Early exit: Check if we've hit target coverage
		currentCoverage := float64(afterCount) / float64(totalDomainCount)
		if currentCoverage >= pbc.config.TargetCoverage {
			return elbowResult{
				elbowPoint:      i + 1,
				coverageAtElbow: currentCoverage,
				reasoning:       "Target coverage reached",
			}
		}

		// Early exit: Check if marginal returns dropped below sensitivity threshold
		if i > pbc.config.MinPatterns && marginalReturn < pbc.config.ElbowSensitivity {
			return elbowResult{
				elbowPoint:      i,
				coverageAtElbow: currentCoverage,
				reasoning:       "Marginal returns below sensitivity threshold",
			}
		}
	}

	// If we get here, we processed all patterns without hitting target coverage
	// Try to find elbow using second derivative
	if len(marginalReturns) >= 3 {
		elbowIdx := findElbowPointSecondDerivative(marginalReturns)
		if elbowIdx > 0 {
			coverage := float64(countCoveredDomains(sortedPatterns[:elbowIdx], pbc.domainSetCache)) / float64(totalDomainCount)
			return elbowResult{
				elbowPoint:      elbowIdx,
				coverageAtElbow: coverage,
				reasoning:       "Elbow detected via second derivative",
			}
		}
	}

	// Fallback: Use all patterns (they all contribute)
	finalCoverage := float64(len(coveredDomains)) / float64(totalDomainCount)
	return elbowResult{
		elbowPoint:      len(sortedPatterns),
		coverageAtElbow: finalCoverage,
		reasoning:       "All patterns contribute meaningfully",
	}
}

// findElbowPointSecondDerivative finds the elbow using second derivative
// Returns the index where the sharpest drop in marginal returns occurs
func findElbowPointSecondDerivative(marginalReturns []float64) int {
	if len(marginalReturns) < 3 {
		return len(marginalReturns)
	}

	// Calculate second derivative (rate of change of marginal returns)
	secondDerivatives := make([]float64, len(marginalReturns)-2)
	for i := 1; i < len(marginalReturns)-1; i++ {
		// Second derivative: f''(x) = f(x-1) - 2*f(x) + f(x+1)
		secondDerivatives[i-1] = marginalReturns[i-1] - 2*marginalReturns[i] + marginalReturns[i+1]
	}

	// Find the point with the most negative second derivative (sharpest drop)
	minIdx := 0
	minVal := secondDerivatives[0]
	for i, val := range secondDerivatives {
		if val < minVal {
			minVal = val
			minIdx = i
		}
	}

	// Only accept if the drop is significant (absolute value > 0.01)
	if math.Abs(minVal) < 0.01 {
		return -1 // No clear elbow
	}

	return minIdx + 2 // Adjust for derivative offset
}

// countCoveredDomains counts unique domains covered by a set of patterns
func countCoveredDomains(patterns []*DSLPattern, domainSetCache map[*DSLPattern]map[string]bool) int {
	covered := make(map[string]bool)
	for _, p := range patterns {
		if domainSet, ok := domainSetCache[p]; ok {
			for domain := range domainSet {
				covered[domain] = true
			}
		}
	}
	return len(covered)
}

// clamp restricts a value to the range [min, max]
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

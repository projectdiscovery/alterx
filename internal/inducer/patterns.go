package inducer

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ============================================================================
// DEPRECATED: This file contains the OLD regex-based pattern generation approach
//
// Status: AWAITING DELETION - Pending orchestrator integration completion
//
// The new architecture uses DSLGenerator (dsl_generator.go) which generates
// AlterX DSL patterns directly, bypassing regex entirely. This is simpler,
// more maintainable, and more semantic.
//
// This file will be DELETED once:
// 1. Orchestrator is updated to use DSLGenerator instead of PatternGenerator
// 2. induction.go is updated to work with DSLPattern instead of Pattern
// 3. All tests pass with the new approach
//
// DO NOT USE PatternGenerator for new code. Use DSLGenerator instead.
// ============================================================================

// Pattern represents a learned regex pattern
type Pattern struct {
	Regex     string   // The generated regex pattern
	Coverage  int      // Number of domains this pattern covers
	Domains   []string // The actual domains used to generate this pattern
	Ratio     float64  // Generation ratio (for quality filtering)
	Confidence float64  // Quality score (0-1)
}

// PatternGenerator converts closures into regex patterns
// Implements the closure_to_regex algorithm from regulator
// Patterns are root-agnostic (only represent subdomain structure)
//
// DEPRECATED: Use DSLGenerator instead
type PatternGenerator struct{}

// NewPatternGenerator creates a new pattern generator
//
// DEPRECATED: Use NewDSLGenerator instead
func NewPatternGenerator() *PatternGenerator {
	return &PatternGenerator{}
}

// GeneratePattern converts a closure into a regex pattern
// This is the main implementation of the closure_to_regex algorithm
func (pg *PatternGenerator) GeneratePattern(closure *Closure) (*Pattern, error) {
	if len(closure.Domains) == 0 {
		return nil, fmt.Errorf("empty closure")
	}

	if len(closure.Domains) == 1 {
		// Single domain - no pattern needed
		return nil, fmt.Errorf("single domain closure")
	}

	// Step 1: Tokenize all domains in the closure
	tokenized := make([]*TokenizedDomain, 0, len(closure.Domains))
	for _, domain := range closure.Domains {
		td, err := Tokenize(domain)
		if err != nil {
			continue
		}
		tokenized = append(tokenized, td)
	}

	if len(tokenized) < 2 {
		return nil, fmt.Errorf("not enough tokenized domains")
	}

	// Step 2: Build level-position map
	levelMap := pg.buildLevelPositionMap(tokenized)

	// Step 3: Generate regex from level map
	regex := pg.generateRegexFromLevelMap(levelMap, tokenized)

	// Pattern is root-agnostic (no root domain appended)
	// DSL converter will add {{suffix}} placeholder later

	// Calculate confidence using the formula from config_format.md
	// confidence = (0.85 * ratio_score) + (0.15 * coverage_score)
	// where ratio_score = 1.0 / ratio and coverage_score = min(1.0, log10(coverage) / 3.0)
	confidence := calculateConfidence(len(closure.Domains), 0)

	pattern := &Pattern{
		Regex:      regex,
		Coverage:   len(closure.Domains),
		Domains:    closure.Domains,
		Confidence: confidence,
	}

	return pattern, nil
}

// buildLevelPositionMap creates the level-position-tokens map
// Structure: level → position → set of token values
func (pg *PatternGenerator) buildLevelPositionMap(tokenized []*TokenizedDomain) map[int]map[int]map[string]bool {
	levelMap := make(map[int]map[int]map[string]bool)

	for _, td := range tokenized {
		for levelIdx, level := range td.Levels {
			// Ensure level exists
			if levelMap[levelIdx] == nil {
				levelMap[levelIdx] = make(map[int]map[string]bool)
			}

			// Process each token at this level
			for _, token := range level.Tokens {
				pos := token.Position

				// Ensure position exists
				if levelMap[levelIdx][pos] == nil {
					levelMap[levelIdx][pos] = make(map[string]bool)
				}

				// Add token value
				levelMap[levelIdx][pos][token.Value] = true
			}
		}
	}

	// Check for missing levels/positions and add None markers
	maxLevel := pg.getMaxLevel(tokenized)
	for levelIdx := 0; levelIdx <= maxLevel; levelIdx++ {
		for _, td := range tokenized {
			if levelIdx >= len(td.Levels) {
				// This domain doesn't have this level
				if levelMap[levelIdx] == nil {
					levelMap[levelIdx] = make(map[int]map[string]bool)
				}
				if levelMap[levelIdx][0] == nil {
					levelMap[levelIdx][0] = make(map[string]bool)
				}
				levelMap[levelIdx][0][""] = true // Empty string marks optional level
			}
		}
	}

	return levelMap
}

// generateRegexFromLevelMap converts the level-position map into a regex string
func (pg *PatternGenerator) generateRegexFromLevelMap(levelMap map[int]map[int]map[string]bool, tokenized []*TokenizedDomain) string {
	// Get sorted level indices
	levels := make([]int, 0, len(levelMap))
	for levelIdx := range levelMap {
		levels = append(levels, levelIdx)
	}
	sort.Ints(levels)

	regexParts := []string{}

	for _, levelIdx := range levels {
		positions := levelMap[levelIdx]

		// Get sorted position indices
		posIndices := make([]int, 0, len(positions))
		for pos := range positions {
			posIndices = append(posIndices, pos)
		}
		sort.Ints(posIndices)

		levelRegex := ""

		for _, pos := range posIndices {
			tokens := positions[pos]

			// Convert token set to sorted slice
			tokenList := make([]string, 0, len(tokens))
			for token := range tokens {
				if token != "" { // Skip empty markers
					tokenList = append(tokenList, token)
				}
			}

			if len(tokenList) == 0 {
				continue // Skip empty positions
			}

			sort.Strings(tokenList)

			if len(tokenList) == 1 {
				// Single token - no alternation needed
				levelRegex += escapeRegex(tokenList[0])
			} else {
				// Multiple tokens - create alternation
				escaped := make([]string, len(tokenList))
				for i, token := range tokenList {
					escaped[i] = escapeRegex(token)
				}
				levelRegex += "(" + strings.Join(escaped, "|") + ")"
			}
		}

		// Check if level is optional
		isOptional := pg.isLevelOptional(levelIdx, tokenized)

		if levelIdx > 0 && levelRegex != "" {
			// Add dot separator for non-first levels
			if isOptional {
				regexParts = append(regexParts, "(\\."+levelRegex+")?")
			} else {
				regexParts = append(regexParts, "\\."+levelRegex)
			}
		} else if levelRegex != "" {
			// First level (no dot prefix)
			if isOptional {
				regexParts = append(regexParts, "("+levelRegex+")?")
			} else {
				regexParts = append(regexParts, levelRegex)
			}
		}
	}

	return strings.Join(regexParts, "")
}

// isLevelOptional checks if a level is missing in some domains
func (pg *PatternGenerator) isLevelOptional(levelIdx int, tokenized []*TokenizedDomain) bool {
	missingCount := 0

	for _, td := range tokenized {
		if levelIdx >= len(td.Levels) {
			missingCount++
		}
	}

	// If any domain is missing this level, it's optional
	return missingCount > 0
}

// getMaxLevel returns the maximum level index across all tokenized domains
func (pg *PatternGenerator) getMaxLevel(tokenized []*TokenizedDomain) int {
	maxLevel := 0
	for _, td := range tokenized {
		if len(td.Levels)-1 > maxLevel {
			maxLevel = len(td.Levels) - 1
		}
	}
	return maxLevel
}

// escapeRegex escapes special regex characters in a token
func escapeRegex(s string) string {
	// IMPORTANT: Escape backslash first to avoid double-escaping
	result := strings.ReplaceAll(s, "\\", "\\\\")

	// Then escape other special regex characters
	replacements := []struct{ old, new string }{
		{".", "\\."},
		{"*", "\\*"},
		{"+", "\\+"},
		{"?", "\\?"},
		{"[", "\\["},
		{"]", "\\]"},
		{"(", "\\("},
		{")", "\\)"},
		{"{", "\\{"},
		{"}", "\\}"},
		{"^", "\\^"},
		{"$", "\\$"},
		{"|", "\\|"},
	}

	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}

	return result
}

// GeneratePatternsFromClosures converts multiple closures into patterns
func (pg *PatternGenerator) GeneratePatternsFromClosures(closures []*Closure) []*Pattern {
	patterns := make([]*Pattern, 0, len(closures))

	for _, closure := range closures {
		pattern, err := pg.GeneratePattern(closure)
		if err != nil {
			continue
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

// calculateConfidence implements the confidence scoring formula from config_format.md
//
// Formula:
//   confidence = (0.85 * ratio_score) + (0.15 * coverage_score)
//   where:
//     ratio_score = 1.0 / ratio (ratio = possible_generations / observed_count)
//     coverage_score = min(1.0, log10(coverage) / 3.0)
//
// Parameters:
//   coverage: Number of domains this pattern covers
//   ratio: Generation ratio (possible_generations / observed_count)
//          If ratio is 0, it's calculated later and confidence is based on coverage only
//
// Returns:
//   Confidence score between 0.0 and 1.0 (higher is better)
//
// Interpretation:
//   - 0.84 → 84% of generated subdomains will be valid (excellent)
//   - 0.53 → 53% of generated subdomains will be valid (moderate)
//   - 0.36 → 36% of generated subdomains will be valid (low quality)
func calculateConfidence(coverage int, ratio float64) float64 {
	// Handle edge cases
	if coverage <= 0 {
		return 0.0
	}

	// Calculate coverage score (15% weight, logarithmic scale)
	// log10(coverage) / 3.0 gives:
	//   coverage = 10   → log10(10) / 3   = 0.33
	//   coverage = 100  → log10(100) / 3  = 0.67
	//   coverage = 1000 → log10(1000) / 3 = 1.00
	coverageScore := math.Min(1.0, math.Log10(float64(coverage))/3.0)

	// Calculate ratio score (85% weight)
	// ratio_score = 1.0 / ratio
	// If ratio is 0 or not yet calculated, use conservative estimate
	var ratioScore float64
	if ratio > 0 {
		ratioScore = 1.0 / ratio
	} else {
		// No ratio provided - assume perfect ratio (1.0)
		ratioScore = 1.0
	}

	// Clamp ratioScore to [0, 1] range
	ratioScore = math.Min(1.0, math.Max(0.0, ratioScore))

	// Final confidence: weighted average
	confidence := (0.85 * ratioScore) + (0.15 * coverageScore)

	// Ensure confidence is in [0, 1] range
	return math.Min(1.0, math.Max(0.0, confidence))
}

// UpdateConfidence recalculates confidence after ratio is determined
// This should be called after the pattern's Ratio field is set by quality filtering
func (p *Pattern) UpdateConfidence() {
	if p == nil {
		return
	}
	p.Confidence = calculateConfidence(p.Coverage, p.Ratio)
}

package inducer

import (
	"fmt"
	"sort"
	"strings"
)

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
type PatternGenerator struct {
	rootDomain string // The root domain (e.g., "example.com")
}

// NewPatternGenerator creates a new pattern generator
func NewPatternGenerator(rootDomain string) *PatternGenerator {
	return &PatternGenerator{
		rootDomain: rootDomain,
	}
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

	// Step 4: Add root domain
	if pg.rootDomain != "" {
		regex = regex + "." + pg.rootDomain
	}

	pattern := &Pattern{
		Regex:      regex,
		Coverage:   len(closure.Domains),
		Domains:    closure.Domains,
		Confidence: 1.0, // TODO: Implement quality scoring
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

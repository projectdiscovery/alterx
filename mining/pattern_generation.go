package mining

import (
	"fmt"
	"sort"
	"strings"
)

// Mined pattern representation in DSL format
type DSLPattern struct {
	ID       string                 `json:"id"`
	Metadata map[string]interface{} `json:"metadata"`
	Pattern  string                 `json:"pattern"`
	Payloads map[string][]string    `json:"payloads"`
}

// generatePattern generates a simplified DSL pattern from a set of subdomain strings.
//
// SIMPLIFIED ALGORITHM (matches Python reference):
// 1. Sort the subdomains for consistency
// 2. Find the common prefix across all subdomains
// 3. If common prefix exists: return "{common}{{p0}}"
// 4. If no common prefix: return "{{p0}}"
//
// EXAMPLE:
//
//	Input:  ["api-prod", "api-staging"]
//	Output: DSLPattern{
//	  Pattern: "api-{{p0}}",
//	  Payloads: {"p0": ["prod", "staging"]}
//	}
//
//	Input:  ["web", "api", "app"]
//	Output: DSLPattern{
//	  Pattern: "{{p0}}",
//	  Payloads: {"p0": ["api", "app", "web"]}
//	}
//
// RETURNS:
//   - *DSLPattern with pattern string and payloads map
//   - error if generation fails
func (p *PatternMiner) generatePattern(subdomains []string) (*DSLPattern, error) {
	if len(subdomains) == 0 {
		return nil, nil
	}

	// Single subdomain - return as-is
	if len(subdomains) == 1 {
		return &DSLPattern{
			Pattern:  subdomains[0],
			Payloads: make(map[string][]string),
			Metadata: make(map[string]interface{}),
		}, nil
	}

	// Sort for consistency (matches Python)
	sorted := make([]string, len(subdomains))
	copy(sorted, subdomains)
	sort.Strings(sorted)

	// Check if all subdomains are identical
	allSame := true
	for _, s := range sorted[1:] {
		if s != sorted[0] {
			allSame = false
			break
		}
	}

	// If all subdomains are identical, return as static pattern (no variables)
	if allSame {
		return &DSLPattern{
			Pattern:  sorted[0],
			Payloads: make(map[string][]string),
			Metadata: make(map[string]interface{}),
		}, nil
	}

	// Find common prefix length across all subdomains
	first := sorted[0]
	commonLen := 0
	for i := 0; i < len(first); i++ {
		allMatch := true
		for _, s := range sorted {
			if i >= len(s) || s[i] != first[i] {
				allMatch = false
				break
			}
		}
		if allMatch {
			commonLen = i + 1
		} else {
			break
		}
	}

	var pattern string
	var payloads map[string][]string

	if commonLen > 0 {
		// Has common prefix
		common := first[:commonLen]
		pattern = common + "{{p0}}"

		// Extract suffixes as payload
		suffixes := make(map[string]struct{})
		for _, s := range sorted {
			suffix := s[commonLen:]
			suffixes[suffix] = struct{}{}
		}

		// Convert to sorted slice
		payloadValues := make([]string, 0, len(suffixes))
		for suffix := range suffixes {
			payloadValues = append(payloadValues, suffix)
		}
		sort.Strings(payloadValues)

		payloads = map[string][]string{
			"p0": payloadValues,
		}
	} else {
		// No common prefix
		pattern = "{{p0}}"

		// All subdomains become payload
		payloads = map[string][]string{
			"p0": sorted,
		}
	}

	dslPattern := &DSLPattern{
		Pattern:  pattern,
		Payloads: payloads,
		Metadata: make(map[string]interface{}),
	}

	// Max length check
	if p.options.MaxPatternLength > 0 && len(pattern) > p.options.MaxPatternLength {
		return nil, nil
	}

	// Quality check
	if !p.isGoodPattern(dslPattern, len(subdomains)) {
		return nil, nil
	}

	return dslPattern, nil
}

// analyzeTokenAlignment analyzes token positions across multiple tokenized subdomains.
//
// ALGORITHM:
//  1. Build hierarchical map: levels[levelIdx][positionIdx] = set of unique tokens
//  2. For each level and position, collect all token values across all subdomains
//  3. Classify each position:
//     - STATIC: len(unique_values) == 1 (all subdomains have same token)
//     - VARIABLE: len(unique_values) > 1 (different tokens exist)
//  4. Detect OPTIONAL positions and levels:
//     - Position optional: not all subdomains (with that level) have token at that position
//     - Level optional: not all subdomains have that level
//
// SPECIAL CASE EXAMPLES:
//
// Example 1 - Optional Position:
//   Input: ["api-prod", "api"]
//   Level 0, Position 1: only "api-prod" has "-prod" → Position 1 is OPTIONAL
//   Result: pattern "api{{p0}}", payloads: {"p0": ["-prod", ""]}
//
// Example 2 - Optional Level:
//   Input: ["api.dev", "api"]
//   Level 1: only "api.dev" has "dev" → Level 1 is OPTIONAL
//   Result: pattern "api.{{p0}}", payloads: {"p0": ["dev", ""]}
//
// Example 3 - Variable Position:
//   Input: ["api-prod-1", "api-staging-2"]
//   Level 0, Position 1: has {"-prod", "-staging"} → VARIABLE
//   Level 0, Position 2: has {"-1", "-2"} → VARIABLE
//   Result: pattern "api{{p0}}{{p1}}"
//
// RETURNS: []LevelPosition with classification and optionality metadata
func (p *PatternMiner) analyzeTokenAlignment(tokenized []TokenizedSubdomain) []LevelPosition {
	if len(tokenized) == 0 {
		return nil
	}

	// Build hierarchical map: levels[levelIdx][positionIdx] = set of unique values
	// This allows us to see all variations at each position to detect STATIC vs VARIABLE
	levels := make(map[int]map[int]map[string]struct{})

	// Track all occurrences (including duplicates) to detect OPTIONAL positions
	// If len(optional[level][pos]) < totalMembers, position is optional
	optional := make(map[int]map[int][]string)

	// STEP 1: Collect all tokens at each level and position across all subdomains
	for _, ts := range tokenized {
		for levelIdx, level := range ts.Levels {
			if _, ok := levels[levelIdx]; !ok {
				levels[levelIdx] = make(map[int]map[string]struct{})
				optional[levelIdx] = make(map[int][]string)
			}

			for posIdx, token := range level.Tokens {
				if _, ok := levels[levelIdx][posIdx]; !ok {
					levels[levelIdx][posIdx] = make(map[string]struct{})
					optional[levelIdx][posIdx] = []string{}
				}
				levels[levelIdx][posIdx][token] = struct{}{}          // unique values
				optional[levelIdx][posIdx] = append(optional[levelIdx][posIdx], token) // all occurrences
			}
		}
	}

	// STEP 2: Build LevelPosition structures with classification
	result := make([]LevelPosition, 0)
	totalMembers := len(tokenized)
	varCounter := 0 // Sequential counter for variable names: p0, p1, p2, ...

	for levelIdx := 0; levelIdx < len(levels); levelIdx++ {
		levelData := levels[levelIdx]
		lp := LevelPosition{
			LevelIndex: levelIdx,
			Positions:  make([]TokenPosition, 0),
		}

		// SPECIAL CASE: Detect if entire level is optional
		// Level is optional when some subdomains lack this level entirely
		// Example: ["api", "api.dev"] → Level 1 is optional (only second has it)
		membersWithLevel := 0
		for _, ts := range tokenized {
			if levelIdx < len(ts.Levels) {
				membersWithLevel++
			}
		}
		lp.IsOptional = membersWithLevel < totalMembers

		// Analyze each position in this level
		for posIdx := 0; posIdx < len(levelData); posIdx++ {
			if tokens, ok := levelData[posIdx]; ok {
				tp := TokenPosition{
					Index:  posIdx,
					Values: make([]string, 0, len(tokens)),
				}

				// Collect unique values at this position
				for token := range tokens {
					tp.Values = append(tp.Values, token)
				}

				// STEP 1: Detect if position is optional
				// Position is optional when NOT ALL subdomains (that have this level)
				// have a token at this position
				// Example: ["api-prod", "api"] → Position 1 ("-prod") is optional
				positionCount := len(optional[levelIdx][posIdx])
				tp.IsOptional = positionCount < totalMembers

				// STEP 2: Classify position as Static or Variable
				// A position is VARIABLE if:
				// 1. Multiple unique values exist (len > 1), OR
				// 2. Position is optional (needs empty string in payload)
				//
				// CRITICAL: Optional positions with single value must be VARIABLE
				// Example: ["api-prod", "api"] → position 1 has ["-prod"] but is optional
				//          Must be VARIABLE to allow both "api-prod" and "api" generation
				if len(tp.Values) > 1 || tp.IsOptional {
					tp.Type = TokenPositionVariable
					// Variable positions get placeholder name: p0, p1, p2, ...
					tp.VarName = fmt.Sprintf("p%d", varCounter)
					varCounter++
				} else {
					tp.Type = TokenPositionStatic
					// Static positions use literal value in pattern
				}

				lp.Positions = append(lp.Positions, tp)
			}
		}

		result = append(result, lp)
	}

	return result
}

// TokenPosition represents metadata about a token position in the pattern.
type TokenPosition struct {
	Index      int               // Position index in token array
	Type       TokenPositionType // Static or Variable
	Values     []string          // All values seen at this position
	VarName    string            // Variable name if Type is Variable (e.g., "p0", "p1")
	IsOptional bool              // Whether this position is optional (not all members have it)
}

// LevelPosition represents all token positions within a single hierarchical level.
type LevelPosition struct {
	LevelIndex int              // Index of this level in the hierarchy (0 = leftmost subdomain part)
	Positions  []TokenPosition  // Token positions within this level
	IsOptional bool             // Whether this entire level is optional
}

// TokenPositionType indicates whether a token position is static or variable.
type TokenPositionType int

const (
	// TokenPositionStatic indicates all subdomains have same value at this position
	TokenPositionStatic TokenPositionType = iota
	// TokenPositionVariable indicates subdomains have different values at this position
	TokenPositionVariable
)

// buildDSLPattern constructs a DSL pattern string from level position analysis.
//
// ALGORITHM:
// 1. Iterate through levels and their token positions
// 2. For static positions: use literal value
// 3. For variable positions: use placeholder syntax (e.g., {{p0}})
// 4. Join levels with dots, positions within levels directly
//
// EXAMPLE:
//
//	Input:  Level 0: [Static("api"), Variable("p0"), Variable("p1")]
//	Output: "api{{p0}}{{p1}}"
func (p *PatternMiner) buildDSLPattern(levelPositions []LevelPosition) string {
	if len(levelPositions) == 0 {
		return ""
	}

	levels := make([]string, 0, len(levelPositions))

	for _, lp := range levelPositions {
		levelPattern := ""

		for _, tp := range lp.Positions {
			if tp.Type == TokenPositionStatic {
				// Use literal value for static positions
				if len(tp.Values) > 0 {
					levelPattern += tp.Values[0]
				}
			} else {
				// Use placeholder for variable positions
				levelPattern += "{{" + tp.VarName + "}}"
			}
		}

		levels = append(levels, levelPattern)
	}

	// Join levels with dots
	return strings.Join(levels, ".")
}

// extractPayloads extracts payload values for each variable in the pattern.
//
// ALGORITHM:
// 1. For each variable position in the pattern
// 2. Collect all unique values seen at that position
// 3. IMPORTANT: Add empty string "" if position is optional
//    (this allows the variable to be omitted when generating domains)
// 4. Build a map of variable_name → []values
//
// EXAMPLES:
//
// Example 1 - Required positions:
//   Pattern: "api{{p0}}{{p1}}"
//   Subdomains: ["api-prod-1", "api-prod-2", "api-staging-1"]
//   Output: {"p0": ["-prod", "-staging"], "p1": ["-1", "-2"]}
//
// Example 2 - Optional position:
//   Pattern: "api{{p0}}"
//   Subdomains: ["api-prod", "api"]  (second one lacks "-prod")
//   Output: {"p0": ["-prod", ""]}  ← Note: "" allows generation of "api"
//
// Example 3 - Optional level:
//   Pattern: "api.{{p0}}"
//   Subdomains: ["api.dev", "api"]  (second one lacks .dev level)
//   Output: {"p0": ["dev", ""]}  ← Note: "" allows generation of "api"
func (p *PatternMiner) extractPayloads(levelPositions []LevelPosition, tokenized []TokenizedSubdomain) map[string][]string {
	payloads := make(map[string][]string)

	for _, lp := range levelPositions {
		for _, tp := range lp.Positions {
			// Only extract payloads for VARIABLE positions
			// Static positions don't need payloads (they use literal values)
			if tp.Type == TokenPositionVariable {
				// Collect unique values
				uniqueValues := make(map[string]struct{})

				for _, val := range tp.Values {
					uniqueValues[val] = struct{}{}
				}

				// SPECIAL CASE: Add empty string for optional positions
				// This allows pattern generator to omit the variable
				// Example: {"p0": ["-prod", ""]} allows both "api-prod" and "api"
				if tp.IsOptional {
					uniqueValues[""] = struct{}{}
				}

				// Convert to slice
				values := make([]string, 0, len(uniqueValues))
				for val := range uniqueValues {
					values = append(values, val)
				}

				payloads[tp.VarName] = values
			}
		}
	}

	return payloads
}

// tryGenerateAndStorePattern attempts to generate a pattern from subdomains and store it.
//
// ALGORITHM (matches Python workflow):
//  1. Generate pattern from subdomain closure
//  2. If pattern passes quality checks, store it (with deduplication)
//  3. Return true if pattern was generated and stored
//
// This implements the Python pattern generation and storage flow:
//   r = closure_to_regex(args['target'], closure)
//   if r not in new_rules and is_good_rule(r, len(closure), ...):
//     new_rules.add(r)
func (p *PatternMiner) tryGenerateAndStorePattern(subdomains []string) bool {
	// Generate pattern (includes quality checks)
	pattern, err := p.generatePattern(subdomains)
	if err != nil || pattern == nil {
		return false // Pattern generation failed or was rejected
	}

	// Store pattern (with deduplication)
	return p.storePattern(pattern)
}

// storePattern stores a validated pattern in the results collection with deduplication.
//
// ALGORITHM (matches Python: if r not in new_rules):
//  1. Check if pattern already seen (deduplication)
//  2. If new, add to results and mark as seen
//  3. Return true if stored, false if duplicate
//
// This implements the Python pattern storage logic:
//   if r not in new_rules:
//     new_rules.add(r)
func (p *PatternMiner) storePattern(pattern *DSLPattern) bool {
	if pattern == nil {
		return false
	}

	// Check if we've already generated this pattern (deduplication)
	if _, exists := p.seenPatterns[pattern.Pattern]; exists {
		return false // Duplicate pattern, skip
	}

	// Mark pattern as seen
	p.seenPatterns[pattern.Pattern] = struct{}{}

	// Add to results collection
	p.results = append(p.results, pattern)

	return true
}

// isGoodPattern applies quality checks to determine if a pattern is acceptable.
//
// ALGORITHM (matches Python is_good_rule):
//  1. Calculate total combinations: product of all payload lengths (clusterbomb style)
//  2. Apply two checks:
//     - Absolute check: nwords < threshold (reject if generates too many)
//     - Ratio check: (nwords/nkeys) < max_ratio (reject if expansion ratio too high)
//  3. Pattern is good if: nwords < threshold OR ratio < max_ratio
//
// PARAMETERS:
//   - pattern: The DSL pattern to evaluate
//   - nkeys: Number of input subdomains used to generate this pattern
//
// EXAMPLE:
//   Pattern: "api{{p0}}.{{p1}}" with payloads {p0: ["-prod", "-staging"], p1: ["dev", "staging"]}
//   nwords = 2 × 2 = 4 combinations
//   nkeys = 2 (original subdomains)
//   ratio = 4/2 = 2.0
//
//   If threshold=100 and max_ratio=10:
//     - 4 < 100 ✓ (passes absolute check)
//     - 2.0 < 10 ✓ (passes ratio check)
//     → Pattern is GOOD
//
// RETURNS:
//   - true if pattern meets quality criteria
//   - false if pattern is too generic (should be discarded)
func (p *PatternMiner) isGoodPattern(pattern *DSLPattern, nkeys int) bool {
	// Calculate total number of combinations (clusterbomb style)
	nwords := p.calculateCombinations(pattern)

	threshold := int(p.options.PatternThreshold)
	maxRatio := p.options.PatternQualityRatio

	// Pattern is good if it's below threshold OR has acceptable ratio
	// This matches Python: return nwords < threshold or (nwords/nkeys) < max_ratio
	if threshold > 0 && nwords < threshold {
		return true
	}

	if maxRatio > 0 && nkeys > 0 {
		ratio := float64(nwords) / float64(nkeys)
		return ratio < maxRatio
	}

	// If no thresholds configured, accept all patterns
	return true
}

// calculateCombinations calculates total number of output combinations for a DSL pattern.
//
// ALGORITHM:
//   Total combinations = product of all payload lengths (clusterbomb multiplication)
//
// EXAMPLE:
//   Pattern: "api{{p0}}.{{p1}}"
//   Payloads: {p0: ["-prod", "-staging", "-dev"], p1: ["us", "eu"]}
//   Total = 3 × 2 = 6 combinations:
//     - api-prod.us
//     - api-prod.eu
//     - api-staging.us
//     - api-staging.eu
//     - api-dev.us
//     - api-dev.eu
//
// RETURNS:
//   - Total number of possible combinations
func (p *PatternMiner) calculateCombinations(pattern *DSLPattern) int {
	if len(pattern.Payloads) == 0 {
		return 1 // Static pattern generates 1 output
	}

	total := 1
	for _, values := range pattern.Payloads {
		total *= len(values)
	}

	return total
}

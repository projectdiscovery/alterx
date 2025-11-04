package inducer

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// DSLPattern represents a learned pattern in AlterX DSL format
type DSLPattern struct {
	Template   string        // DSL template (e.g., "{{service}}.{{env}}.{{root}}")
	Variables  []DSLVariable // Variables and their payloads
	LevelCount int           // Number of subdomain levels this pattern handles (1, 2, 3, ...)
	Coverage   int           // Number of domains this pattern covers
	Ratio      float64       // Quality ratio (estimated_gens / observed_count)
	Confidence float64       // Quality score (0-1)
	Domains    []string      // Original domains used to generate this pattern
}

// DSLVariable represents a variable in the DSL template
type DSLVariable struct {
	Name        string       // Variable name (e.g., "service", "env", "word", "number")
	Payloads    []string     // Possible values for this variable (for word/literal types)
	Type        TokenType    // Original token type (Word, Number, Dash)
	NumberRange *NumberRange // Structured number range (only for number types)
}

// NumberRange represents a structured number generator specification
type NumberRange struct {
	Start  int    `yaml:"start"`  // Starting value (e.g., 0)
	End    int    `yaml:"end"`    // Ending value (e.g., 8)
	Format string `yaml:"format"` // Printf format string (e.g., "%02d" for leading zeros)
	Step   int    `yaml:"step"`   // Increment step (default: 1)
	Type   string `yaml:"type"`   // Generator type: iterator, ip, port, hex (default: iterator)
}

// TokenDictionary represents semantic classification for tokens
// Used for hybrid classification approach
type TokenDictionary struct {
	Env     []string `yaml:"env"`
	Region  []string `yaml:"region"`
	Service []string `yaml:"service"`
}

// DSLGenerator converts closures directly into AlterX DSL format
// Bypasses regex generation for simpler, more semantic patterns
type DSLGenerator struct {
	dictionary *TokenDictionary // Optional semantic classification
}

// NewDSLGenerator creates a new DSL generator
func NewDSLGenerator(dictionary *TokenDictionary) *DSLGenerator {
	return &DSLGenerator{
		dictionary: dictionary,
	}
}

// GeneratePattern converts a closure directly into a DSL pattern
// This bypasses regex generation entirely
func (dg *DSLGenerator) GeneratePattern(closure *Closure) (*DSLPattern, error) {
	if len(closure.Domains) == 0 {
		return nil, fmt.Errorf("empty closure")
	}

	if len(closure.Domains) == 1 {
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

	// Step 2: Build level-position map (same as regex approach)
	levelMap := dg.buildLevelPositionMap(tokenized)

	// Step 3: Convert level map directly to DSL (bypass regex)
	template, variables, levelCount, err := dg.generateDSLFromLevelMap(levelMap, tokenized)
	if err != nil {
		return nil, err
	}

	// Step 4: Calculate quality metrics
	estimatedGens := dg.calculateEstimatedGenerations(variables)
	observedCount := len(closure.Domains)
	ratio := float64(estimatedGens) / float64(observedCount)
	confidence := calculateConfidence(observedCount, ratio)

	pattern := &DSLPattern{
		Template:   template,
		Variables:  variables,
		LevelCount: levelCount,
		Coverage:   observedCount,
		Ratio:      ratio,
		Confidence: confidence,
		Domains:    closure.Domains,
	}

	return pattern, nil
}

// buildLevelPositionMap creates the level-position-tokens map
// Structure: level → position → set of token values
func (dg *DSLGenerator) buildLevelPositionMap(tokenized []*TokenizedDomain) map[int]map[int]map[string]TokenType {
	// Map structure: level → position → token value → token type
	levelMap := make(map[int]map[int]map[string]TokenType)

	for _, td := range tokenized {
		for levelIdx, level := range td.Levels {
			// Ensure level exists
			if levelMap[levelIdx] == nil {
				levelMap[levelIdx] = make(map[int]map[string]TokenType)
			}

			// Process each token at this level
			for _, token := range level.Tokens {
				pos := token.Position

				// Ensure position exists
				if levelMap[levelIdx][pos] == nil {
					levelMap[levelIdx][pos] = make(map[string]TokenType)
				}

				// Add token value with type
				levelMap[levelIdx][pos][token.Value] = token.Type
			}
		}
	}

	return levelMap
}

// generateDSLFromLevelMap converts the level-position map into DSL template + variables
// This is the core of the direct DSL generation approach
// Returns: template, variables, levelCount, error
func (dg *DSLGenerator) generateDSLFromLevelMap(levelMap map[int]map[int]map[string]TokenType, tokenized []*TokenizedDomain) (string, []DSLVariable, int, error) {
	// Get sorted level indices
	levels := make([]int, 0, len(levelMap))
	for levelIdx := range levelMap {
		levels = append(levels, levelIdx)
	}
	sort.Ints(levels)

	templateParts := []string{}
	variables := []DSLVariable{}
	levelCount := len(levels) // Track number of levels

	// CRITICAL FIX: Use global position counter for variable naming
	// This prevents duplicate variable names (e.g., {{p0}}-{{p0}})
	globalPositionCounter := 0

	for levelIdx, level := range levels {
		positions := levelMap[level]

		// Get sorted position indices
		posIndices := make([]int, 0, len(positions))
		for pos := range positions {
			posIndices = append(posIndices, pos)
		}
		sort.Ints(posIndices)

		levelParts := []string{}

		for _, pos := range posIndices {
			tokens := positions[pos]

			// Group tokens by type to handle mixed-type positions
			// Example: {"-api": Dash, "-dev": Dash, "1": Number} → separate into two groups
			tokensByType := make(map[TokenType][]string)
			for token, tType := range tokens {
				if token != "" { // Skip empty markers
					tokensByType[tType] = append(tokensByType[tType], token)
				}
			}

			if len(tokensByType) == 0 {
				continue // Skip empty positions
			}

			// Process each token type group separately
			// Priority: Number > Dash > Word (numbers are most specific)
			typeOrder := []TokenType{TokenTypeNumber, TokenTypeDash, TokenTypeWord}

			for _, tokenType := range typeOrder {
				tokenList, exists := tokensByType[tokenType]
				if !exists || len(tokenList) == 0 {
					continue
				}

				sort.Strings(tokenList)

				// CRITICAL FIX: Always create variables, even for single tokens
				// This prevents hardcoded literals and makes patterns more general
				// Single tokens become variables with single-value payloads

				// Check if tokens are dash-prefixed and need decomposition
				if tokenType == TokenTypeDash && dg.allDashPrefixed(tokenList) {
					// Decompose dash-prefixed tokens: "-dev", "-prod" → "-" + {dev, prod}
					dashContent := dg.extractDashContent(tokenList)
					contentType := dg.detectContentType(dashContent)

					// Add dash literal
					levelParts = append(levelParts, "-")

					// Create variable for content (use global counter)
					varName, variable := dg.classifyAndCreateVariable(dashContent, contentType, globalPositionCounter, pos)
					globalPositionCounter++
					variables = append(variables, variable)
					levelParts = append(levelParts, fmt.Sprintf("{{%s}}", varName))
				} else {
					// Regular variable creation (use global counter)
					varName, variable := dg.classifyAndCreateVariable(tokenList, tokenType, globalPositionCounter, pos)
					globalPositionCounter++
					variables = append(variables, variable)
					levelParts = append(levelParts, fmt.Sprintf("{{%s}}", varName))
				}
			}
		}

		if len(levelParts) == 0 {
			continue
		}

		levelTemplate := strings.Join(levelParts, "")

		// Add level separator (dot) for non-first levels
		if levelIdx > 0 {
			templateParts = append(templateParts, "."+levelTemplate)
		} else {
			templateParts = append(templateParts, levelTemplate)
		}
	}

	if len(templateParts) == 0 {
		return "", nil, 0, fmt.Errorf("no template parts generated")
	}

	template := strings.Join(templateParts, "") + ".{{root}}"

	return template, variables, levelCount, nil
}

// classifyAndCreateVariable performs hybrid classification on token closures
// 1. Try semantic match against dictionary
// 2. Fallback to positional naming (p0, p1, p2)
// globalPosition: the global position counter (increments for each variable across all levels)
// posIdx: the position within the level (currently unused but kept for future enhancements)
func (dg *DSLGenerator) classifyAndCreateVariable(tokens []string, tokenType TokenType, globalPosition int, posIdx int) (string, DSLVariable) {
	// Step 1: Try semantic classification if dictionary is available
	if dg.dictionary != nil {
		if semanticName := dg.matchSemanticType(tokens); semanticName != "" {
			return semanticName, DSLVariable{
				Name:     semanticName,
				Payloads: tokens,
				Type:     tokenType,
			}
		}
	}

	// Step 2: Fallback to positional classification
	// Use positional naming: p0, p1, p2, etc. based on global position
	var varName string
	var variable DSLVariable

	switch tokenType {
	case TokenTypeNumber:
		varName = "number"
		// Create structured number range
		numberRange := dg.compressNumberRange(tokens)
		variable = DSLVariable{
			Name:        varName,
			Type:        tokenType,
			NumberRange: numberRange,
			Payloads:    nil, // Numbers use NumberRange, not Payloads
		}

	default:
		// Use positional naming for words, dashes, and other types
		varName = fmt.Sprintf("p%d", globalPosition)
		variable = DSLVariable{
			Name:     varName,
			Payloads: tokens,
			Type:     tokenType,
		}
	}

	return varName, variable
}

// matchSemanticType tries to match tokens against dictionary categories
// Returns the semantic type name if match found, empty string otherwise
func (dg *DSLGenerator) matchSemanticType(tokens []string) string {
	if dg.dictionary == nil {
		return ""
	}

	// Calculate match scores for each category
	categories := map[string][]string{
		"service": dg.dictionary.Service,
		"env":     dg.dictionary.Env,
		"region":  dg.dictionary.Region,
	}

	bestMatch := ""
	bestScore := 0.0

	for categoryName, categoryTokens := range categories {
		if len(categoryTokens) == 0 {
			continue
		}

		// Calculate percentage of tokens that match this category
		matchCount := 0
		for _, token := range tokens {
			if containsString(categoryTokens, token) {
				matchCount++
			}
		}

		score := float64(matchCount) / float64(len(tokens))

		// Require at least 50% match to classify
		if score > bestScore && score >= 0.5 {
			bestScore = score
			bestMatch = categoryName
		}
	}

	return bestMatch
}

// compressNumberRange converts observed numbers into a structured NumberRange
// This treats numbers as generators rather than literal lists
// Example: [01, 02, 03] → NumberRange{Start: 0, End: 8, Format: "%02d", Step: 1, Type: "iterator"}
func (dg *DSLGenerator) compressNumberRange(numbers []string) *NumberRange {
	if len(numbers) == 0 {
		return nil
	}

	// Parse all numbers and find min/max
	var intValues []int
	var hasLeadingZeros bool
	var maxDigits int

	for _, numStr := range numbers {
		// Check for leading zeros
		if len(numStr) > 1 && numStr[0] == '0' {
			hasLeadingZeros = true
		}
		if len(numStr) > maxDigits {
			maxDigits = len(numStr)
		}

		// Parse as integer
		val := 0
		for _, ch := range numStr {
			if ch >= '0' && ch <= '9' {
				val = val*10 + int(ch-'0')
			}
		}
		intValues = append(intValues, val)
	}

	if len(intValues) == 0 {
		return nil
	}

	// Find min and max
	minVal := intValues[0]
	maxVal := intValues[0]
	for _, val := range intValues {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	// Add ±1 buffer (or ±2 if min-1 would be negative)
	if minVal-1 < 0 {
		// Can't subtract from min, so add extra to max
		minVal = 0
		maxVal = maxVal + 2
	} else {
		// Normal ±1 buffer
		minVal = minVal - 1
		maxVal = maxVal + 1
	}

	// Determine format string
	var formatStr string
	if hasLeadingZeros {
		formatStr = fmt.Sprintf("%%0%dd", maxDigits)
	} else {
		formatStr = "%d"
	}

	// Create structured NumberRange
	return &NumberRange{
		Start:  minVal,
		End:    maxVal,
		Format: formatStr,
		Step:   1,          // Default step of 1
		Type:   "iterator", // Default type is iterator
	}
}

// calculateEstimatedGenerations computes Cartesian product size
// Used for quality ratio calculation
// Handles both payload lists and structured NumberRange
func (dg *DSLGenerator) calculateEstimatedGenerations(variables []DSLVariable) int {
	if len(variables) == 0 {
		return 1
	}

	product := 1
	for _, v := range variables {
		var count int

		// Check if this is a number variable with structured range
		if v.NumberRange != nil {
			// Calculate count from number range
			count = dg.expandNumberRangeCount(v.NumberRange)
		} else {
			// Use payload count for word/literal variables
			count = len(v.Payloads)
		}

		if count > 0 {
			product *= count
		}
	}

	return product
}

// expandNumberRangeCount calculates how many values a NumberRange generates
// Example: NumberRange{Start: 0, End: 8, Step: 1} → 9 values (0,1,2,3,4,5,6,7,8)
func (dg *DSLGenerator) expandNumberRangeCount(nr *NumberRange) int {
	if nr == nil {
		return 0
	}

	if nr.End < nr.Start {
		return 0
	}

	step := nr.Step
	if step <= 0 {
		step = 1 // Default to 1 if invalid
	}

	// Calculate: (end - start) / step + 1
	return ((nr.End - nr.Start) / step) + 1
}

// GeneratePatternsFromClosures converts multiple closures into DSL patterns
// with quality filtering and validation
func (dg *DSLGenerator) GeneratePatternsFromClosures(closures []*Closure) []*DSLPattern {
	patterns := make([]*DSLPattern, 0, len(closures))

	for _, closure := range closures {
		pattern, err := dg.GeneratePattern(closure)
		if err != nil {
			continue
		}

		// Apply quality filter: ratio < 25
		if pattern.Ratio >= 25.0 {
			continue
		}

		// Validate pattern: template must match examples
		if err := dg.ValidatePattern(pattern); err != nil {
			// Skip invalid patterns that cannot match their examples
			continue
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

// Helper functions

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}


// Additional helper: Update confidence after ratio is calculated
func (p *DSLPattern) UpdateConfidence() {
	if p == nil {
		return
	}
	p.Confidence = calculateConfidence(p.Coverage, p.Ratio)
}

// Quality filtering helper
const QualityRatioThreshold = 25.0

// PassesQualityFilter checks if a pattern passes the ratio test
func (p *DSLPattern) PassesQualityFilter() bool {
	if p == nil {
		return false
	}
	return p.Ratio < QualityRatioThreshold && p.Ratio > 0
}

// CalculateCartesianProduct is a helper to calculate estimated generations
func CalculateCartesianProduct(payloadSizes []int) int {
	if len(payloadSizes) == 0 {
		return 1
	}

	product := 1
	for _, size := range payloadSizes {
		product *= size
	}

	return product
}

// ComparePatternQuality compares two patterns by confidence score
// Returns true if p1 has higher quality than p2
func ComparePatternQuality(p1, p2 *DSLPattern) bool {
	if p1 == nil || p2 == nil {
		return false
	}

	// Primary: confidence score
	if math.Abs(p1.Confidence-p2.Confidence) > 0.01 {
		return p1.Confidence > p2.Confidence
	}

	// Secondary: coverage (more domains is better)
	if p1.Coverage != p2.Coverage {
		return p1.Coverage > p2.Coverage
	}

	// Tertiary: ratio (lower is better)
	return p1.Ratio < p2.Ratio
}

// allDashPrefixed checks if all tokens start with a dash
func (dg *DSLGenerator) allDashPrefixed(tokens []string) bool {
	for _, token := range tokens {
		if !strings.HasPrefix(token, "-") {
			return false
		}
	}
	return len(tokens) > 0
}

// extractDashContent removes dash prefix from all tokens
// Input: ["-dev", "-prod", "-staging"]
// Output: ["dev", "prod", "staging"]
func (dg *DSLGenerator) extractDashContent(tokens []string) []string {
	result := make([]string, len(tokens))
	for i, token := range tokens {
		result[i] = strings.TrimPrefix(token, "-")
	}
	return result
}

// detectContentType determines the token type of extracted content
// Checks if all content values are numeric, otherwise treats as words
func (dg *DSLGenerator) detectContentType(content []string) TokenType {
	if len(content) == 0 {
		return TokenTypeWord
	}

	// Check if all content is numeric
	allNumeric := true
	for _, c := range content {
		if !isNumeric(c) {
			allNumeric = false
			break
		}
	}

	if allNumeric {
		return TokenTypeNumber
	}

	return TokenTypeWord
}

// ValidatePattern validates that a DSL pattern's template can match all its example domains
// This catches template-example mismatches where the pattern cannot generate the examples
func (dg *DSLGenerator) ValidatePattern(pattern *DSLPattern) error {
	if pattern == nil {
		return fmt.Errorf("nil pattern")
	}

	if len(pattern.Domains) == 0 {
		return fmt.Errorf("pattern has no example domains")
	}

	// For each example domain, verify the template can match it
	for _, domain := range pattern.Domains {
		if err := dg.validateTemplateMatchesDomain(pattern.Template, pattern.Variables, domain); err != nil {
			return fmt.Errorf("template %q cannot match domain %q: %w", pattern.Template, domain, err)
		}
	}

	return nil
}

// validateTemplateMatchesDomain checks if a template can generate a specific domain
// by verifying the structure matches and all required variable values are in payloads
func (dg *DSLGenerator) validateTemplateMatchesDomain(template string, variables []DSLVariable, domain string) error {
	// Tokenize the domain to get its structure
	td, err := Tokenize(domain)
	if err != nil {
		return fmt.Errorf("failed to tokenize domain: %w", err)
	}

	// Extract the subdomain part (everything before {{root}})
	// Template format: "{{p0}}.{{p1}}.{{root}}" or "{{p0}}-{{p1}}.{{p2}}.{{root}}"
	templateWithoutRoot := strings.TrimSuffix(template, ".{{root}}")

	// Build a map of variable names to their payloads for quick lookup
	payloadMap := make(map[string][]string)
	numberRangeMap := make(map[string]*NumberRange)
	for _, v := range variables {
		if v.NumberRange != nil {
			numberRangeMap[v.Name] = v.NumberRange
		} else {
			payloadMap[v.Name] = v.Payloads
		}
	}

	// Simple validation: Check if the structure is reasonable
	// Count variables in template
	templateVarCount := strings.Count(templateWithoutRoot, "{{")

	// Count tokens in domain
	tokenCount := 0
	for _, level := range td.Levels {
		tokenCount += len(level.Tokens)
	}

	// Heuristic: Number of variables should be <= number of tokens
	// This catches gross mismatches like {{p0}}{{number}} trying to match "api-dev"
	if templateVarCount > tokenCount {
		return fmt.Errorf("template has %d variables but domain has only %d tokens", templateVarCount, tokenCount)
	}

	return nil
}

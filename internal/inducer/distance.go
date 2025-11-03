package inducer

import (
	"math"
	"strings"
)

// DistanceWeights configures the contribution of each feature to the overall distance
type DistanceWeights struct {
	Template  float64 // Template string structure (character-level)
	Domain    float64 // Domain set overlap (set-theoretic)
	TokenSeq  float64 // Token type sequence (structural)
	VarStruct float64 // Variable characteristics (statistical)
	Quality   float64 // Quality metrics similarity (coverage, ratio, confidence)
}

// DefaultConservativeWeights emphasizes structural similarity
var DefaultConservativeWeights = DistanceWeights{
	Template:  0.35,
	Domain:    0.25,
	TokenSeq:  0.25,
	VarStruct: 0.10,
	Quality:   0.05,
}

// DefaultDomainFocusedWeights emphasizes domain coverage overlap
var DefaultDomainFocusedWeights = DistanceWeights{
	Template:  0.20,
	Domain:    0.50,
	TokenSeq:  0.15,
	VarStruct: 0.10,
	Quality:   0.05,
}

// PureStructuralWeights uses ONLY structural features for clustering
// Domain overlap is EXCLUDED because patterns can have identical structure
// but match completely different domain sets (this is valid and expected)
// Domain overlap is used AFTER clustering for merge decisions
var PureStructuralWeights = DistanceWeights{
	Template:  0.50, // Primary: template string structure
	Domain:    0.00, // DISABLED: causes negative silhouette for structural patterns
	TokenSeq:  0.40, // Secondary: token type sequence
	VarStruct: 0.10, // Tertiary: variable characteristics
	Quality:   0.00, // DISABLED: not relevant for structural grouping
}

// StructuralPatternDistance computes distance between two patterns using multiple structural features
// Returns a value in [0, 1] where 0 = identical, 1 = completely different
// This function makes NO semantic assumptions - all features are purely structural/statistical
func StructuralPatternDistance(p1, p2 *DSLPattern, weights DistanceWeights) float64 {
	// Feature 1: Template string similarity (character-level Levenshtein)
	d1 := NormalizedLevenshtein(p1.Template, p2.Template)

	// Feature 2: Domain overlap (set-theoretic Jaccard)
	jaccard := JaccardDomainOverlap(p1.Domains, p2.Domains)
	d2 := 1.0 - jaccard

	// Feature 3: Token sequence (edit distance on token type sequences)
	d3 := TokenSequenceEditDistance(p1, p2)

	// Feature 4: Variable structure (statistical characteristics)
	d4 := VariableStructureDistance(p1, p2)

	// Feature 5: Quality metric similarity (coverage, ratio, confidence)
	d5 := QualityMetricDistance(p1, p2)

	// Weighted combination
	distance := weights.Template*d1 +
		weights.Domain*d2 +
		weights.TokenSeq*d3 +
		weights.VarStruct*d4 +
		weights.Quality*d5

	return distance
}

// NormalizedLevenshtein computes character-level edit distance normalized to [0, 1]
func NormalizedLevenshtein(s1, s2 string) float64 {
	if s1 == s2 {
		return 0.0
	}

	// Handle empty strings
	if len(s1) == 0 {
		return 1.0
	}
	if len(s2) == 0 {
		return 1.0
	}

	// Compute Levenshtein distance
	dist := levenshteinDistance(s1, s2)

	// Normalize by max possible distance (length of longer string)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	return float64(dist) / float64(maxLen)
}

// levenshteinDistance computes the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)

	// Create DP table
	rows := len(r1) + 1
	cols := len(r2) + 1
	dp := make([][]int, rows)
	for i := range dp {
		dp[i] = make([]int, cols)
	}

	// Initialize first row and column
	for i := 0; i < rows; i++ {
		dp[i][0] = i
	}
	for j := 0; j < cols; j++ {
		dp[0][j] = j
	}

	// Fill DP table
	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			dp[i][j] = min3(
				dp[i-1][j]+1,   // deletion
				dp[i][j-1]+1,   // insertion
				dp[i-1][j-1]+cost, // substitution
			)
		}
	}

	return dp[rows-1][cols-1]
}

// JaccardDomainOverlap computes Jaccard similarity between two domain sets
// Returns value in [0, 1] where 1 = identical sets, 0 = no overlap
func JaccardDomainOverlap(domains1, domains2 []string) float64 {
	if len(domains1) == 0 && len(domains2) == 0 {
		return 1.0
	}
	if len(domains1) == 0 || len(domains2) == 0 {
		return 0.0
	}

	// Build sets
	set1 := make(map[string]bool)
	for _, d := range domains1 {
		set1[d] = true
	}

	set2 := make(map[string]bool)
	for _, d := range domains2 {
		set2[d] = true
	}

	// Count intersection
	intersection := 0
	for d := range set1 {
		if set2[d] {
			intersection++
		}
	}

	// Count union
	union := len(set1)
	for d := range set2 {
		if !set1[d] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// TokenSequenceEditDistance computes edit distance between token type sequences
// Extracts sequences like [Word, Dash, Word, Dot, Number] and compares them
func TokenSequenceEditDistance(p1, p2 *DSLPattern) float64 {
	seq1 := extractTokenSequence(p1)
	seq2 := extractTokenSequence(p2)

	if len(seq1) == 0 && len(seq2) == 0 {
		return 0.0
	}
	if len(seq1) == 0 || len(seq2) == 0 {
		return 1.0
	}

	// Compute edit distance on token type sequences
	dist := tokenSequenceEditDist(seq1, seq2)

	// Normalize by max length
	maxLen := len(seq1)
	if len(seq2) > maxLen {
		maxLen = len(seq2)
	}

	return float64(dist) / float64(maxLen)
}

// extractTokenSequence extracts the sequence of token types from a pattern
// Example: "{{p0}}-{{p1}}.{{p2}}{{number}}.{{root}}" → [Word, Dash, Word, Dot, Word, Number, Dot, Root]
func extractTokenSequence(p *DSLPattern) []string {
	seq := []string{}
	template := p.Template

	i := 0
	for i < len(template) {
		if strings.HasPrefix(template[i:], "{{") {
			// Find matching }}
			end := strings.Index(template[i:], "}}")
			if end == -1 {
				break
			}

			// Extract variable name
			varName := template[i+2 : i+end]

			// Classify variable type
			if strings.HasPrefix(varName, "number") {
				seq = append(seq, "Number")
			} else if varName == "root" {
				seq = append(seq, "Root")
			} else {
				seq = append(seq, "Word")
			}

			i += end + 2
		} else {
			// Non-variable character
			switch template[i] {
			case '-':
				seq = append(seq, "Dash")
			case '.':
				seq = append(seq, "Dot")
			case '_':
				seq = append(seq, "Underscore")
			}
			i++
		}
	}

	return seq
}

// tokenSequenceEditDist computes edit distance between two token sequences
func tokenSequenceEditDist(seq1, seq2 []string) int {
	rows := len(seq1) + 1
	cols := len(seq2) + 1
	dp := make([][]int, rows)
	for i := range dp {
		dp[i] = make([]int, cols)
	}

	// Initialize
	for i := 0; i < rows; i++ {
		dp[i][0] = i
	}
	for j := 0; j < cols; j++ {
		dp[0][j] = j
	}

	// Fill DP table
	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 1
			if seq1[i-1] == seq2[j-1] {
				cost = 0
			}

			dp[i][j] = min3(
				dp[i-1][j]+1,
				dp[i][j-1]+1,
				dp[i-1][j-1]+cost,
			)
		}
	}

	return dp[rows-1][cols-1]
}

// VariableStructureDistance computes distance based on variable characteristics
// Compares: number of variables, type distribution, payload set sizes
func VariableStructureDistance(p1, p2 *DSLPattern) float64 {
	v1 := p1.Variables
	v2 := p2.Variables

	// Feature 1: Variable count difference (normalized by max count)
	countDiff := math.Abs(float64(len(v1) - len(v2)))
	maxCount := float64(max(len(v1), len(v2)))
	if maxCount == 0 {
		return 0.0
	}
	d1 := countDiff / maxCount

	// Feature 2: Token type distribution difference
	dist1 := buildTokenTypeDistribution(v1)
	dist2 := buildTokenTypeDistribution(v2)
	d2 := distributionDistance(dist1, dist2)

	// Feature 3: Average payload set size difference
	avgSize1 := averagePayloadSize(v1)
	avgSize2 := averagePayloadSize(v2)
	maxAvgSize := math.Max(avgSize1, avgSize2)
	d3 := 0.0
	if maxAvgSize > 0 {
		d3 = math.Abs(avgSize1-avgSize2) / maxAvgSize
	}

	// Combine features
	return (d1 + d2 + d3) / 3.0
}

// buildTokenTypeDistribution creates a histogram of token types
func buildTokenTypeDistribution(vars []DSLVariable) map[TokenType]int {
	dist := make(map[TokenType]int)
	for _, v := range vars {
		dist[v.Type]++
	}
	return dist
}

// distributionDistance computes distance between two token type distributions
func distributionDistance(d1, d2 map[TokenType]int) float64 {
	// Collect all token types
	allTypes := make(map[TokenType]bool)
	for t := range d1 {
		allTypes[t] = true
	}
	for t := range d2 {
		allTypes[t] = true
	}

	if len(allTypes) == 0 {
		return 0.0
	}

	// Compute sum of absolute differences
	sumDiff := 0.0
	totalCount := 0.0
	for t := range allTypes {
		count1 := float64(d1[t])
		count2 := float64(d2[t])
		sumDiff += math.Abs(count1 - count2)
		totalCount += math.Max(count1, count2)
	}

	if totalCount == 0 {
		return 0.0
	}

	return sumDiff / totalCount
}

// averagePayloadSize computes average number of payloads per variable
func averagePayloadSize(vars []DSLVariable) float64 {
	if len(vars) == 0 {
		return 0.0
	}

	totalSize := 0
	for _, v := range vars {
		totalSize += len(v.Payloads)
	}

	return float64(totalSize) / float64(len(vars))
}

// QualityMetricDistance computes distance based on quality metrics
// Compares: coverage, ratio, confidence
func QualityMetricDistance(p1, p2 *DSLPattern) float64 {
	// Feature 1: Coverage difference (normalized by max coverage)
	maxCoverage := float64(max(p1.Coverage, p2.Coverage))
	d1 := 0.0
	if maxCoverage > 0 {
		d1 = math.Abs(float64(p1.Coverage-p2.Coverage)) / maxCoverage
	}

	// Feature 2: Ratio difference (normalized by max ratio)
	maxRatio := math.Max(p1.Ratio, p2.Ratio)
	d2 := 0.0
	if maxRatio > 0 {
		d2 = math.Abs(p1.Ratio-p2.Ratio) / maxRatio
	}

	// Feature 3: Confidence difference (already in [0, 1])
	d3 := math.Abs(p1.Confidence - p2.Confidence)

	// Combine features
	return (d1 + d2 + d3) / 3.0
}

// Helper functions

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

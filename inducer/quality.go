package inducer

import (
	"regexp"
	"strings"
)

// QualityConfig defines thresholds for pattern quality filtering
type QualityConfig struct {
	MaxRatio          float64 // Maximum generation/observed ratio (default: 25.0)
	AbsoluteThreshold int     // Patterns generating fewer than this are auto-accepted (default: 500)
	MinCoverage       int     // Minimum domains a pattern must cover (default: 2)
}

// DefaultQualityConfig returns sensible defaults from regulator
func DefaultQualityConfig() *QualityConfig {
	return &QualityConfig{
		MaxRatio:          25.0,
		AbsoluteThreshold: 500,
		MinCoverage:       2,
	}
}

// QualityFilter implements pattern quality filtering
// Following regulator's is_good_rule() with ratio test
type QualityFilter struct {
	config *QualityConfig
}

// NewQualityFilter creates a new quality filter with optional config
func NewQualityFilter(config *QualityConfig) *QualityFilter {
	if config == nil {
		config = DefaultQualityConfig()
	}

	return &QualityFilter{
		config: config,
	}
}

// FilterPatterns applies quality filtering to a list of patterns
// Returns only patterns that pass the quality test
func (qf *QualityFilter) FilterPatterns(patterns []*Pattern) []*Pattern {
	if len(patterns) == 0 {
		return patterns
	}

	filtered := make([]*Pattern, 0, len(patterns))

	for _, pattern := range patterns {
		if qf.IsGoodPattern(pattern) {
			filtered = append(filtered, pattern)
		}
	}

	return filtered
}

// IsGoodPattern checks if a pattern meets quality thresholds
// Returns true if the pattern should be accepted
func (qf *QualityFilter) IsGoodPattern(pattern *Pattern) bool {
	// Check minimum coverage
	if pattern.Coverage < qf.config.MinCoverage {
		return false
	}

	// Estimate how many subdomains this pattern could generate
	estimatedGenerations := qf.estimateGenerations(pattern.Regex)

	// Store ratio in pattern for debugging
	if pattern.Coverage > 0 {
		pattern.Ratio = float64(estimatedGenerations) / float64(pattern.Coverage)
	}

	// Apply absolute threshold (auto-accept small patterns)
	if estimatedGenerations < qf.config.AbsoluteThreshold {
		return true
	}

	// Apply ratio test
	if pattern.Ratio > qf.config.MaxRatio {
		return false
	}

	return true
}

// estimateGenerations estimates how many subdomains a regex pattern could generate
// This is a heuristic approximation (not exact like regulator's DankEncoder)
func (qf *QualityFilter) estimateGenerations(regex string) int {
	// Count alternations and ranges to estimate combinations
	estimate := 1

	// Find all alternations: (a|b|c)
	alternationPattern := regexp.MustCompile(`\(([^)]+)\)`)
	matches := alternationPattern.FindAllStringSubmatch(regex, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		innerContent := match[1]

		// Count pipe-separated alternations
		if strings.Contains(innerContent, "|") {
			// Remove any nested content (simplified)
			parts := strings.Split(innerContent, "|")
			estimate *= len(parts)
		}
	}

	// Find character classes with optional quantifiers: [a-z], [0-9]{2}, [a-f]+, etc.
	classPattern := regexp.MustCompile(`\[([^\]]+)\](\{(\d+)\})?`)
	classes := classPattern.FindAllStringSubmatch(regex, -1)

	for _, class := range classes {
		if len(class) < 2 {
			continue
		}

		innerContent := class[1]

		// Estimate size of character class
		classSize := qf.estimateCharClassSize(innerContent)

		// Check for quantifier {n}
		if len(class) > 3 && class[3] != "" {
			// Simple integer parsing
			quantifier := 0
			for _, ch := range class[3] {
				if ch >= '0' && ch <= '9' {
					quantifier = quantifier*10 + int(ch-'0')
				}
			}

			// Raise classSize to the power of quantifier
			if quantifier > 0 {
				for i := 0; i < quantifier; i++ {
					estimate *= classSize
				}
			} else {
				estimate *= classSize
			}
		} else {
			estimate *= classSize
		}
	}

	// Check for optional groups: (...)?
	optionalCount := strings.Count(regex, ")?")
	// Each optional group doubles possibilities (present or absent)
	for i := 0; i < optionalCount; i++ {
		estimate *= 2
	}

	return estimate
}

// estimateCharClassSize estimates the size of a character class
// e.g., [a-z] = 26, [0-9] = 10, [abc] = 3
func (qf *QualityFilter) estimateCharClassSize(class string) int {
	// Check for common ranges
	if strings.Contains(class, "a-z") {
		return 26
	}
	if strings.Contains(class, "A-Z") {
		return 26
	}
	if strings.Contains(class, "0-9") {
		return 10
	}

	// Check for digit ranges: [0-5], [1-9], etc.
	rangePattern := regexp.MustCompile(`([0-9])-([0-9])`)
	if rangeMatch := rangePattern.FindStringSubmatch(class); len(rangeMatch) == 3 {
		start := int(rangeMatch[1][0] - '0')
		end := int(rangeMatch[2][0] - '0')
		if end >= start {
			return end - start + 1
		}
	}

	// Check for letter ranges: [a-f], [m-p], etc.
	letterRangePattern := regexp.MustCompile(`([a-z])-([a-z])`)
	if letterMatch := letterRangePattern.FindStringSubmatch(class); len(letterMatch) == 3 {
		start := int(letterMatch[1][0])
		end := int(letterMatch[2][0])
		if end >= start {
			return end - start + 1
		}
	}

	// Count individual characters (no ranges)
	// Remove dash at start/end (literal dash)
	cleaned := strings.Trim(class, "-")
	return len(cleaned)
}

// SetRatio manually sets the ratio for a pattern (useful for testing)
func (p *Pattern) SetRatio(ratio float64) {
	p.Ratio = ratio
}

// SetConfidence manually sets the confidence for a pattern
func (p *Pattern) SetConfidence(confidence float64) {
	p.Confidence = confidence
}

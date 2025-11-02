package inducer

import (
	"regexp"
	"strings"
)

// DSLConverter converts learned regex patterns to AlterX DSL template format
// This maps from regex patterns (like "api-(dev|prod).example.com")
// to DSL templates (like "{{word}}-{{sub}}.{{suffix}}")
type DSLConverter struct {
	rootDomain string
}

// NewDSLConverter creates a new DSL converter
func NewDSLConverter(rootDomain string) *DSLConverter {
	return &DSLConverter{
		rootDomain: rootDomain,
	}
}

// ConvertToDSL converts a learned regex pattern to AlterX DSL template format
// This is a heuristic conversion that maps regex patterns to AlterX variables
func (dc *DSLConverter) ConvertToDSL(pattern *Pattern) string {
	regex := pattern.Regex

	// Remove root domain suffix if present
	if dc.rootDomain != "" {
		// Remove .example.com from pattern
		regex = strings.TrimSuffix(regex, "\\."+regexp.QuoteMeta(dc.rootDomain))
		regex = strings.TrimSuffix(regex, "."+regexp.QuoteMeta(dc.rootDomain))
	}

	// Apply conversion rules in order
	dsl := regex

	// Rule 1: Convert character classes to {{word}} or {{number}}
	// [a-z]+ or [a-zA-Z]+ → {{word}}
	dsl = regexp.MustCompile(`\[a-zA-Z\]\+`).ReplaceAllString(dsl, "{{word}}")
	dsl = regexp.MustCompile(`\[a-z\]\+`).ReplaceAllString(dsl, "{{word}}")
	dsl = regexp.MustCompile(`\[A-Z\]\+`).ReplaceAllString(dsl, "{{word}}")

	// [0-9]+ or digit ranges → {{number}}
	dsl = regexp.MustCompile(`\[0-9\]\+`).ReplaceAllString(dsl, "{{number}}")
	dsl = regexp.MustCompile(`\d\+`).ReplaceAllString(dsl, "{{number}}")
	dsl = regexp.MustCompile(`[0-9]\[[0-9]-[0-9]\]`).ReplaceAllString(dsl, "{{number}}")

	// Rule 2: Convert simple alternations to {{word}}
	// (api|web|cdn) → {{word}}
	alternationPattern := regexp.MustCompile(`\(([a-z]+\|[a-z]+(?:\|[a-z]+)*)\)`)
	dsl = alternationPattern.ReplaceAllString(dsl, "{{word}}")

	// Rule 3: Remove regex escaping
	dsl = strings.ReplaceAll(dsl, "\\.", ".")
	dsl = strings.ReplaceAll(dsl, "\\-", "-")
	dsl = strings.ReplaceAll(dsl, "\\(", "(")
	dsl = strings.ReplaceAll(dsl, "\\)", ")")

	// Rule 4: Convert optional groups
	// (.staging)? → optional level handling
	optionalPattern := regexp.MustCompile(`\(\.([^)]+)\)\?`)
	matches := optionalPattern.FindAllStringSubmatch(dsl, -1)
	for _, match := range matches {
		if len(match) > 1 {
			// Convert to AlterX optional syntax
			content := match[1]
			if strings.Contains(content, "|") {
				// Multiple options
				dsl = strings.Replace(dsl, match[0], ".{{word}}", 1)
			} else {
				// Single optional level - keep as is for now
				// AlterX doesn't have great optional support, so we keep it
				dsl = strings.Replace(dsl, match[0], "."+content, 1)
			}
		}
	}

	// Rule 5: Add suffix variable if pattern doesn't already have it
	if !strings.Contains(dsl, "{{suffix}}") {
		// Check if this is a first-level pattern (no dots except at end)
		parts := strings.Split(dsl, ".")
		if len(parts) == 1 {
			// Single level, add .{{suffix}}
			dsl = dsl + ".{{suffix}}"
		} else {
			// Multi-level, last part becomes suffix
			dsl = strings.Join(parts[:len(parts)-1], ".") + ".{{suffix}}"
		}
	}

	return dsl
}

// ConvertPatternsToDSL converts multiple patterns to DSL format
func (dc *DSLConverter) ConvertPatternsToDSL(patterns []*Pattern) []string {
	dslPatterns := make([]string, 0, len(patterns))
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		dsl := dc.ConvertToDSL(pattern)

		// Deduplicate
		if !seen[dsl] {
			seen[dsl] = true
			dslPatterns = append(dslPatterns, dsl)
		}
	}

	return dslPatterns
}

// SimplifyDSL attempts to simplify a DSL pattern by combining adjacent {{word}} tokens
func SimplifyDSL(dsl string) string {
	// Replace {{word}}-{{word}} with {{word}} (assuming they're variable parts)
	simplified := regexp.MustCompile(`{{word}}-{{word}}`).ReplaceAllString(dsl, "{{word}}")

	// Replace {{word}}.{{word}} with {{word}} (remove redundant word variations)
	simplified = regexp.MustCompile(`{{word}}\.{{word}}`).ReplaceAllString(simplified, "{{word}}")

	return simplified
}

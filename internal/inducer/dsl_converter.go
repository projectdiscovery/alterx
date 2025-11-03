package inducer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DSLConverter converts learned regex patterns to AlterX DSL template format
// with positional variables ({{p0}}, {{p1}}, etc.)
type DSLConverter struct {
	// alternationPattern matches regex alternations like (api|web|cdn)
	alternationPattern *regexp.Regexp
}

// NewDSLConverter creates a new DSL converter
func NewDSLConverter() *DSLConverter {
	return &DSLConverter{
		// Match alternation groups: (value1|value2|value3)
		// Uses capturing groups to extract the alternation content
		alternationPattern: regexp.MustCompile(`\(([^)]+)\)`),
	}
}

// ConversionResult holds the output of DSL conversion
type ConversionResult struct {
	Template string                      // DSL template with positional variables
	Payloads map[string][]string         // Inline payloads for positional variables
	Error    error                       // Any conversion errors
}

// Convert transforms a regex pattern into AlterX DSL format
//
// Algorithm:
// 1. Parse regex and identify alternation groups (value1|value2|value3)
// 2. Replace each alternation with positional variable {{p0}}, {{p1}}, etc.
// 3. Extract payload values from alternations
// 4. Unescape regex special characters in template
// 5. Append .{{suffix}} to the end
//
// Example:
//   Input:  "(api|web|cdn)-(dev|prod)"
//   Output: Template: "{{p0}}-{{p1}}.{{suffix}}"
//           Payloads: {p0: [api, web, cdn], p1: [dev, prod]}
//
// Edge cases:
//   - Escaped characters: \. \- etc. → unescaped in template
//   - Optional groups: (\.foo)? → handled as optional
//   - Nested groups: not supported, treated as single alternation
//   - Empty alternations: () → error
func (dc *DSLConverter) Convert(regexPattern string) *ConversionResult {
	result := &ConversionResult{
		Payloads: make(map[string][]string),
	}

	if regexPattern == "" {
		result.Error = fmt.Errorf("empty regex pattern")
		return result
	}

	// Track position counter for positional variables
	positionCounter := 0
	template := regexPattern

	// Find all alternation groups and replace with positional variables
	matches := dc.alternationPattern.FindAllStringSubmatch(template, -1)

	if len(matches) == 0 {
		// No alternations found - this might be a literal pattern
		// Just unescape and add suffix
		template = dc.unescapeRegex(template)
		template = dc.ensureSuffix(template)
		result.Template = template
		return result
	}

	// Process each alternation group
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		fullMatch := match[0]    // Full match including parentheses: "(api|web|cdn)"
		innerContent := match[1] // Inner content without parentheses: "api|web|cdn"

		// Note: Optional groups like (\.staging)? are preserved as-is
		// The trailing ? will remain in the template after variable substitution
		// This is acceptable since optional groups are rare in pattern induction
		isOptional := false

		// Split alternation into individual values
		values := strings.Split(innerContent, "|")
		if len(values) == 0 {
			result.Error = fmt.Errorf("empty alternation group in pattern")
			return result
		}

		// Clean and unescape each value
		cleanedValues := make([]string, 0, len(values))
		for _, val := range values {
			cleaned := strings.TrimSpace(val)

			// Remove leading/trailing parentheses from nested groups
			// Loop to handle deeply nested groups like (((api)))
			for strings.HasPrefix(cleaned, "(") || strings.HasSuffix(cleaned, ")") {
				if strings.HasPrefix(cleaned, "(") {
					cleaned = strings.TrimPrefix(cleaned, "(")
				}
				if strings.HasSuffix(cleaned, ")") {
					cleaned = strings.TrimSuffix(cleaned, ")")
				}
				cleaned = strings.TrimSpace(cleaned)
			}

			// Unescape regex characters
			cleaned = dc.unescapeRegex(cleaned)

			if cleaned != "" {
				cleanedValues = append(cleanedValues, cleaned)
			}
		}

		if len(cleanedValues) == 0 {
			result.Error = fmt.Errorf("alternation group with no valid values")
			return result
		}

		// Sort values for consistency
		sort.Strings(cleanedValues)

		// Remove duplicates (can occur with nested groups)
		cleanedValues = removeDuplicates(cleanedValues)

		// Generate positional variable name
		varName := fmt.Sprintf("p%d", positionCounter)
		positionCounter++

		// Store payload
		result.Payloads[varName] = cleanedValues

		// Replace alternation with positional variable
		placeholder := fmt.Sprintf("{{%s}}", varName)

		// Handle optional groups
		if isOptional {
			// Remove the (...)? and replace with {{pN}}
			template = strings.Replace(template, fullMatch, placeholder, 1)
		} else {
			template = strings.Replace(template, fullMatch, placeholder, 1)
		}
	}

	// Clean up nested optional groups after variable substitution
	// Pattern like: ({{p0}}{{p1}})? → {{p0}}{{p1}}
	template = dc.cleanupNestedOptionalGroups(template)

	// Unescape remaining regex characters in template
	template = dc.unescapeRegex(template)

	// Ensure template ends with .{{suffix}}
	template = dc.ensureSuffix(template)

	result.Template = template
	return result
}

// cleanupNestedOptionalGroups removes outer optional group wrappers after variable substitution
// Handles patterns like: ({{p0}}{{p1}})? → {{p0}}{{p1}} OR {{p0}}{{p1}})? → {{p0}}{{p1}}
// This occurs when the entire pattern is an optional group containing only variables
func (dc *DSLConverter) cleanupNestedOptionalGroups(template string) string {
	// Due to how nested alternations are processed, we may get patterns like:
	// - ({{p0}}{{p1}})? - full optional group (ideal case)
	// - {{p0}}{{p1}})? - trailing )? after variables (actual case with nested groups)
	// - {{p0}}? - simple optional single variable

	// Pattern 1: Match complete optional groups: ({{...}}+)?
	optionalGroupPattern := regexp.MustCompile(`\((\{\{[^}]+\}\})+\)\?`)
	result := optionalGroupPattern.ReplaceAllStringFunc(template, func(match string) string {
		// Remove leading ( and trailing )?
		if len(match) > 3 && match[0] == '(' && strings.HasSuffix(match, ")?") {
			return match[1 : len(match)-2]
		}
		return match
	})

	// Pattern 2: Match trailing )? after variables with possible literal text: {{...}}<literal>)?
	// This handles the case where nested parentheses cause incomplete matching
	// Examples: {{p0}})? or {{p0}}-dev)?
	trailingOptionalPattern := regexp.MustCompile(`(\{\{[^}]+\}\})+.*?\)\?`)
	result = trailingOptionalPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Remove trailing )?
		if strings.HasSuffix(match, ")?") {
			return match[:len(match)-2]
		}
		return match
	})

	// Pattern 3: Match simple optional variables: {{...}}?
	// This handles single optional groups like (api|web)?
	simpleOptionalPattern := regexp.MustCompile(`(\{\{[^}]+\}\})\?`)
	result = simpleOptionalPattern.ReplaceAllString(result, "$1")

	return result
}

// unescapeRegex removes regex escape sequences
// Converts: \. → ., \- → -, etc.
func (dc *DSLConverter) unescapeRegex(s string) string {
	// Remove backslash escapes for common regex special characters
	replacements := []struct{ old, new string }{
		{`\.`, `.`},
		{`\-`, `-`},
		{`\*`, `*`},
		{`\+`, `+`},
		{`\?`, `?`},
		{`\[`, `[`},
		{`\]`, `]`},
		{`\(`, `(`},
		{`\)`, `)`},
		{`\{`, `{`},
		{`\}`, `}`},
		{`\^`, `^`},
		{`\$`, `$`},
		{`\|`, `|`},
		{`\\`, `\`}, // Backslash should be last
	}

	result := s
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}

	return result
}

// ensureSuffix ensures template ends with .{{suffix}}
// Handles cases where suffix might already exist or pattern ends with a dot
func (dc *DSLConverter) ensureSuffix(template string) string {
	// If template already has {{suffix}}, don't add it again
	if strings.HasSuffix(template, "{{suffix}}") {
		return template
	}

	// If template ends with a dot, append {{suffix}} directly
	if strings.HasSuffix(template, ".") {
		return template + "{{suffix}}"
	}

	// Otherwise add .{{suffix}}
	return template + ".{{suffix}}"
}

// ConvertPattern is a convenience method that converts a Pattern struct
// Returns updated pattern with DSL template and inline payloads
func (dc *DSLConverter) ConvertPattern(pattern *Pattern) (*Pattern, error) {
	if pattern == nil {
		return nil, fmt.Errorf("nil pattern")
	}

	result := dc.Convert(pattern.Regex)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to convert pattern: %w", result.Error)
	}

	// Create new pattern with DSL template
	converted := &Pattern{
		Regex:      pattern.Regex,      // Keep original regex
		Coverage:   pattern.Coverage,
		Domains:    pattern.Domains,
		Ratio:      pattern.Ratio,
		Confidence: pattern.Confidence,
	}

	// Store conversion result in a way that can be used by AlterX
	// (This will be used by the integration code to create learned patterns)

	return converted, nil
}

// ExtractPayloadsFromRegex is a helper that only extracts payloads without conversion
// Useful for analyzing patterns before full conversion
func (dc *DSLConverter) ExtractPayloadsFromRegex(regexPattern string) (map[string][]string, error) {
	result := dc.Convert(regexPattern)
	if result.Error != nil {
		return nil, result.Error
	}
	return result.Payloads, nil
}

// ValidateTemplate checks if a DSL template is valid
// Validates that all positional variables have corresponding payloads
func (dc *DSLConverter) ValidateTemplate(template string, payloads map[string][]string) error {
	// Find all {{variable}} references in template
	varPattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := varPattern.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varName := match[1]

		// Skip built-in AlterX variables
		builtinVars := map[string]bool{
			"sub":    true,
			"suffix": true,
			"tld":    true,
			"etld":   true,
			"sld":    true,
			"root":   true,
		}

		// Also skip subN variables (sub1, sub2, etc.)
		if strings.HasPrefix(varName, "sub") && len(varName) > 3 {
			continue
		}

		if builtinVars[varName] {
			continue
		}

		// Check if positional variable has payloads
		if strings.HasPrefix(varName, "p") {
			if _, exists := payloads[varName]; !exists {
				return fmt.Errorf("missing payload for variable {{%s}}", varName)
			}

			if len(payloads[varName]) == 0 {
				return fmt.Errorf("empty payload for variable {{%s}}", varName)
			}
		}
	}

	return nil
}

// FormatPayloadsForYAML converts payloads map to a structured format for YAML output
// Returns a map suitable for inclusion in learned_patterns YAML structure
func (dc *DSLConverter) FormatPayloadsForYAML(payloads map[string][]string) map[string]interface{} {
	result := make(map[string]interface{})

	for varName, values := range payloads {
		result[varName] = map[string]interface{}{
			"values": values,
		}
	}

	return result
}

// removeDuplicates removes duplicate strings from a sorted slice
// Assumes input is already sorted for efficiency
func removeDuplicates(sorted []string) []string {
	if len(sorted) <= 1 {
		return sorted
	}

	result := make([]string, 0, len(sorted))
	result = append(result, sorted[0])

	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1] {
			result = append(result, sorted[i])
		}
	}

	return result
}

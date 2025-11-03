package inducer

import (
	"fmt"
	"strings"

	"github.com/projectdiscovery/gologger"
)

// EnrichPatterns adds optional variable support to learned patterns
// This enables ClusterBomb to generate both with and without optional variables
// Example: {{p0}}{{number}}-{{p2}} can generate both "api-dev" and "api01-dev"
func EnrichPatterns(patterns []*DSLPattern, tokenDict *TokenDictionary) []*DSLPattern {
	enriched := make([]*DSLPattern, 0, len(patterns))

	for _, pattern := range patterns {
		// Clone the pattern to avoid modifying the original
		enrichedPattern := clonePattern(pattern)

		// Analyze pattern structure and determine which variables can be optional
		optionalVars := determineOptionalVariables(enrichedPattern, tokenDict)

		// Add empty string "" to payloads for optional variables
		for _, varName := range optionalVars {
			addEmptyOptionToVariable(enrichedPattern, varName)
		}

		enriched = append(enriched, enrichedPattern)
	}

	return enriched
}

// determineOptionalVariables analyzes a pattern and decides which variables can be optional
// Uses token dictionary semantics and pattern structure to make intelligent decisions
func determineOptionalVariables(pattern *DSLPattern, tokenDict *TokenDictionary) []string {
	var optional []string

	// Parse template to understand variable positions
	template := pattern.Template
	variables := extractVariablesFromTemplate(template)

	for _, varName := range variables {
		// Find the variable in pattern.Variables
		var variable *DSLVariable
		for i := range pattern.Variables {
			if pattern.Variables[i].Name == varName {
				variable = &pattern.Variables[i]
				break
			}
		}

		if variable == nil {
			continue
		}

		// Decision rules for making variables optional:
		//
		// 1. Numbers are ALWAYS optional (common pattern: api01 vs api)
		if variable.Type == TokenTypeNumber || variable.NumberRange != nil {
			optional = append(optional, varName)
			gologger.Verbose().Msgf("  Variable %s marked optional (reason: number type)", varName)
			continue
		}

		// 2. Semantic tokens can be optional if they're modifiers (env, region, version)
		if tokenDict != nil {
			isModifier := isSemanticModifier(variable, tokenDict)
			if isModifier {
				optional = append(optional, varName)
				gologger.Verbose().Msgf("  Variable %s marked optional (reason: semantic modifier)", varName)
				continue
			}
		}

		// 3. Positional variables with single value are likely required (core identifiers)
		// Example: p0=[api] is core, shouldn't be omitted
		if len(variable.Payloads) == 1 {
			gologger.Verbose().Msgf("  Variable %s kept required (reason: single value core identifier)", varName)
			continue
		}

		// 4. Positional variables in middle of pattern can be optional if pattern makes sense without them
		// Example: {{p0}}-{{p1}}-{{p2}} where p1 could be optional
		if isMiddleVariable(varName, template) && len(variable.Payloads) > 1 {
			// Only make optional if omitting doesn't break the pattern structure
			if wouldCreateValidPattern(template, varName) {
				optional = append(optional, varName)
				gologger.Verbose().Msgf("  Variable %s marked optional (reason: middle position with valid omit)", varName)
			}
		}
	}

	return optional
}

// isSemanticModifier checks if a variable is a semantic modifier (env, region, version, etc.)
func isSemanticModifier(variable *DSLVariable, tokenDict *TokenDictionary) bool {
	// Check if any payload matches known semantic categories
	for _, payload := range variable.Payloads {
		// Check environment tokens (dev, prod, staging, qa)
		for _, env := range tokenDict.Env {
			if payload == env {
				return true
			}
		}
		// Check region tokens (us-east-1, eu-central-1, etc.)
		for _, region := range tokenDict.Region {
			if payload == region {
				return true
			}
		}
	}
	return false
}

// isMiddleVariable checks if a variable is in the middle of a pattern (not first or last)
func isMiddleVariable(varName string, template string) bool {
	// Extract all variables from template
	vars := extractVariablesFromTemplate(template)
	if len(vars) <= 2 {
		return false // No middle position in patterns with 2 or fewer variables
	}

	// Find position of this variable
	for i, v := range vars {
		if v == varName {
			// Middle variables are not first or last
			return i > 0 && i < len(vars)-1
		}
	}
	return false
}

// wouldCreateValidPattern checks if omitting a variable would create a valid subdomain pattern
func wouldCreateValidPattern(template string, varName string) bool {
	// Simulate omitting the variable by replacing with empty string
	simulated := strings.ReplaceAll(template, "{{"+varName+"}}", "")

	// Check for invalid patterns that would result
	// Pattern must not have consecutive delimiters after omission
	if strings.Contains(simulated, "--") || strings.Contains(simulated, "-.") || strings.Contains(simulated, "._") {
		return false
	}

	// Pattern must not start with delimiter
	if strings.HasPrefix(strings.TrimSpace(simulated), "-") || strings.HasPrefix(strings.TrimSpace(simulated), ".") {
		return false
	}

	return true
}

// extractVariablesFromTemplate extracts all {{variable}} names from a template
func extractVariablesFromTemplate(template string) []string {
	var variables []string
	parts := strings.Split(template, "{{")

	for _, part := range parts {
		if strings.Contains(part, "}}") {
			varName := strings.Split(part, "}}")[0]
			// Skip built-in variables like root, suffix
			if varName != "root" && varName != "suffix" && varName != "tld" && varName != "etld" {
				variables = append(variables, varName)
			}
		}
	}

	return variables
}

// addEmptyOptionToVariable adds empty string "" as first value to variable's payloads
func addEmptyOptionToVariable(pattern *DSLPattern, varName string) {
	for i := range pattern.Variables {
		if pattern.Variables[i].Name == varName {
			variable := &pattern.Variables[i]

			// Handle NumberRange - expand it and add empty string
			if variable.NumberRange != nil {
				// Expand the NumberRange to actual values
				expanded := expandNumberRangeToStrings(variable.NumberRange)
				// Prepend empty string for omit functionality
				variable.Payloads = append([]string{""}, expanded...)
				gologger.Verbose().Msgf("  Added empty option to number variable %s (total values: %d including omit)", varName, len(variable.Payloads))
			} else {
				// For word variables, prepend empty string
				if len(variable.Payloads) == 0 || variable.Payloads[0] != "" {
					variable.Payloads = append([]string{""}, variable.Payloads...)
				}
				gologger.Verbose().Msgf("  Added empty option to word variable %s (new payload count: %d)", varName, len(variable.Payloads))
			}

			break
		}
	}
}

// expandNumberRangeToStrings converts a NumberRange to a slice of formatted strings
func expandNumberRangeToStrings(nr *NumberRange) []string {
	var result []string

	step := nr.Step
	if step == 0 {
		step = 1
	}

	format := nr.Format
	if format == "" {
		format = "%d"
	}

	for i := nr.Start; i <= nr.End; i += step {
		result = append(result, fmt.Sprintf(format, i))
	}

	return result
}

// clonePattern creates a deep copy of a DSLPattern
func clonePattern(pattern *DSLPattern) *DSLPattern {
	clone := &DSLPattern{
		Template:   pattern.Template,
		LevelCount: pattern.LevelCount,
		Coverage:   pattern.Coverage,
		Ratio:      pattern.Ratio,
		Confidence: pattern.Confidence,
		Variables:  make([]DSLVariable, len(pattern.Variables)),
		Domains:    make([]string, len(pattern.Domains)),
	}

	// Deep copy variables
	for i, variable := range pattern.Variables {
		clone.Variables[i] = DSLVariable{
			Name:     variable.Name,
			Type:     variable.Type,
			Payloads: make([]string, len(variable.Payloads)),
		}
		copy(clone.Variables[i].Payloads, variable.Payloads)

		// Clone NumberRange if present
		if variable.NumberRange != nil {
			clone.Variables[i].NumberRange = &NumberRange{
				Start:  variable.NumberRange.Start,
				End:    variable.NumberRange.End,
				Format: variable.NumberRange.Format,
				Step:   variable.NumberRange.Step,
				Type:   variable.NumberRange.Type,
			}
		}
	}

	// Copy domains
	copy(clone.Domains, pattern.Domains)

	return clone
}
